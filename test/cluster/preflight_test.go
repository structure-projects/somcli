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
)

// TestSC_K14_Preflight somcli 该在节点上做的前置准备，一件件验。
//
// 与 SC-K01「集群能不能用」分开：那条是结果，这条是过程。结果绿了不代表过程做全了 ——
// 内核参数可能是宿主本来就设好的（E2E 的宿主准备就 modprobe 过 br_netfilter），
// 那种情况下节点照样 Ready，而换一台干净机器就装不上。
// 所以这里断言的是**落在节点上的文件**：文件在，重启之后参数才还在。
//
// 宿主上做的那三件事（swapoff、modprobe、cgroup 命名空间）不在这条用例的判据里，
// 它们是容器化节点共享宿主内核的代价，不是 somcli 的活。
func TestSC_K14_Preflight(t *testing.T) {
	t.Cleanup(func() {
		if t.Failed() {
			dumpNodeLogs()
		}
	})

	resetCluster(t, master, worker)

	cfg := clusterConfig(t, "sc-k14", "1.28.2", "containerd", "", master)
	code, out := runSomcli(t, "cluster", "create", "-f", cfg)
	if code != 0 {
		t.Fatalf("cluster create 退出码 = %d\n输出：\n%s", code, out)
	}

	// F8：参数得写进文件，不能只靠 sysctl -w。只设不写的话重启就没了，
	// 而重启后的表现是 Pod 网络莫名不通。
	sysctlConf, err := ssh(master, "cat /etc/sysctl.d/k8s.conf 2>/dev/null || true")
	if err != nil {
		t.Fatalf("读 /etc/sysctl.d/k8s.conf 失败: %v", err)
	}
	for _, key := range []string{
		"net.bridge.bridge-nf-call-iptables",
		"net.ipv4.ip_forward",
	} {
		if !strings.Contains(sysctlConf, key) {
			t.Errorf("/etc/sysctl.d/k8s.conf 里没有 %s（F8）：\n%s", key, sysctlConf)
		}
	}

	// 模块同理：modprobe 的效果重启就没了，要靠 modules-load.d 才是持久的。
	modConf, err := ssh(master, "cat /etc/modules-load.d/k8s.conf 2>/dev/null || true")
	if err != nil {
		t.Fatalf("读 /etc/modules-load.d/k8s.conf 失败: %v", err)
	}
	for _, mod := range []string{"overlay", "br_netfilter"} {
		if !strings.Contains(modConf, mod) {
			t.Errorf("/etc/modules-load.d/k8s.conf 里没有 %s（F8）：\n%s", mod, modConf)
		}
	}

	// swap 必须是关着的：kubeadm 见到 swap 直接拒绝。
	swaps, err := ssh(master, "swapon --show || true")
	if err != nil {
		t.Fatalf("读 swap 状态失败: %v", err)
	}
	if strings.TrimSpace(swaps) != "" {
		t.Errorf("节点上还有 swap 在用：\n%s", swaps)
	}

	// F10：防火墙开着的话，worker 连不上 6443、kubelet 连不上 10250，
	// 表现是节点 NotReady 而不是"被墙了"。
	fw, err := ssh(master, "systemctl is-active firewalld 2>/dev/null || true")
	if err != nil {
		t.Fatalf("读 firewalld 状态失败: %v", err)
	}
	if strings.TrimSpace(fw) == "active" {
		t.Errorf("firewalld 仍在运行（F10）")
	}
}
