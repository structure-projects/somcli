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

// TestSC_E15_DefaultAbortStopsAtFirstFailure 不写 on_error 时第一个失败即中止。
//
// 默认必须是 abort：装东西是有依赖顺序的，前一个没装上还往下装，
// 后面的失败全是派生的，真正的原因被埋在一长串报错里。
func TestSC_E15_DefaultAbortStopsAtFirstFailure(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: fails
    version: "1.0"
    post_install:
      - "exit 7"
  - name: never-reached
    version: "1.0"
    post_install:
      - "echo reached > {{.WorkDir}}/reached.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("资源失败却退出码 0，输出：\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(workdir, "reached.txt")); !os.IsNotExist(err) {
		t.Errorf("默认策略下失败后仍继续装了后续资源。输出：\n%s", out)
	}
}

// TestSC_E15_ContinueKeepsGoingButStillFails on_error: continue 继续后续资源，退出码仍非 0。
//
// 两条判据缺一不可：
//   - 继续 —— 一个可选组件装不上不该阻断整份编排；
//   - 仍然非 0 —— 失败却退 0 正是 M0 清掉的那类事，CI 与外层脚本只看退出码，
//     报告成功等于把半装状态藏起来。
func TestSC_E15_ContinueKeepsGoingButStillFails(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: optional-fails
    version: "1.0"
    on_error: continue
    post_install:
      - "exit 7"
  - name: still-runs
    version: "1.0"
    post_install:
      - "echo ran > {{.WorkDir}}/ran.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("on_error: continue 下失败却退出码 0，输出：\n%s", out)
	}
	if got := strings.TrimSpace(readFile(t, filepath.Join(workdir, "ran.txt"))); got != "ran" {
		t.Errorf("on_error: continue 没有继续装后续资源，产物 = %q。输出：\n%s", got, out)
	}
	if !strings.Contains(out, "optional-fails") {
		t.Errorf("末尾没有汇总是哪个资源失败的，输出：\n%s", out)
	}
	if strings.Contains(out, "[SUCCESS] optional-fails") {
		t.Errorf("失败的资源被打印成成功，输出：\n%s", out)
	}
}

// TestSC_E15_FailedResourceIsNotRecorded on_error: continue 下失败的资源不得记入状态。
//
// 记进去的话下一次运行会跳过它，用户看到"已安装"而它从来没装成 ——
// continue 会因此从"容错"变成"永久静默地漏装一个组件"。
func TestSC_E15_FailedResourceIsNotRecorded(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: optional-fails
    version: "1.0"
    on_error: continue
    post_install:
      - "exit 7"
  - name: succeeds
    version: "1.0"
    post_install:
      - "true"
`)

	if code, out := runIn(t, workdir, "install", "-f", cfg); code == 0 {
		t.Fatalf("期望非 0 退出码，输出：\n%s", out)
	}

	code, out := runIn(t, workdir, "status")
	if code != 0 {
		t.Fatalf("status 退出码 = %d，输出：\n%s", code, out)
	}
	if strings.Contains(out, "optional-fails") {
		t.Errorf("失败的资源被记入状态，下次会被当成已安装跳过。status 输出：\n%s", out)
	}
	if !strings.Contains(out, "succeeds") {
		t.Errorf("同一轮里装成的资源没有记入状态，重跑会重复实施。status 输出：\n%s", out)
	}
}

// TestSC_E15_UnknownOnErrorValueFails on_error 写错必须报错并列出可用值。
//
// 与未知 method 同一个理由：静默当成默认值处理，写了 on_error: coninue 的用户
// 会以为容错开着，直到某次真的失败才发现整条流水线停了。
func TestSC_E15_UnknownOnErrorValueFails(t *testing.T) {
	cfg := writeConfig(t, `
resources:
  - name: typo
    version: "1.0"
    on_error: coninue
    post_install:
      - "true"
`)

	code, out := run(t, "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("on_error 取值写错却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "coninue") {
		t.Errorf("错误信息没有回显写错的取值，输出：\n%s", out)
	}
	for _, want := range []string{"abort", "continue"} {
		if !strings.Contains(out, want) {
			t.Errorf("错误信息没有列出可用值 %q，用户无从纠正，输出：\n%s", want, out)
		}
	}
}
