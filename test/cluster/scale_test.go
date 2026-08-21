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
	"strings"
	"testing"
	"time"
)

// TestSC_K12_K13_Scale 扩容与缩容。
//
// 两件事放在一条用例里是因为它们共享一个昂贵的前提（一个装好的两节点集群），
// 而且缩容只能摘一台先加进来的机器 —— 拆成两条的话第二条得把第一条再做一遍。
//
// 扩进来的那台用的是 fixture 里的第三个容器（compose 里叫 k8s-master2）。
// 这里按 worker 角色使用它：容器本身是通用的 systemd 节点，主机名叫什么不影响；
// 为此多起一个第四台容器，只会让一晚上的额度多花在 apt 上。
func TestSC_K12_K13_Scale(t *testing.T) {
	t.Cleanup(func() {
		if t.Failed() {
			dumpNodeLogs()
		}
	})

	extra := node{host: master2.host, ip: master2.ip, role: "worker"}
	resetCluster(t, master, worker, extra)

	// 先装一个不含 extra 的集群 —— 扩容要有个"已有集群"才谈得上。
	base := clusterConfig(t, "sc-k12", "1.28.2", "containerd", "", master, worker)
	if code, out := runSomcli(t, "cluster", "create", "-f", base); code != 0 {
		t.Fatalf("cluster create 退出码 = %d\n输出：\n%s", code, out)
	}
	waitFor(t, "两个节点都 Ready", 8*time.Minute, func() (bool, string) {
		got, err := kubectl(t, "get", "nodes", "--no-headers")
		if err != nil {
			return false, err.Error()
		}
		return strings.Count(got, " Ready") == 2, got
	})

	// 扩容用的配置多写一台。要加的节点必须先写进配置里 —— 凭据与角色都在那儿。
	scaled := clusterConfig(t, "sc-k12", "1.28.2", "containerd", "", master, worker, extra)

	t.Run("扩容：新增一台 worker", func(t *testing.T) {
		if code, out := runSomcli(t, "cluster", "add-node", "-f", scaled, "--node", extra.host); code != 0 {
			t.Fatalf("cluster add-node 退出码 = %d\n输出：\n%s", code, out)
		}

		waitFor(t, "三个节点都 Ready", 6*time.Minute, func() (bool, string) {
			got, err := kubectl(t, "get", "nodes", "--no-headers")
			if err != nil {
				return false, err.Error()
			}
			return strings.Count(got, " Ready") == 3, got
		})

		// 判据不止"多了一行"：新节点上得真的能跑起 Pod。CNI 没在这台上铺开时，
		// 节点也能显示 Ready，而 Pod 会一直卡在 ContainerCreating。
		applyManifest(t, `
apiVersion: v1
kind: Pod
metadata:
  name: k12-probe
spec:
  nodeName: `+extra.host+`
  containers:
    - name: probe
      image: busybox:1.36
      command: ["sleep", "600"]
`)
		var podIP string
		waitFor(t, "新节点上的 Pod 跑起来", 4*time.Minute, func() (bool, string) {
			got, err := kubectl(t, "get", "pod", "k12-probe",
				"-o", "jsonpath={.status.phase}/{.status.podIP}")
			if err != nil {
				return false, err.Error()
			}
			parts := strings.SplitN(strings.TrimSpace(got), "/", 2)
			if len(parts) != 2 || parts[0] != "Running" || parts[1] == "" {
				return false, got
			}
			podIP = parts[1]
			return true, got
		})
		if !strings.HasPrefix(podIP, "10.244.") {
			t.Errorf("新节点上的 Pod 地址 %s 不在 10.244.0.0/16 内", podIP)
		}
	})

	t.Run("缩容：摘掉这台 worker", func(t *testing.T) {
		// 留着上一条的探针 Pod 不删：drain 要疏散的正是这种东西。
		// 它没有控制器管着，drain 不带 --force 会直接拒绝执行。
		if code, out := runSomcli(t, "cluster", "remove-node", "-f", scaled,
			"--node", extra.host, "--force"); code != 0 {
			t.Fatalf("cluster remove-node 退出码 = %d\n输出：\n%s", code, out)
		}

		got, err := kubectl(t, "get", "nodes", "--no-headers")
		if err != nil {
			t.Fatalf("查节点失败: %v", err)
		}
		if strings.Contains(got, extra.host) {
			t.Errorf("摘除之后集群里仍有 %s：\n%s", extra.host, got)
		}
		if n := strings.Count(got, " Ready"); n != 2 {
			t.Errorf("摘除之后 Ready 节点数 = %d，期望 2：\n%s", n, got)
		}

		// 剩下两台不该被牵连：缩容只该动被摘的那一台。
		for _, n := range []node{master, worker} {
			if !strings.Contains(got, n.host) {
				t.Errorf("摘一台 worker 却把 %s 也弄没了：\n%s", n.host, got)
			}
		}

		// 被摘的机器要被抹干净 —— 走的是与 cluster remove 同一套重置，
		// "抹干净等于还能再装一遍"那件事由 SC-K11 验，这里只确认重置确实执行了。
		for _, path := range []string{"/etc/kubernetes", "/etc/cni/net.d"} {
			out, err := ssh(extra, fmt.Sprintf("test -e %s && echo LEFT || echo GONE", path))
			if err != nil {
				t.Errorf("查 %s 上的 %s 失败: %v", extra.host, path, err)
			} else if strings.TrimSpace(out) != "GONE" {
				t.Errorf("%s 上 %s 还在 —— 被摘的节点没重置", extra.host, path)
			}
		}
		if out, err := ssh(extra, "systemctl is-active kubelet || true"); err == nil &&
			strings.TrimSpace(out) == "active" {
			t.Errorf("%s 上 kubelet 仍在运行", extra.host)
		}
	})
}
