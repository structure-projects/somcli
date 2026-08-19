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

// TestSC_E14_ParallelMatchesSerial --parallel N 的结果必须与串行一致，并且真的并发了。
//
// 三条判据缺一不可：
//  1. 产物一致 —— 并发只该改变耗时，不该改变"哪台机器上有什么"；
//  2. 输出顺序确定 —— 各目标的结果按声明顺序打印。goroutine 的完成顺序是随机的，
//     照完成顺序打印会让同一份配置两次运行给出不同输出，用户没法比对两次运行的差异；
//  3. 挂钟时间明显短于串行 —— 少了这条，把 --parallel 实现成空标志（照旧串行）
//     也能让前两条绿掉，这个功能就等于没验。
//
// 每个脚本里的 sleep 是判据 3 的量具：串行是 3×sleep，并发是 1×sleep。
func TestSC_E14_ParallelMatchesSerial(t *testing.T) {
	const sleepSeconds = 4

	run := func(marker string, extra ...string) (time.Duration, string) {
		t.Helper()

		cfg := writeConfig(t, nodesYAML()+fmt.Sprintf(`
resources:
  - name: "fanned-out"
    version: "1.0"
    hosts:
      - "node-a"
      - "node-b"
      - "node-c"
    pre_install:
      - "mkdir -p %s"
    post_install:
      - "sleep %d; cat /etc/hostname > %s"
`, dirOf(marker), sleepSeconds, marker))

		args := append([]string{"install", "-f", cfg}, extra...)
		started := time.Now()
		code, out := runIn(t, t.TempDir(), args...)
		elapsed := time.Since(started)
		if code != 0 {
			t.Fatalf("安装退出码 = %d（args=%v），输出：\n%s", code, extra, out)
		}
		return elapsed, out
	}

	serialMarker := markerPath("e14-serial")
	parallelMarker := markerPath("e14-parallel")

	serialElapsed, serialOut := run(serialMarker)
	parallelElapsed, parallelOut := run(parallelMarker, "--parallel", "3")

	// 判据 1：两次运行在三个节点上都留下了同样的产物
	for _, n := range nodes {
		if got := nodeFile(t, n, serialMarker); got != n.host {
			t.Errorf("串行运行后节点 %s 上产物 = %q，期望 %q。输出：\n%s", n.host, got, n.host, serialOut)
		}
		if got := nodeFile(t, n, parallelMarker); got != n.host {
			t.Errorf("--parallel 3 后节点 %s 上产物 = %q，期望 %q。输出：\n%s", n.host, got, n.host, parallelOut)
		}
	}

	// 判据 2：并发下输出仍按 hosts 的声明顺序
	if order := hostMentionOrder(parallelOut); !isAscending(order) {
		t.Errorf("--parallel 下各节点结果的打印顺序 = %v（-1 表示没出现），期望按声明顺序递增。输出：\n%s",
			order, parallelOut)
	}

	// 判据 3：并发确实压缩了挂钟时间。阈值取串行的一半 —— 三节点理论上是 1/3，
	// 留出 ssh 建连开销的余量，同时仍然拦得住"--parallel 是空标志"。
	if parallelElapsed >= serialElapsed/2 {
		t.Errorf("--parallel 3 耗时 %v，串行耗时 %v，没有并发的迹象（每节点 sleep %ds）",
			parallelElapsed, serialElapsed, sleepSeconds)
	}
}

// TestSC_E14_ParallelKeepsPerNodeIsolation --parallel 下失败仍然只算失败那台的。
//
// 并发最容易坏的地方是错误归属：共享一个 err 变量的话，一台失败会被算到别人头上，
// 或者反过来被后完成的成功覆盖掉。所以让 node-b 单独失败，验另外两台照常走完。
func TestSC_E14_ParallelKeepsPerNodeIsolation(t *testing.T) {
	marker := markerPath("e14-isolation")
	failing := nodes[1] // node-b

	cfg := writeConfig(t, nodesYAML()+fmt.Sprintf(`
resources:
  - name: "fanned-out-with-one-bad-node"
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
      - "cat /etc/hostname > %s"
`, dirOf(marker), failing.host, marker))

	code, out := runIn(t, t.TempDir(), "install", "-f", cfg, "--parallel", "3")
	if code == 0 {
		t.Fatalf("有节点失败却退出码 0，输出：\n%s", out)
	}

	for _, n := range nodes {
		got := nodeFile(t, n, marker)
		if n.host == failing.host {
			if got != "" {
				t.Errorf("失败的节点 %s 仍然继续执行了后续阶段，产物 = %q。输出：\n%s", n.host, got, out)
			}
			continue
		}
		if got != n.host {
			t.Errorf("节点 %s 被别人的失败带累了，产物 = %q，期望 %q。输出：\n%s", n.host, got, n.host, out)
		}
	}
	if !strings.Contains(out, failing.host) {
		t.Errorf("输出没有指明是哪个节点失败的，输出：\n%s", out)
	}
}

// hostMentionOrder 返回各节点名在输出里首次出现的位置，按 nodes 的声明顺序排列。
func hostMentionOrder(out string) []int {
	order := make([]int, 0, len(nodes))
	for _, n := range nodes {
		order = append(order, strings.Index(out, n.host))
	}
	return order
}

func isAscending(v []int) bool {
	for i := 1; i < len(v); i++ {
		if v[i-1] < 0 || v[i] <= v[i-1] {
			return false
		}
	}
	return len(v) > 0 && v[0] >= 0
}
