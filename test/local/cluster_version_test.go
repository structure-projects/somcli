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

// SC-K08 的本机一半：版本号写坏了要在连节点之前拒绝。
//
// 真的装 1.28 / 1.29 / 1.30 三遍在 cluster 组（test/cluster/version_matrix_test.go），
// 这里管的是配置层面就能判定的那几种写法 —— 它们的共同点是失败会推迟到下载或
// kubeadm init，而那时节点已经被改过一遍。

// TestSC_K08_EmptyVersionRejected 不写 version 必须拒绝。
//
// 不拒绝的后果不止"init 失败"：kubeadm/kubelet/kubectl 会先按安装清单里的兜底版本
// 装到节点上，于是"配置里漏了一个键"表现成"装了个没人要求的版本，然后 init 报
// could not parse version" —— 两件事都指不到根因。
func TestSC_K08_EmptyVersionRejected(t *testing.T) {
	cfg := clusterConfig(t, "", "containerd", 1, 1, "")

	code, out := run(t, "cluster", "create", "-f", cfg)
	if code == 0 {
		t.Fatalf("没写 version 却退出 0，输出：\n%s", out)
	}
	if touchedNodes(out) {
		t.Fatalf("已经开始连节点才失败 —— 这时机器已经被改过一遍了。\n输出：\n%s", out)
	}
	// 报错里要指名是哪个键：只说"版本不能为空"的话，配置里带 version 的地方有好几处
	// （containerdVersion / cniVersion / 每条资源自己的 version），看不出该改哪个。
	if !strings.Contains(out, "k8sConfig.version") {
		t.Errorf("拒绝的理由里没指名 k8sConfig.version：\n%s", out)
	}
}

// TestSC_K08_MalformedVersionRejected 版本号形状不对的几种写法都要拒绝。
//
// configs/k8s 里每条下载地址都是 .../v{{.Version}}/...：
// 写 v1.30.0 会拼出 vv1.30.0，写 1.30 拼出 v1.30（dl.k8s.io 上没有版本线只有发布版本），
// 两者都是 404，而报错只会说"下载失败"。
func TestSC_K08_MalformedVersionRejected(t *testing.T) {
	for _, version := range []string{"v1.30.0", "1.30", "latest", "1.30.x"} {
		t.Run(version, func(t *testing.T) {
			cfg := clusterConfig(t, version, "containerd", 1, 1, "")

			code, out := run(t, "cluster", "create", "-f", cfg)
			if code == 0 {
				t.Fatalf("version: %q 却退出 0，输出：\n%s", version, out)
			}
			if touchedNodes(out) {
				t.Fatalf("已经开始连节点才失败 —— 这时机器已经被改过一遍了。\n输出：\n%s", out)
			}
			if !strings.Contains(out, "k8sConfig.version") {
				t.Errorf("拒绝的理由里没指名 k8sConfig.version：\n%s", out)
			}
		})
	}
}

// TestSC_K08_ComponentVersionWithVPrefixRejected 组件版本带 v 前缀也要拒绝。
//
// 与 k8s 版本同一个坑（URL 里已经有 v），但这几个键留空是合法的（applyK8sDefaults
// 会填默认值），所以只查非空值。
func TestSC_K08_ComponentVersionWithVPrefixRejected(t *testing.T) {
	cfg := clusterConfig(t, "1.28.2", "containerd", 1, 1,
		"      containerdVersion: \"v1.7.22\"\n")

	code, out := run(t, "cluster", "create", "-f", cfg)
	if code == 0 {
		t.Fatalf("containerdVersion 带 v 前缀却退出 0，输出：\n%s", out)
	}
	if touchedNodes(out) {
		t.Fatalf("已经开始连节点才失败 —— 这时机器已经被改过一遍了。\n输出：\n%s", out)
	}
	if !strings.Contains(out, "containerdVersion") {
		t.Errorf("拒绝的理由里没指名 containerdVersion：\n%s", out)
	}
}

// TestSC_K08_ThreeVersionsAcceptedByConfigCheck 1.28 / 1.29 / 1.30 都要能过配置校验。
//
// 与上面三条是一对：只写拒绝的话，一个"凡是 version 都报错"的实现同样能全绿。
// 这条不装集群（本机装不了），判据是走到了连节点那一步 —— 说明配置本身没被挡下来。
func TestSC_K08_ThreeVersionsAcceptedByConfigCheck(t *testing.T) {
	for _, version := range []string{"1.28.2", "1.29.8", "1.30.4"} {
		version := version
		t.Run(version, func(t *testing.T) {
			// 节点地址不可路由，SSH 要等超时，几条之间没有共享状态，并行跑。
			t.Parallel()

			cfg := clusterConfig(t, version, "containerd", 1, 1, "")

			_, out := run(t, "cluster", "create", "-f", cfg)
			if !touchedNodes(out) {
				t.Fatalf("version: %q 在连节点之前就失败了 —— 这是个合法版本。\n输出：\n%s",
					version, out)
			}
		})
	}
}
