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
	"reflect"
	"strings"
	"testing"
)

// TestSC_E01_InstallSingleResource 本机安装单个资源。
//
// 这是产品最核心的一条路径：resources 里声明什么，就在目标上发生什么。
// 判据不是"退出码 0"，而是脚本真的跑了 —— post_install 写一个带内容的标记文件，
// 用例读它。少了这个正向产物断言，一个什么都不做的实现也能让用例全绿。
func TestSC_E01_InstallSingleResource(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: single-tool
    version: "1.2.3"
    post_install:
      - "echo single-tool-1.2.3 > {{.WorkDir}}/installed.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("安装单个资源退出码 = %d，输出：\n%s", code, out)
	}

	got := strings.TrimSpace(readFile(t, filepath.Join(workdir, "installed.txt")))
	if got != "single-tool-1.2.3" {
		t.Errorf("post_install 产物内容 = %q，期望 %q。输出：\n%s", got, "single-tool-1.2.3", out)
	}
}

// TestSC_E02_ResourcesRunInDeclaredOrder resources 数组顺序即执行顺序。
//
// 这是配置的语义承诺：安装 kubelet 之前得先装 containerd，用户靠数组顺序表达依赖。
// 三个资源各往同一个文件追加一行，行序就是执行序。名字故意不按字典序声明
// （gamma → alpha → beta），否则遍历顺序被改成排序也照样通过。
func TestSC_E02_ResourcesRunInDeclaredOrder(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: gamma
    version: "1.0"
    post_install:
      - "echo gamma >> {{.WorkDir}}/order.txt"
  - name: alpha
    version: "1.0"
    post_install:
      - "echo alpha >> {{.WorkDir}}/order.txt"
  - name: beta
    version: "1.0"
    post_install:
      - "echo beta >> {{.WorkDir}}/order.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("多资源安装退出码 = %d，输出：\n%s", code, out)
	}

	want := []string{"gamma", "alpha", "beta"}
	got := lines(t, filepath.Join(workdir, "order.txt"))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("执行顺序 = %v，期望与 resources 声明顺序一致 %v。输出：\n%s", got, want, out)
	}
}

// TestSC_E02_ScriptsWithinResourceRunInOrder 同一资源内 pre_install 先于 post_install，
// 且各自内部按数组顺序执行。
func TestSC_E02_ScriptsWithinResourceRunInOrder(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: staged
    version: "1.0"
    pre_install:
      - "echo pre-1 >> {{.WorkDir}}/order.txt"
      - "echo pre-2 >> {{.WorkDir}}/order.txt"
    post_install:
      - "echo post-1 >> {{.WorkDir}}/order.txt"
      - "echo post-2 >> {{.WorkDir}}/order.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("安装退出码 = %d，输出：\n%s", code, out)
	}

	want := []string{"pre-1", "pre-2", "post-1", "post-2"}
	got := lines(t, filepath.Join(workdir, "order.txt"))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("脚本执行顺序 = %v，期望 %v。输出：\n%s", got, want, out)
	}
}

// TestSC_E07_InstallByNameOnlyTouchesNamedResource install -n 只装被点名的那个。
//
// 负向断言是重点：未被点名的资源一个都不许执行。InstallTool 曾经在命中之后不 return，
// 继续遍历剩余资源（虽然只有命中的会装），而入口根本没有接上 -n —— 函数写了却零调用点，
// 与 SetNode 断链是同一类缺陷。这里断言 b 有产物、a 与 c 无产物。
func TestSC_E07_InstallByNameOnlyTouchesNamedResource(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: tool-a
    version: "1.0"
    post_install:
      - "echo a > {{.WorkDir}}/a.txt"
  - name: tool-b
    version: "1.0"
    post_install:
      - "echo b > {{.WorkDir}}/b.txt"
  - name: tool-c
    version: "1.0"
    post_install:
      - "echo c > {{.WorkDir}}/c.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg, "-n", "tool-b")
	if code != 0 {
		t.Fatalf("按名安装退出码 = %d，输出：\n%s", code, out)
	}

	if got := strings.TrimSpace(readFile(t, filepath.Join(workdir, "b.txt"))); got != "b" {
		t.Errorf("被点名的资源产物内容 = %q，期望 %q", got, "b")
	}
	for _, name := range []string{"a.txt", "c.txt"} {
		if _, err := os.Stat(filepath.Join(workdir, name)); !os.IsNotExist(err) {
			t.Errorf("未被点名的资源仍然执行了，产生了 %s。输出：\n%s", name, out)
		}
	}
}

// TestSC_E07_InstallByUnknownNameFails 点名一个不存在的资源必须报错。
// 静默什么都不做是更坏的结果：用户以为装上了，实际拼错了名字。
func TestSC_E07_InstallByUnknownNameFails(t *testing.T) {
	cfg := writeConfig(t, `
resources:
  - name: tool-a
    version: "1.0"
    post_install:
      - "true"
`)

	code, out := run(t, "install", "-f", cfg, "-n", "no-such-tool")
	if code == 0 {
		t.Fatalf("点名不存在的资源却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "no-such-tool") {
		t.Errorf("错误信息未指出拼错的资源名，输出：\n%s", out)
	}
	if !strings.Contains(out, "tool-a") {
		t.Errorf("错误信息未列出已声明的资源，用户无从纠正，输出：\n%s", out)
	}
	if strings.Contains(out, "[SUCCESS]") {
		t.Errorf("失败路径上打印了 [SUCCESS]，输出：\n%s", out)
	}
}
