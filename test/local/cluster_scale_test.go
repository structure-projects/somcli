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
	"strings"
	"testing"
)

// 扩缩容里"在动手之前就该说不"的那几种情况。真正加减节点要真集群（cluster 组），
// 但下面这些判断都发生在连节点之前，本机就能验。
//
// 节点 IP 仍取 192.0.2.0/24：万一实现没拒绝，也不能让它真往某台机器上动手。

// TestSC_K12_AddUnknownNodeRejected --node 指了配置里没有的主机名时必须当场拒绝。
//
// 要加的节点得先写进配置的 nodes（用户名、密码、IP、角色都在那儿）。名字打错时
// 若不拦，产品要么去连一台不知道凭据的机器，要么静默地什么都不做还报成功。
func TestSC_K12_AddUnknownNodeRejected(t *testing.T) {
	cfg := clusterConfig(t, "1.28.2", "containerd", 1, 1, "")

	code, out := run(t, "cluster", "add-node", "-f", cfg, "--node", "w9")
	if code == 0 {
		t.Fatalf("--node 指了配置里没有的 w9 却退出 0，输出：\n%s", out)
	}
	if touchedNodes(out) {
		t.Fatalf("已经开始连节点才失败 —— 配置里有没有这台，读一遍配置就知道。\n输出：\n%s", out)
	}
	// 报错要列出可用主机名：否则用户只知道"不是这个"，不知道"那是哪个"。
	if !strings.Contains(out, "m1") || !strings.Contains(out, "w1") {
		t.Errorf("拒绝时没列出配置里可用的主机名：\n%s", out)
	}
}

// TestSC_K13_RemoveFirstMasterRejected 摘第一台 master 等于拆集群，必须拒绝并指向 cluster remove。
//
// 第一台 master 上有 admin.conf，缩容的每一步（drain、delete node）都在它上面执行；
// 摘掉它之后剩下的步骤没有地方可跑，而单 master 集群摘了它就什么都不剩了。
// 不拦的话表现是"跑了一半失败"，此时负载已经被疏散、机器已经被抹了一部分。
func TestSC_K13_RemoveFirstMasterRejected(t *testing.T) {
	cfg := clusterConfig(t, "1.28.2", "containerd", 1, 1, "")

	code, out := run(t, "cluster", "remove-node", "-f", cfg, "--node", "m1", "--force")
	if code == 0 {
		t.Fatalf("摘第一台 master 却退出 0，输出：\n%s", out)
	}
	if touchedNodes(out) {
		t.Fatalf("已经开始连节点才失败 —— 这时负载可能已经被疏散了。\n输出：\n%s", out)
	}
	if !strings.Contains(out, "cluster remove") {
		t.Errorf("没告诉用户拆整个集群该用 cluster remove：\n%s", out)
	}
}

// TestSC_K13_RemoveKnownNodeGetsToNodes 配置里有的 worker 不该被上面那条规则误伤。
//
// 与前一条成对：只写"拒绝"的话，一个"remove-node 一律拒绝"的实现也能过。
// 这条不断言最终成功（节点是不可路由地址，连不上），只断言它确实走到了连节点那一步。
func TestSC_K13_RemoveKnownNodeGetsToNodes(t *testing.T) {
	// 这条必然要等 SSH 连不可路由地址超时（30 秒，超时值写在产品里）。
	t.Parallel()

	cfg := clusterConfig(t, "1.28.2", "containerd", 1, 1, "")

	code, out := run(t, "cluster", "remove-node", "-f", cfg, "--node", "w1", "--force")
	if code == 0 {
		t.Fatalf("节点是不可路由地址，不该成功（用例前提坏了），输出：\n%s", out)
	}
	if !touchedNodes(out) {
		t.Fatalf("摘一台配置里有的 worker，却在连节点之前就被拦下了。\n输出：\n%s", out)
	}
}
