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

// E6：registry uninstall 得有自己的标志。
//
// 原实现一个标志都没声明，读的是 install 的包级变量 —— 于是"卸载哪一个"完全没法表达，
// `--hostname` 直接是 unknown flag。
func TestE6_RegistryUninstallOwnsItsFlags(t *testing.T) {
	code, out := run(t, "registry", "uninstall", "--help")
	if code != 0 {
		t.Fatalf("查看帮助退出码 = %d，输出：\n%s", code, out)
	}
	for _, want := range []string{"--hostname", "--version"} {
		if !strings.Contains(out, want) {
			t.Fatalf("uninstall 的帮助里没有 %s：\n%s", want, out)
		}
	}
}

// E6：这两个标志是可选的 —— 卸载靠安装目录，不该逼用户报出当初装的是哪个版本。
func TestE6_RegistryUninstallFlagsAreOptional(t *testing.T) {
	_, out := run(t, "registry", "uninstall")
	if strings.Contains(out, "required flag") {
		t.Fatalf("卸载被要求必填标志：\n%s", out)
	}
}

// E6：给了值就要校验格式，不能收下一个明显不对的主机名当没看见。
//
// 两处都得排掉 "unknown flag"：不排的话，一个压根不认识这些标志的实现也会非 0 退出、
// 而且报错里恰好带着 hostname 字样，用例就跟着绿了。
func TestE6_RegistryUninstallValidatesGivenValues(t *testing.T) {
	code, out := run(t, "registry", "uninstall", "--hostname", "not-a-host")
	if code == 0 {
		t.Fatalf("主机名格式不对却退出 0，输出：\n%s", out)
	}
	if strings.Contains(out, "unknown flag") {
		t.Fatalf("--hostname 压根没被识别：\n%s", out)
	}
	if !strings.Contains(out, "hostname") {
		t.Fatalf("错误信息没有点出问题在主机名，输出：\n%s", out)
	}

	code, out = run(t, "registry", "uninstall", "--version", "2.9.0")
	if code == 0 {
		t.Fatalf("版本没有 v 前缀却退出 0，输出：\n%s", out)
	}
	if strings.Contains(out, "unknown flag") {
		t.Fatalf("--version 压根没被识别：\n%s", out)
	}
	if !strings.Contains(out, "version") {
		t.Fatalf("错误信息没有点出问题在版本号，输出：\n%s", out)
	}
}

// E7：delete 得认 -n/--namespace。
//
// 原实现没注册这个标志，于是 `somcli delete pod web -n prod` 死在
// "unknown shorthand flag: 'n'" —— 命名空间是 k8s 里删东西的必要坐标。
func TestE7_DeleteAcceptsNamespaceFlag(t *testing.T) {
	code, out := run(t, "delete", "--help")
	if code != 0 {
		t.Fatalf("查看帮助退出码 = %d，输出：\n%s", code, out)
	}
	if !strings.Contains(out, "--namespace") {
		t.Fatalf("delete 的帮助里没有 --namespace：\n%s", out)
	}

	// 本机没有集群，delete 必然失败；判据是"失败在找不到集群"而不是"失败在标志不认识"，
	// 这正好把标志解析与后续动作分开来看
	_, out = run(t, "delete", "pod", "web", "-n", "prod")
	if strings.Contains(out, "unknown shorthand flag") || strings.Contains(out, "unknown flag") {
		t.Fatalf("-n 没有被识别：\n%s", out)
	}
}
