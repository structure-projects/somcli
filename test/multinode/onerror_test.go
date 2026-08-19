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
)

// TestSC_F05_DefaultAbortStopsAtFailingNode 默认策略下一台节点失败即中止整份编排。
//
// 判据落在**后续阶段**而不是失败那台：一个节点的 pre_install 没跑完，
// 它上面的 post_install 当然不该跑；但更要紧的是别的节点也不该往下走 ——
// 多节点部署里各节点是一个整体，让两台走到第三步、一台停在第二步，
// 收场时是个没人描述过的半成品状态。
func TestSC_F05_DefaultAbortStopsAtFailingNode(t *testing.T) {
	marker := markerPath("f05-abort")
	failing := nodes[1] // node-b

	cfg := writeConfig(t, nodesYAML()+fmt.Sprintf(`
resources:
  - name: "aborts"
    version: "1.0"
    hosts:
      - "node-a"
      - "node-b"
      - "node-c"
    pre_install:
      - "mkdir -p %s"
      - 'test "$(cat /etc/hostname)" != %s'
    post_install:
      - "cat /etc/hostname > %s"
`, dirOf(marker), failing.host, marker))

	code, out := runIn(t, t.TempDir(), "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("有节点失败却退出码 0，输出：\n%s", out)
	}
	for _, n := range nodes {
		if got := nodeFile(t, n, marker); got != "" {
			t.Errorf("默认 abort 下节点 %s 仍执行了后续阶段，产物 = %q。输出：\n%s", n.host, got, out)
		}
	}
	if !strings.Contains(out, failing.host) {
		t.Errorf("输出没有指明是哪个节点失败的，输出：\n%s", out)
	}
}

// TestSC_E15_ContinueFinishesSurvivingNodes on_error: continue 下剩下的节点走完整个资源。
//
// "继续"的含义必须是**继续走完后面的阶段**，而不是只把后面的资源接着装：
// 一个节点在 pre_install 挂了就让另外两个节点也拿不到 post_install 的话，
// continue 与 abort 在单资源配置里没有任何区别，这个策略就是个空设置。
//
// 退出码仍然非 0 —— 这是 M0 清掉的那类事，CI 只看退出码，退 0 等于把失败节点藏起来。
func TestSC_E15_ContinueFinishesSurvivingNodes(t *testing.T) {
	preMarker := markerPath("e15-pre")
	postMarker := markerPath("e15-post")
	failing := nodes[1] // node-b

	cfg := writeConfig(t, nodesYAML()+fmt.Sprintf(`
resources:
  - name: "tolerates-one-node"
    version: "1.0"
    on_error: continue
    hosts:
      - "node-a"
      - "node-b"
      - "node-c"
    pre_install:
      - "mkdir -p %s && mkdir -p %s"
      - "cat /etc/hostname > %s"
      - 'test "$(cat /etc/hostname)" != %s'
    post_install:
      - "cat /etc/hostname > %s"
`, dirOf(preMarker), dirOf(postMarker), preMarker, failing.host, postMarker))

	code, out := runIn(t, t.TempDir(), "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("on_error: continue 下有节点失败却退出码 0，输出：\n%s", out)
	}

	for _, n := range nodes {
		// 失败之前的那一步在三台上都该跑过 —— 否则失败点判断错了，用例验的不是这件事
		if got := nodeFile(t, n, preMarker); got != n.host {
			t.Fatalf("节点 %s 在失败点之前的脚本没跑，产物 = %q。输出：\n%s", n.host, got, out)
		}

		got := nodeFile(t, n, postMarker)
		if n.host == failing.host {
			if got != "" {
				t.Errorf("失败的节点 %s 却继续执行了 post_install，产物 = %q。输出：\n%s", n.host, got, out)
			}
			continue
		}
		if got != n.host {
			t.Errorf("on_error: continue 下存活节点 %s 没有走完整个资源，产物 = %q。输出：\n%s", n.host, got, out)
		}
	}
}

// TestSC_E16_StateRecordsOnlySurvivingNodes 部分节点失败时状态只记装成的那些。
//
// 这是逐节点记账真正的用处：记成"这个资源装好了"的话，下次 install 会把失败那台
// 一起跳过，用户看到"已安装"而那台机器上什么都没有 —— 半装状态从此再也修不回来。
// 反过来，存活节点必须记上，否则重跑会在它们上面重复实施。
func TestSC_E16_StateRecordsOnlySurvivingNodes(t *testing.T) {
	workdir := t.TempDir()
	marker := markerPath("e16-partial")
	failing := nodes[1] // node-b

	cfg := writeConfig(t, nodesYAML()+fmt.Sprintf(`
resources:
  - name: "partially-installed"
    version: "1.0"
    on_error: continue
    hosts:
      - "node-a"
      - "node-b"
      - "node-c"
    pre_install:
      - "mkdir -p %s"
      - 'test "$(cat /etc/hostname)" != %s'
    post_install:
      - "cat /etc/hostname >> %s"
`, dirOf(marker), failing.host, marker))

	if code, out := runIn(t, workdir, "install", "-f", cfg); code == 0 {
		t.Fatalf("有节点失败却退出码 0，输出：\n%s", out)
	}

	code, out := runIn(t, workdir, "status")
	if code != 0 {
		t.Fatalf("status 退出码 = %d，输出：\n%s", code, out)
	}
	if strings.Contains(out, failing.host) {
		t.Errorf("失败的节点 %s 被记入状态，下次 install 会跳过它。status 输出：\n%s", failing.host, out)
	}
	for _, n := range nodes {
		if n.host == failing.host {
			continue
		}
		if !strings.Contains(out, n.host) {
			t.Errorf("装成的节点 %s 没有记入状态，重跑会重复实施。status 输出：\n%s", n.host, out)
		}
	}

	// 后果验到底：重跑只该动没装成的那台。产物用追加写，行数即执行次数。
	code, out = runIn(t, workdir, "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("失败节点仍未修好，重跑却退出码 0，输出：\n%s", out)
	}
	for _, n := range nodes {
		if n.host == failing.host {
			continue
		}
		if got := strings.Split(nodeFile(t, n, marker), "\n"); len(got) != 1 {
			t.Errorf("已记账的节点 %s 被重复实施了 %d 次（%v）。输出：\n%s", n.host, len(got), got, out)
		}
	}
}
