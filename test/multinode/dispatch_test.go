//go:build multinode

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

package multinode

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestSC_E04_AllNodesInstall 一个资源声明三个节点，三个节点都要真的装上。
//
// 每个节点把自己的 /etc/hostname 写进产物：只断言"文件存在"的话，三次都跑在同一台机器上
// 也能过；比对主机名才能确认脚本分别落在了各自的节点。
func TestSC_E04_AllNodesInstall(t *testing.T) {
	marker := markerPath("e04")

	cfg := writeConfig(t, nodesYAML()+fmt.Sprintf(`
resources:
  - name: "all-nodes"
    version: "1.0"
    hosts:
      - "node-a"
      - "node-b"
      - "node-c"
    pre_install:
      - "mkdir -p %s"
    post_install:
      - "cat /etc/hostname > %s"
`, dirOf(marker), marker))

	code, out := runIn(t, t.TempDir(), "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("三节点安装失败，退出码 = %d，输出：\n%s", code, out)
	}

	for _, n := range nodes {
		if got := nodeFile(t, n, marker); got != n.host {
			t.Errorf("节点 %s 上的产物是 %q，期望 %q —— 脚本没有分别在各自节点上执行\n输出：\n%s",
				n.host, got, n.host, out)
		}
	}
}

// TestSC_E05_HostsTargetingIsExclusive hosts 点名谁就只装谁 —— 这是 D1 的终极回归。
//
// D1 是 cluster.LoadConfig 没把节点登记进全局节点表，于是声明为远程的安装被当作本机执行：
// 用户以为装到了节点上，实际全落在操作机。所以判据是三条负向断言一起成立：
// 未点名的两个节点没有产物、操作机也没有产物 —— 只验"目标节点有产物"是拦不住这个缺陷的。
func TestSC_E05_HostsTargetingIsExclusive(t *testing.T) {
	marker := markerPath("e05")
	target := nodes[1] // node-b

	cfg := writeConfig(t, nodesYAML()+fmt.Sprintf(`
resources:
  - name: "only-node-b"
    version: "1.0"
    hosts:
      - %q
    pre_install:
      - "mkdir -p %s"
    post_install:
      - "cat /etc/hostname > %s"
`, target.host, dirOf(marker), marker))

	code, out := runIn(t, t.TempDir(), "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("定向安装失败，退出码 = %d，输出：\n%s", code, out)
	}

	if got := nodeFile(t, target, marker); got != target.host {
		t.Fatalf("被点名的节点 %s 上产物是 %q，期望 %q\n输出：\n%s", target.host, got, target.host, out)
	}
	for _, n := range nodes {
		if n.host == target.host {
			continue
		}
		if got := nodeFile(t, n, marker); got != "" {
			t.Errorf("hosts 只点名了 %s，节点 %s 上却有产物 %q", target.host, n.host, got)
		}
	}
	if localFileExists(marker) {
		t.Errorf("声明为远程的安装落在了操作机上（D1 复现）：%s", marker)
	}
}

// TestSC_E06_MixedLocalAndRemote 同一份配置里本机与远程混排，各自落到该落的地方。
//
// 不声明 hosts 的资源在操作机执行，声明了的在节点执行。两个方向都要验：
// 只验一个方向的话，"全都在操作机跑"或"全都甩给节点"都可能蒙过去。
func TestSC_E06_MixedLocalAndRemote(t *testing.T) {
	localMarker := markerPath("e06-local")
	remoteMarker := markerPath("e06-remote")
	target := nodes[0] // node-a

	cfg := writeConfig(t, nodesYAML()+fmt.Sprintf(`
resources:
  - name: "on-operator"
    version: "1.0"
    post_install:
      - "mkdir -p %s"
      - "echo operator > %s"
  - name: "on-node-a"
    version: "1.0"
    hosts:
      - %q
    pre_install:
      - "mkdir -p %s"
    post_install:
      - "cat /etc/hostname > %s"
`, dirOf(localMarker), localMarker, target.host, dirOf(remoteMarker), remoteMarker))

	code, out := runIn(t, t.TempDir(), "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("混合编排失败，退出码 = %d，输出：\n%s", code, out)
	}

	// 未声明 hosts 的那个：操作机上有，节点上没有。
	if !localFileExists(localMarker) {
		t.Errorf("未声明 hosts 的资源没有在操作机上执行：缺 %s\n输出：\n%s", localMarker, out)
	}
	if got := nodeFile(t, target, localMarker); got != "" {
		t.Errorf("未声明 hosts 的资源却跑到了节点 %s 上，产物 %q", target.host, got)
	}

	// 声明了 hosts 的那个：节点上有，操作机上没有。
	if got := nodeFile(t, target, remoteMarker); got != target.host {
		t.Errorf("节点 %s 上产物是 %q，期望 %q\n输出：\n%s", target.host, got, target.host, out)
	}
	if localFileExists(remoteMarker) {
		t.Errorf("声明为远程的资源落在了操作机上（D1 复现）：%s", remoteMarker)
	}
}

// markerPath 每次运行都换路径：固定路径会被上一次的残留污染，
// 那样负向断言（"这里不该有文件"）就变成了恒红或恒绿。
func markerPath(name string) string {
	return fmt.Sprintf("/tmp/somcli-%s-%d/marker.txt", name, time.Now().UnixNano())
}

func dirOf(path string) string {
	if i := strings.LastIndex(path, "/"); i > 0 {
		return path[:i]
	}
	return "."
}
