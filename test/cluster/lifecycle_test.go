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

// TestSC_K11_RemoveLeavesCleanNodes cluster remove 之后节点必须能再装一遍。
//
// 「干净」不取"某个目录没了"作为最终判据 —— 那种断言太容易糊过去（`rm -rf` 谁都会写），
// 而实际卡住重装的往往是别的东西：上一次 CNI 留下的网桥（Pod 会拿到上一个网段里的
// 地址，表现是跨节点不通）、没退出的 etcd 成员、还在跑的 kubelet。
//
// 所以判据是"再装一遍能成"，目录检查只作为失败时的定位信息一并给出。
func TestSC_K11_RemoveLeavesCleanNodes(t *testing.T) {
	t.Cleanup(func() {
		if t.Failed() {
			dumpNodeLogs()
		}
	})

	resetCluster(t, master, worker)

	cfg := clusterConfig(t, "sc-k11", "1.28.2", "containerd", "", master, worker)
	if code, out := runSomcli(t, "cluster", "create", "-f", cfg); code != 0 {
		t.Fatalf("cluster create 退出码 = %d\n输出：\n%s", code, out)
	}
	waitFor(t, "两个节点都 Ready", 8*time.Minute, func() (bool, string) {
		got, err := kubectl(t, "get", "nodes", "--no-headers")
		if err != nil {
			return false, err.Error()
		}
		return strings.Count(got, " Ready") == 2, got
	})

	// --force 是为了跳过交互确认：没有终端时 AskForConfirmation 读到 EOF 会当成"否"。
	code, out := runSomcli(t, "cluster", "remove", "-f", cfg, "--force")
	if code != 0 {
		t.Fatalf("cluster remove 退出码 = %d\n输出：\n%s", code, out)
	}

	for _, n := range []node{master, worker} {
		for _, path := range []string{"/etc/kubernetes", "/etc/cni/net.d"} {
			if out, err := ssh(n, fmt.Sprintf("test -e %s && echo LEFT || echo GONE", path)); err != nil {
				t.Errorf("查 %s 上的 %s 失败: %v", n.host, path, err)
			} else if strings.TrimSpace(out) != "GONE" {
				t.Errorf("%s 上 %s 还在", n.host, path)
			}
		}
		// kubelet 由 kubeadm reset 停掉。还在跑的话下一次 kubeadm init 会撞上
		// "端口已占用"，而报错指向的是端口，不是"上次没停"。
		if out, err := ssh(n, "systemctl is-active kubelet || true"); err == nil &&
			strings.TrimSpace(out) == "active" {
			t.Errorf("%s 上 kubelet 仍在运行", n.host)
		}
		// CNI 的网桥。
		if out, err := ssh(n, "ip link show cni0 >/dev/null 2>&1 && echo LEFT || echo GONE"); err != nil {
			t.Errorf("查 %s 上的 cni0 失败: %v", n.host, err)
		} else if strings.TrimSpace(out) != "GONE" {
			t.Errorf("%s 上 cni0 网桥还在：下一次装出来的 Pod 会拿到上一次网段里的地址", n.host)
		}
	}

	// 真正的判据：同一套节点上再装一遍。
	// 不做 resetCluster —— 那正是这条用例要验的事，替它清场就什么也没验。
	again := clusterConfig(t, "sc-k11-again", "1.28.2", "containerd", "", master, worker)
	if code, out := runSomcli(t, "cluster", "create", "-f", again); code != 0 {
		t.Fatalf("remove 之后再装一遍失败（退出码 %d）—— 节点没被清干净。\n输出：\n%s", code, out)
	}
	waitFor(t, "重装后两个节点都 Ready", 8*time.Minute, func() (bool, string) {
		got, err := kubectl(t, "get", "nodes", "--no-headers")
		if err != nil {
			return false, err.Error()
		}
		return strings.Count(got, " Ready") == 2, got
	})

	// Pod 地址仍落在配置的网段里：上一次的网桥没清掉时，这里会看到旧网段的地址。
	applyManifest(t, `
apiVersion: v1
kind: Pod
metadata:
  name: k11-probe
spec:
  nodeName: `+worker.host+`
  containers:
    - name: probe
      image: busybox:1.36
      command: ["sleep", "600"]
`)
	var podIP string
	waitFor(t, "探针 Pod 拿到地址", 4*time.Minute, func() (bool, string) {
		got, err := kubectl(t, "get", "pod", "k11-probe", "-o", "jsonpath={.status.podIP}")
		if err != nil {
			return false, err.Error()
		}
		podIP = strings.TrimSpace(got)
		return podIP != "", got
	})
	if !strings.HasPrefix(podIP, "10.244.") {
		t.Errorf("重装后 Pod 地址 %s 不在 10.244.0.0/16 内", podIP)
	}
}
