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

// TestSC_E13_TemplateVarsRenderInScripts 脚本里的模板变量渲染成真值。
//
// 观察手法：让脚本把渲染结果 echo 到文件，再读文件。这是从外部看模板引擎的唯一办法。
// {{.WorkDir}} 必须等于 --workdir 传进去的路径 —— 否则派生目录全错，产物会落到别处。
func TestSC_E13_TemplateVarsRenderInScripts(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: tmpl-tool
    version: "9.9.9"
    post_install:
      - "echo name={{.Name}} >> {{.WorkDir}}/rendered.txt"
      - "echo version={{.Version}} >> {{.WorkDir}}/rendered.txt"
      - "echo workdir={{.WorkDir}} >> {{.WorkDir}}/rendered.txt"
      - "echo downloaddir={{.DownloadDir}} >> {{.WorkDir}}/rendered.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("安装退出码 = %d，输出：\n%s", code, out)
	}

	got := readFile(t, filepath.Join(workdir, "rendered.txt"))
	for _, want := range []string{
		"name=tmpl-tool",
		"version=9.9.9",
		"workdir=" + workdir,
		"downloaddir=" + filepath.Join(workdir, "download"),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("渲染结果缺少 %q，实际内容：\n%s", want, got)
		}
	}
}

// TestSC_E13_ShellOperatorsWork 配置里的 shell 运算符要一路活着到 sh 手上。
//
// 注意这条用例并不是 F1 的回归（html/template 只转义插值出来的内容，不动模板字面量，
// 所以字面量里的 && | 在 F1 缺陷下也是好的）——真正的 F1 回归见下一条。
// 这条守的是另一件事：脚本被当成整条命令行交给 shell，而不是被拆成 argv 直接 exec。
func TestSC_E13_ShellOperatorsWork(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: shell-tool
    version: "1.0"
    post_install:
      - "echo first > {{.WorkDir}}/chained.txt && echo second >> {{.WorkDir}}/chained.txt"
      - "echo keep-me | tr 'a-z' 'A-Z' > {{.WorkDir}}/piped.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("含 shell 运算符的脚本退出码 = %d，输出：\n%s", code, out)
	}

	if got := lines(t, filepath.Join(workdir, "chained.txt")); len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Errorf("&& 串联未生效，产物 = %v。输出：\n%s", got, out)
	}
	if got := strings.TrimSpace(readFile(t, filepath.Join(workdir, "piped.txt"))); got != "KEEP-ME" {
		t.Errorf("管道未生效，产物 = %q。输出：\n%s", got, out)
	}
}

// TestSC_E13_SpecialCharsInValuesSurviveRendering F1 的回归：渲染出的值不得被 HTML 转义。
//
// ParseStr 曾经用 html/template，而它只对插值结果动手 —— 所以必须让特殊字符出现在
// 值里，而不是模板字面量里。这里让 workdir 路径带上 ' 与 &，资源名也带上 &：
// 在缺陷版本上 {{.WorkDir}} 会渲染成 it&#39;s&amp;fine，产物落到一个没人找得到的
// 路径上，命令行看起来还一切正常。
func TestSC_E13_SpecialCharsInValuesSurviveRendering(t *testing.T) {
	workdir := filepath.Join(t.TempDir(), "it's&fine")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatalf("创建带特殊字符的 workdir 失败: %v", err)
	}

	cfg := writeConfig(t, `
resources:
  - name: amp&tool
    version: "1.0"
    post_install:
      - 'echo "name={{.Name}}" > "{{.WorkDir}}/rendered.txt"'
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("workdir 含 ' 与 & 时安装退出码 = %d，输出：\n%s", code, out)
	}

	// 文件不在这个路径上，就说明 {{.WorkDir}} 被转义了。
	got := strings.TrimSpace(readFile(t, filepath.Join(workdir, "rendered.txt")))
	if got != "name=amp&tool" {
		t.Errorf("渲染结果被转义，产物 = %q，期望 %q", got, "name=amp&tool")
	}
}

// TestSC_F06_UnknownTemplateVarFails 引用不存在的变量必须报错。
//
// text/template 对未定义字段本就返回错误，这条用例守的是"错误别在半路被吞掉"：
// 退出码要非 0，且错误信息里要出现那个变量名，用户才知道改哪儿。
func TestSC_F06_UnknownTemplateVarFails(t *testing.T) {
	t.Run("SC-F06/脚本里的未知变量", func(t *testing.T) {
		workdir := t.TempDir()
		cfg := writeConfig(t, `
resources:
  - name: bad-tmpl
    version: "1.0"
    post_install:
      - "echo {{.NoSuchVar}} > {{.WorkDir}}/never.txt"
`)

		code, out := runIn(t, workdir, "install", "-f", cfg)
		if code == 0 {
			t.Fatalf("引用不存在的模板变量却退出码 0，输出：\n%s", out)
		}
		if !strings.Contains(out, "NoSuchVar") {
			t.Errorf("错误信息未指出出错的变量名，输出：\n%s", out)
		}
		if strings.Contains(out, "[SUCCESS]") {
			t.Errorf("失败路径上打印了 [SUCCESS]，输出：\n%s", out)
		}
	})

	// {{.Workdir}} 与 {{.WorkDir}} 只差一个大小写，示例配置里曾经全写错。
	// 它必须报错而不是渲染成空串 —— 渲染成空会让产物落到 /scripts 这种根路径下。
	t.Run("SC-F06/大小写写错的 WorkDir", func(t *testing.T) {
		workdir := t.TempDir()
		cfg := writeConfig(t, `
resources:
  - name: bad-case
    version: "1.0"
    post_install:
      - "echo x > {{.Workdir}}/never.txt"
`)

		code, out := runIn(t, workdir, "install", "-f", cfg)
		if code == 0 {
			t.Fatalf("{{.Workdir}} 大小写写错却退出码 0，输出：\n%s", out)
		}
		if !strings.Contains(out, "Workdir") {
			t.Errorf("错误信息未指出出错的变量名，输出：\n%s", out)
		}
	})
}
