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

// TestSC_E10_UninstallRunsRemoveScriptsInReverseOrder 卸载按声明逆序执行。
//
// E2：remove_scripts 字段过去零消费，根本没有 uninstall 入口 —— 装上去的东西只能手工拆。
//
// 逆序不是风格问题：配置是按依赖顺序写的（先容器运行时、再 k8s 组件），
// 顺着拆会先删掉被依赖的东西，后面的卸载脚本就没法执行了。
// 三个资源各追加一行，行序即执行序；名字故意不按字典序声明，避免"排序"也能碰巧通过。
func TestSC_E10_UninstallRunsRemoveScriptsInReverseOrder(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: gamma
    version: "1.0"
    remove_scripts:
      - "echo gamma >> {{.WorkDir}}/order.txt"
  - name: alpha
    version: "1.0"
    remove_scripts:
      - "echo alpha >> {{.WorkDir}}/order.txt"
  - name: beta
    version: "1.0"
    remove_scripts:
      - "echo beta >> {{.WorkDir}}/order.txt"
`)

	code, out := runIn(t, workdir, "uninstall", "-f", cfg)
	if code != 0 {
		t.Fatalf("卸载退出码 = %d，输出：\n%s", code, out)
	}

	want := []string{"beta", "alpha", "gamma"}
	if got := lines(t, filepath.Join(workdir, "order.txt")); !reflect.DeepEqual(got, want) {
		t.Errorf("卸载顺序 = %v，期望与声明顺序相反 %v。输出：\n%s", got, want, out)
	}
}

// TestSC_E10_UninstallScriptsWithinResourceRunInOrder 同一资源内的 remove_scripts 顺序执行。
//
// 与资源之间的逆序相反：一个资源的 remove_scripts 本身就是一段"卸载步骤"
// （先停服务、再删文件、最后删用户），倒过来跑就错了。
func TestSC_E10_UninstallScriptsWithinResourceRunInOrder(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: staged
    version: "1.0"
    remove_scripts:
      - "echo step-1 >> {{.WorkDir}}/order.txt"
      - "echo step-2 >> {{.WorkDir}}/order.txt"
      - "echo step-3 >> {{.WorkDir}}/order.txt"
`)

	code, out := runIn(t, workdir, "uninstall", "-f", cfg)
	if code != 0 {
		t.Fatalf("卸载退出码 = %d，输出：\n%s", code, out)
	}

	want := []string{"step-1", "step-2", "step-3"}
	if got := lines(t, filepath.Join(workdir, "order.txt")); !reflect.DeepEqual(got, want) {
		t.Errorf("remove_scripts 执行顺序 = %v，期望 %v。输出：\n%s", got, want, out)
	}
}

// TestSC_E10_UninstallLeavesCleanEnvironment 装完再卸，安装落下的东西要被清掉。
//
// 这是"环境干净"的完整回路：install 建目录放文件，uninstall 用自己的 remove_scripts 拆掉。
// 只验其中一半的话，remove_scripts 写错路径这种事在用例里看不出来。
func TestSC_E10_UninstallLeavesCleanEnvironment(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: leaves-files
    version: "1.0"
    post_install:
      - "mkdir -p {{.WorkDir}}/opt/tool"
      - "echo payload > {{.WorkDir}}/opt/tool/data.txt"
    remove_scripts:
      - "rm -rf {{.WorkDir}}/opt/tool"
`)

	if code, out := runIn(t, workdir, "install", "-f", cfg); code != 0 {
		t.Fatalf("安装退出码 = %d，输出：\n%s", code, out)
	}
	installed := filepath.Join(workdir, "opt", "tool", "data.txt")
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("安装没有落下预期产物: %v", err)
	}

	code, out := runIn(t, workdir, "uninstall", "-f", cfg)
	if code != 0 {
		t.Fatalf("卸载退出码 = %d，输出：\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(workdir, "opt", "tool")); !os.IsNotExist(err) {
		t.Errorf("卸载后产物仍在 %s。输出：\n%s", installed, out)
	}
}

// TestSC_E10_UninstallByNameOnlyTouchesNamedResource uninstall -n 只拆被点名的那个。
//
// 与 install -n 对称。负向断言是重点：一条 uninstall 命令误拆掉整份配置里的其他组件，
// 后果比装错严重得多。
func TestSC_E10_UninstallByNameOnlyTouchesNamedResource(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: tool-a
    version: "1.0"
    remove_scripts:
      - "echo a > {{.WorkDir}}/a.txt"
  - name: tool-b
    version: "1.0"
    remove_scripts:
      - "echo b > {{.WorkDir}}/b.txt"
  - name: tool-c
    version: "1.0"
    remove_scripts:
      - "echo c > {{.WorkDir}}/c.txt"
`)

	code, out := runIn(t, workdir, "uninstall", "-f", cfg, "-n", "tool-b")
	if code != 0 {
		t.Fatalf("按名卸载退出码 = %d，输出：\n%s", code, out)
	}

	if got := strings.TrimSpace(readFile(t, filepath.Join(workdir, "b.txt"))); got != "b" {
		t.Errorf("被点名的资源卸载产物 = %q，期望 %q", got, "b")
	}
	for _, name := range []string{"a.txt", "c.txt"} {
		if _, err := os.Stat(filepath.Join(workdir, name)); !os.IsNotExist(err) {
			t.Errorf("未被点名的资源也被卸载了，产生了 %s。输出：\n%s", name, out)
		}
	}
}

// TestSC_E10_UninstallByUnknownNameFails 点名一个不存在的资源必须报错。
func TestSC_E10_UninstallByUnknownNameFails(t *testing.T) {
	cfg := writeConfig(t, `
resources:
  - name: tool-a
    version: "1.0"
    remove_scripts:
      - "true"
`)

	code, out := run(t, "uninstall", "-f", cfg, "-n", "no-such-tool")
	if code == 0 {
		t.Fatalf("点名不存在的资源却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "no-such-tool") {
		t.Errorf("错误信息未指出拼错的资源名，输出：\n%s", out)
	}
	if !strings.Contains(out, "tool-a") {
		t.Errorf("错误信息未列出已声明的资源，用户无从纠正，输出：\n%s", out)
	}
}

// TestSC_E10_UninstallWithoutRemoveScriptsWarns 没写 remove_scripts 的资源跳过但要出声。
//
// 不报错：一份配置里往往只有一部分资源需要卸载动作，为此让整个 uninstall 失败
// 会逼用户给每个资源都写一句 true。但必须说出来 ——
// 否则"卸载成功"与"什么都没卸"在输出上没有区别，用户会以为环境已经干净了。
func TestSC_E10_UninstallWithoutRemoveScriptsWarns(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: no-remove
    version: "1.0"
    post_install:
      - "true"
  - name: has-remove
    version: "1.0"
    remove_scripts:
      - "echo removed > {{.WorkDir}}/removed.txt"
`)

	code, out := runIn(t, workdir, "uninstall", "-f", cfg)
	if code != 0 {
		t.Fatalf("卸载退出码 = %d，输出：\n%s", code, out)
	}
	if !strings.Contains(out, "no-remove") {
		t.Errorf("跳过的资源没有出现在输出里，输出：\n%s", out)
	}
	if !strings.Contains(out, "[WARNING]") {
		t.Errorf("跳过一个资源却没有任何警告，输出：\n%s", out)
	}
	// 跳过一个不影响其余资源继续卸载
	if got := strings.TrimSpace(readFile(t, filepath.Join(workdir, "removed.txt"))); got != "removed" {
		t.Errorf("跳过之后的资源没有继续卸载，产物 = %q", got)
	}
}

// TestSC_E10_UninstallFailureStopsAndFailsLoudly 某个资源卸载失败要非 0 退出。
//
// 半拆状态必须让人看见：报告成功的话用户会接着去重装，而残留的旧文件正是最难查的一类故障。
func TestSC_E10_UninstallFailureStopsAndFailsLoudly(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: never-reached
    version: "1.0"
    remove_scripts:
      - "echo reached > {{.WorkDir}}/reached.txt"
  - name: fails
    version: "1.0"
    remove_scripts:
      - "exit 3"
`)

	code, out := runIn(t, workdir, "uninstall", "-f", cfg)
	if code == 0 {
		t.Fatalf("卸载脚本失败却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "fails") {
		t.Errorf("错误信息未指明是哪个资源卸载失败，输出：\n%s", out)
	}
	if strings.Contains(out, "[SUCCESS]") {
		t.Errorf("失败路径上打印了 [SUCCESS]，输出：\n%s", out)
	}
	// 逆序意味着 fails 先跑，它失败之后不该继续拆前面的资源
	if _, err := os.Stat(filepath.Join(workdir, "reached.txt")); !os.IsNotExist(err) {
		t.Errorf("一个资源卸载失败后仍然继续拆了后续资源。输出：\n%s", out)
	}
}

// TestSC_E10_UninstallRequiresFile uninstall 不给 -f 必须报错而不是当无事发生。
func TestSC_E10_UninstallRequiresFile(t *testing.T) {
	code, out := run(t, "uninstall")
	if code == 0 {
		t.Fatalf("uninstall 缺 -f 却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "file") {
		t.Errorf("错误信息未指出缺的是 --file，输出：\n%s", out)
	}
}
