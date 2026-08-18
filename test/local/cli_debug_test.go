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

// okConfig 是一份能跑通的最小配置：不下载任何东西，只跑两条空脚本。
// 用它当载体，是为了让用例只观察 --debug 的效果，不掺进失败路径的噪声。
func okConfig(t *testing.T) string {
	t.Helper()
	return writeConfig(t, `
resources:
  - name: jq
    version: "1.7.1"
    pre_install:
      - "true"
    post_install:
      - "true"
`)
}

// TestSC_X02_DebugFlagEmitsDiagnostics --debug 的验收判据是"真的多出诊断信息"。
//
// 只检查标志被注册是不够的：标志与输出开关之间少一句 SetDebugMode 就会静默失效，
// 而这种失效恰好在最需要排查的时候才会被发现。所以这里跑真二进制看输出。
func TestSC_X02_DebugFlagEmitsDiagnostics(t *testing.T) {
	cfg := okConfig(t)

	code, out := run(t, "install", "-f", cfg, "--debug")
	if code != 0 {
		t.Fatalf("带 --debug 的正常安装退出码 = %d，输出：\n%s", code, out)
	}
	if !strings.Contains(out, "[DEBUG]") {
		t.Errorf("传了 --debug 却没有任何 [DEBUG] 输出，输出：\n%s", out)
	}
}

// TestSC_X02_WithoutDebugStaysQuiet 不传 --debug 时必须安静。
// 诊断输出里带工作目录、配置内容甚至节点连接信息，默认打开会淹没正常日志。
func TestSC_X02_WithoutDebugStaysQuiet(t *testing.T) {
	cfg := okConfig(t)

	code, out := run(t, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("正常安装退出码 = %d，输出：\n%s", code, out)
	}
	if strings.Contains(out, "[DEBUG]") {
		t.Errorf("未传 --debug 仍打印了诊断日志，输出：\n%s", out)
	}
}

// TestSC_X02_DebugIsGlobalFlag --debug 必须是全局标志。
// 若只挂在根命令上而子命令不继承，`somcli install --debug` 会被判为未知标志，
// 用户在最需要它的场景下反而用不了。
func TestSC_X02_DebugIsGlobalFlag(t *testing.T) {
	code, out := run(t, "--help")
	if code != 0 {
		t.Fatalf("--help 退出码 = %d，输出：\n%s", code, out)
	}
	if !strings.Contains(out, "--debug") {
		t.Errorf("根命令帮助未列出 --debug：\n%s", out)
	}

	for _, sub := range []string{"install", "download", "version"} {
		sub := sub
		t.Run("SC-X02/"+sub, func(t *testing.T) {
			code, out := run(t, sub, "--help")
			if code != 0 {
				t.Fatalf("%s --help 退出码 = %d，输出：\n%s", sub, code, out)
			}
			if !strings.Contains(out, "--debug") {
				t.Errorf("子命令 %s 的帮助未列出继承来的 --debug：\n%s", sub, out)
			}
		})
	}
}
