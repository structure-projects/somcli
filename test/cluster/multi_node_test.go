//go:build cluster

/*
Copyright 2023 Structure Projects

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cluster

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestSC_K03_WorkerJoinsAndPodsTalk 1 master + 1 worker：worker 真的加入，且跨节点通。
//
// 一条用例串起三层，因为它们共用同一次安装（装一遍要好几分钟，每层各装一次划不来），
// 且失败层次本身就是诊断信息：
//
//	SC-K03 worker 出现在 kubectl get nodes 里 —— 修 D4 之前必红：
//	  join 命令是从 kubeadm init 输出里按行刮的，而 kubeadm 把它打成两行，
//	  丢掉 --discovery-token-ca-cert-hash 的半条命令跑起来必然失败。
//	SC-K09 两个节点上的 Pod 互相能通 —— 要 CNI 真装上（D5）才可能成立。
//	SC-K10 NodePort 从宿主可访问 —— 数据面从外部进来这条路通不通。
func TestSC_K03_WorkerJoinsAndPodsTalk(t *testing.T) {
	t.Cleanup(func() {
		if t.Failed() {
			dumpNodeLogs()
		}
	})

	resetCluster(t, master, worker)

	cfg := clusterConfig(t, "sc-k03", "1.28.2", "containerd", "", master, worker)
	code, out := runSomcli(t, "cluster", "create", "-f", cfg)
	if code != 0 {
		t.Fatalf("cluster create 退出码 = %d\n输出：\n%s", code, out)
	}

	// SC-K03：worker 在不在集群里，是 join 命令完整与否的唯一可信判据。
	waitFor(t, "两个节点都出现在 kubectl get nodes 里（SC-K03 / D4）", 3*time.Minute,
		func() (bool, string) {
			got, err := kubectl(t, "get", "nodes", "-o", "name")
			if err != nil {
				return false, err.Error()
			}
			return strings.Contains(got, master.host) && strings.Contains(got, worker.host), got
		})

	waitFor(t, "两个节点都 Ready", 5*time.Minute, func() (bool, string) {
		got, err := kubectl(t, "get", "nodes", "--no-headers")
		if err != nil {
			return false, err.Error()
		}
		ready := 0
		for _, line := range strings.Split(strings.TrimSpace(got), "\n") {
			for _, f := range strings.Fields(line) {
				// 只认独立的 Ready：NotReady 里也含这五个字母
				if f == "Ready" {
					ready++
					break
				}
			}
		}
		return ready == 2, got
	})

	assertCrossNodePodTraffic(t)
	assertNodePortReachable(t)
}

// assertCrossNodePodTraffic SC-K09：master 上的 Pod 与 worker 上的 Pod 互相能通。
//
// 两个 Pod 用 nodeName 钉死在不同节点上，这样"通"必须经过 CNI 的跨节点数据面；
// 让调度器自由选的话两个 Pod 可能落在同一台机器上，同节点走的是本地网桥，
// 跨节点不通照样能绿。
func assertCrossNodePodTraffic(t *testing.T) {
	t.Helper()

	applyManifest(t, fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  name: xnode-server
spec:
  nodeName: %s
  # master 默认带 control-plane 污点，不容忍就永远 Pending
  tolerations:
    - operator: "Exists"
  containers:
    - name: server
      image: busybox:1.36
      command: ["sh", "-c", "while true; do echo -e 'HTTP/1.1 200 OK\r\n\r\nsomcli-xnode' | nc -l -p 8080 -q 1; done"]
---
apiVersion: v1
kind: Pod
metadata:
  name: xnode-client
spec:
  nodeName: %s
  containers:
    - name: client
      image: busybox:1.36
      command: ["sleep", "3600"]
`, master.host, worker.host))

	for _, pod := range []string{"xnode-server", "xnode-client"} {
		waitFor(t, "Pod "+pod+" Running", 4*time.Minute, func() (bool, string) {
			got, err := kubectl(t, "get", "pod", pod, "-o", "jsonpath={.status.phase}")
			if err != nil {
				return false, err.Error()
			}
			return strings.TrimSpace(got) == "Running", got
		})
	}

	serverIP, err := kubectl(t, "get", "pod", "xnode-server", "-o", "jsonpath={.status.podIP}")
	if err != nil {
		t.Fatalf("取 server Pod 的 IP 失败: %v", err)
	}
	serverIP = strings.TrimSpace(serverIP)
	if serverIP == "" {
		t.Fatal("server Pod 没有 IP —— CNI 没有给 Pod 分配地址（D5）")
	}
	// Pod 地址必须落在配置的 Pod 网段里。落在 CNI 自带的默认网段上说明清单里的
	// CIDR 没被改成配置里的值，表现会是路由与 kube-proxy 的规则对不上。
	if !strings.HasPrefix(serverIP, "10.244.") {
		t.Errorf("Pod 地址 %s 不在配置的 Pod 网段 10.244.0.0/16 内", serverIP)
	}

	waitFor(t, "worker 上的 Pod 能访问 master 上的 Pod（SC-K09 / D5）", 2*time.Minute,
		func() (bool, string) {
			got, err := kubectl(t, "exec", "xnode-client", "--",
				"wget", "-q", "-T", "5", "-O-", "http://"+serverIP+":8080")
			if err != nil {
				return false, fmt.Sprintf("%v / %s", err, got)
			}
			return strings.Contains(got, "somcli-xnode"), got
		})
}

// assertNodePortReachable SC-K10：NodePort 服务从宿主访问得到。
//
// 从宿主而不是从节点内部发请求：节点内部访问走的是本机 iptables 规则，
// 而"从外面进得来"才是 NodePort 存在的意义。宿主与节点在同一个 docker 网络上，
// 直接连节点 IP 即可。
func assertNodePortReachable(t *testing.T) {
	t.Helper()

	applyManifest(t, fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  name: nodeport-app
  labels:
    app: nodeport-app
spec:
  nodeName: %s
  containers:
    - name: app
      image: busybox:1.36
      command: ["sh", "-c", "while true; do echo -e 'HTTP/1.1 200 OK\r\n\r\nsomcli-nodeport' | nc -l -p 8080 -q 1; done"]
---
apiVersion: v1
kind: Service
metadata:
  name: nodeport-app
spec:
  type: NodePort
  selector:
    app: nodeport-app
  ports:
    - port: 8080
      targetPort: 8080
      nodePort: 31380
`, worker.host))

	waitFor(t, "NodePort 后端 Pod Running", 4*time.Minute, func() (bool, string) {
		got, err := kubectl(t, "get", "pod", "nodeport-app", "-o", "jsonpath={.status.phase}")
		if err != nil {
			return false, err.Error()
		}
		return strings.TrimSpace(got) == "Running", got
	})

	url := fmt.Sprintf("http://%s:31380/", worker.ip)
	waitFor(t, "宿主访问 NodePort "+url+"（SC-K10）", 2*time.Minute, func() (bool, string) {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			return false, err.Error()
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return strings.Contains(string(body), "somcli-nodeport"),
			fmt.Sprintf("HTTP %d: %s", resp.StatusCode, body)
	})
}
