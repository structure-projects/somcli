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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 这一组验的是"集群装什么"已经从代码里搬到配置里了。
//
// 为什么能在本机验：装的过程要真节点，但"这套配置引用了什么、引用得对不对"是
// 连节点之前就该问清楚的事 —— 与 cluster_reject_test.go 同一个道理。
// 节点 IP 同样取 192.0.2.0/24（RFC 5737 TEST-NET-1，不可路由）。

// catalogClusterConfig 生成一份指定了 resources 的 k8s 集群配置。
func catalogClusterConfig(t *testing.T, resources string) string {
	t.Helper()

	return writeConfig(t, `cluster:
  - type: "k8s"
    name: "catalog-probe"
    nodes:
      - host: "m1"
        ip: "192.0.2.10"
        role: "master"
        user: "root"
        sshKey: "~/.ssh/id_rsa"
    k8sConfig:
      version: "1.28.2"
      containerRuntime: "containerd"
      podNetworkCidr: "10.244.0.0/16"
      serviceCidr: "10.96.0.0/12"
      resources: `+resources+`
`)
}

// TestSC_K16_ResourcesKeyTakesEffect k8sConfig.resources 必须真的被消费。
//
// 这个键在配置样例里一直写着，但类型里根本没有它 —— yaml 解析用的是 UnmarshalStrict，
// 于是配了它的配置直接解析失败；换成非严格解析也只是静默忽略。两种下场都一样：
// 用户以为自己指定了装什么，实际装的是代码里写死的那一套。
//
// 判据取"引用一个不存在的名字必须在连节点之前被拒绝"：
//   - 退出 0 或走到连节点，说明这个键仍然没人读；
//   - 报错里得列出可用的名字，否则拼错一个字母只能看着退出码猜。
func TestSC_K16_ResourcesKeyTakesEffect(t *testing.T) {
	cfg := catalogClusterConfig(t, `["base-dependencies", "nosuchresource"]`)

	code, out := run(t, "cluster", "create", "-f", cfg)
	if code == 0 {
		t.Fatalf("引用了不存在的资源却退出 0 —— resources 这个键仍然没人读，输出：\n%s", out)
	}
	if touchedNodes(out) {
		t.Fatalf("已经开始连节点才失败。名字对不对是配置层面就能判定的，"+
			"应在动手前拒绝，否则前面几样已经装到节点上了。\n输出：\n%s", out)
	}
	if !strings.Contains(out, "nosuchresource") {
		t.Errorf("报错里没点出是哪个名字不对：\n%s", out)
	}
	// 可用名单是"拼错一个字母"时唯一能自救的信息。
	for _, name := range []string{"containerd", "kubernetes", "flannel"} {
		if !strings.Contains(out, name) {
			t.Errorf("报错里没列出可用资源 %s，用户无从对照：\n%s", name, out)
		}
	}
}

// TestSC_K16_DefaultResourcesStillWork 不写 resources 时仍按默认组合装。
//
// 与上一条成对：只验"拒绝"的话，一个"凡是 resources 一律拒绝"甚至
// "凡是集群配置一律拒绝"的实现同样能过。
//
// 不断言最终成功（节点不可路由，必然连不上），只断言流程确实走到了连节点那一步。
func TestSC_K16_DefaultResourcesStillWork(t *testing.T) {
	// 这条要等 SSH 连不可路由地址超时（30 秒，超时值写在产品里）。
	t.Parallel()

	cfg := writeConfig(t, `cluster:
  - type: "k8s"
    name: "catalog-default"
    nodes:
      - host: "m1"
        ip: "192.0.2.11"
        role: "master"
        user: "root"
        sshKey: "~/.ssh/id_rsa"
    k8sConfig:
      version: "1.28.2"
      containerRuntime: "containerd"
      podNetworkCidr: "10.244.0.0/16"
      serviceCidr: "10.96.0.0/12"
`)

	code, out := run(t, "cluster", "create", "-f", cfg)
	if code == 0 {
		t.Fatalf("节点是不可路由地址，不该成功（用例前提坏了），输出：\n%s", out)
	}
	if !touchedNodes(out) {
		t.Fatalf("没走到连节点那一步就结束了 —— 不写 resources 的常规配置被拦下了。\n输出：\n%s", out)
	}
}

// TestSC_K17_CatalogOverrideReplacesBuiltin SOMCLI_K8S_CATALOG 指定的目录必须**取代**内置清单。
//
// 这是"不重新编译也能改装什么"的唯一出口：安装内容是 go:embed 进二进制的
// （install.sh 只拷一个二进制，从来不分发 configs/），没有这个出口的话，
// 想换一条 sed 就得自己编译一份 somcli。
//
// 判据取"取代"而不是"合并"：
//   - 目录里自己定义的名字要能被引用（否则这个开关是假的）；
//   - 内置有、目录里没有的名字必须被拒绝（否则用户以为自己换掉了 containerd 的装法，
//     实际用的还是内置那份，而两者的差别要到节点上才暴露）。
func TestSC_K17_CatalogOverrideReplacesBuiltin(t *testing.T) {
	dir := t.TempDir()
	writeCatalogFile(t, dir, "mine.yaml", `resources:
  - name: "mine"
    version: "1.0"
    post_install:
      - "true"
`)
	// 网络插件也得有：cni 默认是 flannel，而校验会连它一起查 ——
	// 换掉整份清单却没带 CNI 的话，失败会推迟到 kubeadm init 之后。
	writeCatalogFile(t, dir, "flannel.yaml", `resources:
  - name: "flannel"
    version: "0.25.6"
    method: "manifest"
    urls:
      - "https://example.invalid/kube-flannel.yml"
    target: "{{.Filename}}"
`)
	env := []string{"SOMCLI_K8S_CATALOG=" + dir}

	t.Run("目录里定义的名字可用", func(t *testing.T) {
		// 这条要等 SSH 连不可路由地址超时（30 秒）。
		t.Parallel()

		cfg := catalogClusterConfig(t, `["mine"]`)
		code, out := runEnvIn(t, t.TempDir(), env, "cluster", "create", "-f", cfg)
		if code == 0 {
			t.Fatalf("节点是不可路由地址，不该成功（用例前提坏了），输出：\n%s", out)
		}
		if !touchedNodes(out) {
			t.Fatalf("自定义清单里明明有 mine，却在校验阶段被拦下 —— "+
				"SOMCLI_K8S_CATALOG 没被读到。\n输出：\n%s", out)
		}
	})

	t.Run("内置有而目录里没有的名字被拒绝", func(t *testing.T) {
		t.Parallel()

		cfg := catalogClusterConfig(t, `["containerd"]`)
		code, out := runEnvIn(t, t.TempDir(), env, "cluster", "create", "-f", cfg)
		if code == 0 || touchedNodes(out) {
			t.Fatalf("指定了替代清单，containerd 却仍然装得下去 —— "+
				"说明用的还是内置那份，用户的改动完全没生效。\n输出：\n%s", out)
		}
		if !strings.Contains(out, "mine") {
			t.Errorf("报错里没列出替代清单里的 mine，看不出当前用的是哪一份：\n%s", out)
		}
	})
}

// TestSC_K17_BadCatalogDirRejected 指了一个不存在的目录必须当场说清楚。
//
// 静默退回内置清单是最坏的处理：用户以为自己换掉了安装内容，装出来的却是内置那份。
func TestSC_K17_BadCatalogDirRejected(t *testing.T) {
	cfg := catalogClusterConfig(t, `["base-dependencies"]`)
	missing := filepath.Join(t.TempDir(), "not-there")

	code, out := runEnvIn(t, t.TempDir(), []string{"SOMCLI_K8S_CATALOG=" + missing},
		"cluster", "create", "-f", cfg)
	if code == 0 {
		t.Fatalf("SOMCLI_K8S_CATALOG 指向不存在的目录却退出 0，输出：\n%s", out)
	}
	if touchedNodes(out) {
		t.Fatalf("清单目录都读不到就已经开始连节点了。\n输出：\n%s", out)
	}
	if !strings.Contains(out, "SOMCLI_K8S_CATALOG") {
		t.Errorf("报错里没提是哪个开关有问题：\n%s", out)
	}
}

// writeCatalogFile 往自定义清单目录里落一个文件。
func writeCatalogFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("写清单文件 %s 失败: %v", name, err)
	}
}
