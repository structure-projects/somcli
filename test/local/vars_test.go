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
	"path/filepath"
	"strings"
	"testing"
)

// varsConfig 一份把自定义变量回显到文件的配置。
// 从外部看模板上下文只有这一种办法：让脚本把渲染结果写出来，再读文件。
const varsConfig = `
vars:
  port: "8080"
  domain: "svc.example.com"
resources:
  - name: vars-tool
    version: "1.0"
    post_install:
      - "echo port={{.Vars.port}} >> {{.WorkDir}}/vars.txt"
      - "echo domain={{.Vars.domain}} >> {{.WorkDir}}/vars.txt"
`

// TestSC_E12_ConfigVars 配置里的 vars 进入模板上下文。
//
// E3：模板上下文过去是写死的 13 个字段，编排业务服务时端口 / 域名 / 密码这类东西
// 无处可传 —— 只能把值硬编码进脚本，同一份配置换个环境就不能用了。
func TestSC_E12_ConfigVars(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, varsConfig)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("退出码 = %d，输出：\n%s", code, out)
	}

	got := readFile(t, filepath.Join(workdir, "vars.txt"))
	for _, want := range []string{"port=8080", "domain=svc.example.com"} {
		if !strings.Contains(got, want) {
			t.Errorf("渲染结果缺少 %q，实际内容：\n%s", want, got)
		}
	}
}

// TestSC_E12_SetOverridesConfigVars --set k=v 覆盖配置里的同名变量。
//
// 优先级方向必须是"命令行赢"：一份配置里放默认值、上线时用 --set 换掉，
// 反过来的话 --set 就等于没有。未在配置里声明的键也要能直接 --set 进来。
func TestSC_E12_SetOverridesConfigVars(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
vars:
  port: "8080"
resources:
  - name: vars-tool
    version: "1.0"
    post_install:
      - "echo port={{.Vars.port}} >> {{.WorkDir}}/vars.txt"
      - "echo extra={{.Vars.extra}} >> {{.WorkDir}}/vars.txt"
`)

	code, out := runIn(t, workdir,
		"--set", "port=9090",
		"--set", "extra=only-from-cli",
		"install", "-f", cfg)
	if code != 0 {
		t.Fatalf("退出码 = %d，输出：\n%s", code, out)
	}

	got := readFile(t, filepath.Join(workdir, "vars.txt"))
	if !strings.Contains(got, "port=9090") {
		t.Errorf("--set 未覆盖配置里的 port，实际内容：\n%s", got)
	}
	if strings.Contains(got, "port=8080") {
		t.Errorf("配置里的旧值仍然生效，实际内容：\n%s", got)
	}
	if !strings.Contains(got, "extra=only-from-cli") {
		t.Errorf("--set 传入的新变量未生效，实际内容：\n%s", got)
	}
}

// TestSC_E12_UndeclaredVarFails 引用没声明的自定义变量必须报错。
//
// 与 SC-F06 同一个理由：渲染成空串会让 "--port=" 这类参数悄悄丢掉，
// 或者让产物落到少了一段的路径上。上下文改成 map 之后这条尤其要守住 ——
// text/template 对 map 的缺键默认渲染 <no value> 而不是报错。
func TestSC_E12_UndeclaredVarFails(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
vars:
  port: "8080"
resources:
  - name: vars-tool
    version: "1.0"
    post_install:
      - "echo {{.Vars.nosuch}} > {{.WorkDir}}/never.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("引用未声明的 vars 却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "nosuch") {
		t.Errorf("错误信息未指出变量名，输出：\n%s", out)
	}
	if strings.Contains(out, "[SUCCESS]") {
		t.Errorf("失败路径上打印了 [SUCCESS]，输出：\n%s", out)
	}
}

// TestSC_E12_SetRejectsMalformedPair --set 不是 k=v 就报错，不静默忽略。
// 忽略的话用户以为传进去了，实际模板里那个变量根本不存在。
func TestSC_E12_SetRejectsMalformedPair(t *testing.T) {
	cfg := writeConfig(t, varsConfig)

	code, out := runIn(t, t.TempDir(), "--set", "port", "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("--set port（缺 =）却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "k=v") {
		t.Errorf("错误信息没说清正确写法，输出：\n%s", out)
	}
}
