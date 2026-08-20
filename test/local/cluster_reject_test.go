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
package local

import (
	"fmt"
	"strings"
	"testing"
)

// 这一组验的是 cluster create 在**动手之前**就该说不的那些情况。
//
// 为什么值得单独一组：集群安装真正跑起来要特权容器 + kubeadm，只有 cluster 组能验；
// 而"配置本身就不可能装成"这件事，判断发生在连节点之前，本机就能验。
// 这也是唯一能在开发机上给出可信信号的集群用例。
//
// 所有配置里的节点 IP 一律取 192.0.2.0/24（RFC 5737 TEST-NET-1，保证不可路由）：
// 用例的前提是"拒绝发生在碰节点之前"，万一实现没拒绝，也绝不能让它真的往某台机器上
// 装 k8s —— 尤其不能是开发机自己（127.0.0.1 会被当作本机直接执行）。

// clusterConfig 生成一份 k8s 集群配置。masters/workers 是节点数量，
// extra 是要塞进 k8sConfig 的额外键（例如 controlPlaneEndpoint）。
func clusterConfig(t *testing.T, version, runtime string, masters, workers int, extra string) string {
	t.Helper()

	var b strings.Builder
	b.WriteString("cluster:\n  - type: \"k8s\"\n    name: \"reject-probe\"\n    nodes:\n")
	for i := 0; i < masters; i++ {
		fmt.Fprintf(&b, `      - host: "m%d"
        ip: "192.0.2.%d"
        role: "master"
        user: "root"
        sshKey: "~/.ssh/id_rsa"
`, i+1, 10+i)
	}
	for i := 0; i < workers; i++ {
		fmt.Fprintf(&b, `      - host: "w%d"
        ip: "192.0.2.%d"
        role: "worker"
        user: "root"
        sshKey: "~/.ssh/id_rsa"
`, i+1, 30+i)
	}
	fmt.Fprintf(&b, `    k8sConfig:
      version: %q
      containerRuntime: %q
      podNetworkCidr: "10.244.0.0/16"
      serviceCidr: "10.96.0.0/12"
`, version, runtime)
	if extra != "" {
		b.WriteString(extra)
	}
	return writeConfig(t, b.String())
}

// touchedNodes 判断这次运行是不是已经开始连节点了。
// 拒绝类用例必须在这之前结束 —— 连上去之后再说"配置不对"，节点上已经被改过了。
func touchedNodes(out string) bool {
	for _, sign := range []string{"SSH执行失败", "正在准备节点", "正在检查操作系统"} {
		if strings.Contains(out, sign) {
			return true
		}
	}
	return false
}

// TestSC_K15_DockershimRejectedOnModernK8s k8s ≥ 1.24 配 runtime: docker 必须当场拒绝。
//
// dockershim 在 1.24 被移除，而 initK8sMaster 至今还写死
// --cri-socket unix:///var/run/dockershim.sock（pkg/cluster/kubernetes.go:325）。
// 配置里默认的 k8s 版本是 1.28 —— 也就是说这条路径必然失败，而失败发生在
// 装完 docker、装完 kubeadm、跑到 kubeadm init 的时候：机器已经被改了一遍。
//
// 判据是"在碰节点之前就拒绝"，不是"最终失败了"：最终失败现在也成立（kubeadm init 会
// 自己报错），但那时用户的机器已经不干净了。
func TestSC_K15_DockershimRejectedOnModernK8s(t *testing.T) {
	cfg := clusterConfig(t, "1.28.2", "docker", 1, 0, "")

	code, out := run(t, "cluster", "create", "-f", cfg)
	if code == 0 {
		t.Fatalf("1.28 配 runtime: docker 却退出 0，输出：\n%s", out)
	}
	if touchedNodes(out) {
		t.Fatalf("已经开始连节点才失败 —— 这时机器已经被改过一遍了。\n"+
			"应在配置校验阶段拒绝：dockershim 在 1.24 已移除，"+
			"而 pkg/cluster/kubernetes.go:325 仍写死 dockershim.sock。\n输出：\n%s", out)
	}
	// 报错必须点出"为什么不行"与"怎么办"，否则用户只能看着一个退出码猜。
	if !strings.Contains(out, "dockershim") {
		t.Errorf("拒绝的理由里没提 dockershim，用户无从判断该改什么：\n%s", out)
	}
	if !strings.Contains(out, "cri-dockerd") && !strings.Contains(out, "containerd") {
		t.Errorf("拒绝时没给出路（cri-dockerd 或改用 containerd）：\n%s", out)
	}
	if !strings.Contains(out, "1.24") {
		t.Errorf("拒绝的理由里没有 1.24 这个分界，看不出是版本问题：\n%s", out)
	}
}

// TestSC_K15_DockerAllowedBeforeDockershimRemoval 1.23 及以下配 docker 不该被这条规则拦。
//
// 与上一条成对：只写"拒绝"的话，一个"凡是 docker 都拒绝"的实现同样能过 ——
// 而那是把能装的场景也一起拒了。
//
// 这一条不断言最终成功（节点是不可路由地址，必然连不上），只断言**不是因为版本被拒**，
// 且流程确实往下走到了连节点那一步。
func TestSC_K15_DockerAllowedBeforeDockershimRemoval(t *testing.T) {
	// 这条必然要等 SSH 连不可路由地址超时（30 秒，超时值写在产品里）。
	// 同类用例并行跑，三次等待重叠成一次，默认测试层不至于因此多花一分半。
	t.Parallel()

	cfg := clusterConfig(t, "1.23.17", "docker", 1, 0, "")

	code, out := run(t, "cluster", "create", "-f", cfg)
	if code == 0 {
		t.Fatalf("节点是不可路由地址，不该成功（用例前提坏了），输出：\n%s", out)
	}
	if strings.Contains(out, "dockershim") && !touchedNodes(out) {
		t.Fatalf("1.23 配 docker 被当成非法拒绝了 —— dockershim 在 1.23 还在。\n输出：\n%s", out)
	}
	if !touchedNodes(out) {
		t.Fatalf("没走到连节点那一步就结束了，说明在校验阶段被拦下 —— 拦错了。\n输出：\n%s", out)
	}
}

// TestSC_K05_MultiMasterWithoutEndpointRejected 声明多个 master 却没有 VIP/LB 时必须拒绝。
//
// F9：kubeadm init 没有 --control-plane-endpoint（pkg/cluster/kubernetes.go:310），
// joinMaster 是空壳 return nil（同文件 :628）。于是配置里写三个 master 时，somcli 会
// 装完全部节点、init 第一个 master、对另外两个 master 打一句 warning 就宣布"创建成功"——
// 用户拿到的是一个单点集群，却以为自己有 HA。
//
// 多 master 结构上需要一个稳定的 apiserver 入口（VIP 或 LB），配置里没有就不可能装成，
// 因此必须在动手前拒绝并告诉用户缺什么。
func TestSC_K05_MultiMasterWithoutEndpointRejected(t *testing.T) {
	cfg := clusterConfig(t, "1.28.2", "containerd", 3, 0, "")

	code, out := run(t, "cluster", "create", "-f", cfg)
	if code == 0 {
		t.Fatalf("三个 master 没有 VIP 却退出 0 —— 用户会以为自己有 HA，输出：\n%s", out)
	}
	if touchedNodes(out) {
		t.Fatalf("已经开始连节点才失败。多 master 缺入口是配置层面就能判定的，"+
			"应在动手前拒绝。\n输出：\n%s", out)
	}
	if !strings.Contains(out, "controlPlaneEndpoint") {
		t.Errorf("没告诉用户该配 controlPlaneEndpoint，只说"+
			"不行等于让人猜：\n%s", out)
	}
}

// TestSC_K05_MultiMasterWithEndpointAccepted 给了入口就不该再被这条规则拦。
//
// 同样与上一条成对：防止实现变成"多 master 一律拒绝"。
func TestSC_K05_MultiMasterWithEndpointAccepted(t *testing.T) {
	// 这条必然要等 SSH 连不可路由地址超时（30 秒，超时值写在产品里）。
	// 同类用例并行跑，三次等待重叠成一次，默认测试层不至于因此多花一分半。
	t.Parallel()

	cfg := clusterConfig(t, "1.28.2", "containerd", 3, 0,
		"      controlPlaneEndpoint: \"192.0.2.100:6443\"\n")

	code, out := run(t, "cluster", "create", "-f", cfg)
	if code == 0 {
		t.Fatalf("节点是不可路由地址，不该成功（用例前提坏了），输出：\n%s", out)
	}
	if !touchedNodes(out) {
		t.Fatalf("给了 controlPlaneEndpoint 仍在校验阶段被拦下。\n输出：\n%s", out)
	}
}

// TestSC_K05_SingleMasterNeedsNoEndpoint 单 master 不该被要求配 VIP。
//
// 单节点是最常见的用法，把 controlPlaneEndpoint 变成必填等于给所有人加了一道无谓的门槛。
func TestSC_K05_SingleMasterNeedsNoEndpoint(t *testing.T) {
	// 这条必然要等 SSH 连不可路由地址超时（30 秒，超时值写在产品里）。
	// 同类用例并行跑，三次等待重叠成一次，默认测试层不至于因此多花一分半。
	t.Parallel()

	cfg := clusterConfig(t, "1.28.2", "containerd", 1, 1, "")

	_, out := run(t, "cluster", "create", "-f", cfg)
	if strings.Contains(out, "controlPlaneEndpoint") && !touchedNodes(out) {
		t.Fatalf("单 master 也被要求配 controlPlaneEndpoint：\n%s", out)
	}
	if !touchedNodes(out) {
		t.Fatalf("单 master 单 worker 的正常配置在校验阶段就被拦下了。\n输出：\n%s", out)
	}
}
