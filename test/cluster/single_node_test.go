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
	"strings"
	"testing"
	"time"
)

// TestSC_K01_SingleMasterContainerd 单 master + containerd，装完必须是个能用的集群。
//
// 「能用」的判据一路收紧，因为每一层都有各自的假绿：
//   - somcli 退出 0：现在就成立，且毫无意义 —— 装不上也会报成功（这正是 M2 的起点）；
//   - kubelet active：kubelet 起来了不代表 apiserver 通；
//   - 节点出现在 kubectl get nodes：出现了不代表 Ready；
//   - 节点 Ready：这一步要 CNI 真的部署了才可能成立（D5：calico/flannel 在代码里零命中）。
//
// 期望在 M2.2 之前是红的，红在哪一层就说明哪个缺陷还在：
//
//	D3 → containerd.service / daemon.json 没落盘，systemctl enable 失败
//	F7 → config.toml 没有 SystemdCgroup，kubelet 与 kubeadm 的 cgroup 驱动打架
//	F8 → 没写 /etc/sysctl.d/k8s.conf，kubeadm preflight 直接拒绝
//	D5 → 没有 CNI，节点永久 NotReady
func TestSC_K01_SingleMasterContainerd(t *testing.T) {
	// 失败时把节点现场打出来。集群失败的原因几乎从不在 somcli 的输出里，
	// 而在 kubelet 日志和 containerd 配置里；CI 上容器随 runner 消失，不捞就没了。
	t.Cleanup(func() {
		if t.Failed() {
			dumpNodeLogs()
		}
	})

	// 节点是全包共用的，前一条用例装的集群还留在上面。不先清场的话 kubeadm init
	// 会以"端口已占用 / 配置已存在"失败，而那与本条要验的东西无关。
	resetCluster(t, master, worker)

	cfg := clusterConfig(t, "sc-k01", "1.28.2", "containerd", "", master)

	code, out := runSomcli(t, "cluster", "create", "-f", cfg)
	if code != 0 {
		t.Fatalf("cluster create 退出码 = %d\n输出：\n%s", code, out)
	}

	// kubelet 必须是 systemd 管着的常驻服务，而不是被谁手动拉起来一次。
	if state, err := ssh(master, "systemctl is-active kubelet || true"); err != nil ||
		strings.TrimSpace(state) != "active" {
		t.Fatalf("master 上 kubelet 不是 active（%q, err=%v）\nsomcli 输出：\n%s",
			strings.TrimSpace(state), err, out)
	}

	// F7：cgroup 驱动两边必须一致。kubeadm 默认 systemd，containerd 默认 cgroupfs，
	// 不改就是 kubelet 反复重启 —— 而且这种失败的报错完全指不到根因上。
	conf, err := ssh(master, "cat /etc/containerd/config.toml 2>/dev/null || true")
	if err != nil {
		t.Fatalf("读 containerd 配置失败: %v", err)
	}
	if !strings.Contains(strings.ReplaceAll(conf, " ", ""), "SystemdCgroup=true") {
		t.Errorf("containerd 配置里没有 SystemdCgroup = true（F7）：\n%s", conf)
	}

	// F8：这两个内核参数缺一个，kubeadm preflight 就会拒绝，且 Pod 网络不通。
	sysctl, err := ssh(master, "sysctl -n net.bridge.bridge-nf-call-iptables net.ipv4.ip_forward || true")
	if err != nil {
		t.Fatalf("读内核参数失败: %v", err)
	}
	for _, line := range strings.Fields(sysctl) {
		if line != "1" {
			t.Errorf("内核参数不是全为 1（F8）：\n%s", sysctl)
			break
		}
	}

	// 节点得先被 apiserver 认识，再谈 Ready。分两步等是为了让失败指向不同的原因：
	// 前者不成立说明 kubeadm init / kubelet 注册就没成，后者不成立基本就是没有 CNI。
	waitFor(t, "节点出现在 kubectl get nodes 里", 2*time.Minute, func() (bool, string) {
		out, err := kubectl(t, "get", "nodes", "-o", "name")
		if err != nil {
			return false, err.Error()
		}
		return strings.Contains(out, master.host), out
	})

	waitFor(t, "节点变成 Ready（要求 CNI 已部署，D5）", 5*time.Minute, func() (bool, string) {
		out, err := kubectl(t, "get", "nodes", "--no-headers")
		if err != nil {
			return false, err.Error()
		}
		// 只认独立的 Ready：NotReady 里也含 "Ready" 这五个字母，用 Contains 会全绿。
		for _, f := range strings.Fields(out) {
			if f == "Ready" {
				return true, out
			}
		}
		return false, out
	})

	// 控制面自身的 Pod 起不来的话，节点 Ready 也只是个空壳集群。
	waitFor(t, "kube-system 里没有非 Running 的控制面 Pod", 3*time.Minute, func() (bool, string) {
		out, err := kubectl(t, "get", "pods", "-n", "kube-system", "--no-headers")
		if err != nil {
			return false, err.Error()
		}
		if strings.TrimSpace(out) == "" {
			return false, "kube-system 里一个 Pod 都没有"
		}
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 3 {
				continue
			}
			if fields[2] != "Running" && fields[2] != "Completed" {
				return false, out
			}
		}
		return true, out
	})
}
