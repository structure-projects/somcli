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

// TestSC_E16_StatusListsInstalledResources somcli status 报出装了什么、什么版本、在哪。
//
// E4：引擎过去无状态，"这台机器上 somcli 装过什么"只能靠人记。
// 状态既然要参与幂等决策，就必须能被人读到 —— 否则跳过某个资源的理由不可查，
// 用户唯一的排查手段是删掉整个工作目录。
func TestSC_E16_StatusListsInstalledResources(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: alpha
    version: "1.2.3"
    post_install:
      - "true"
  - name: beta
    version: "4.5.6"
    method: script
    post_install:
      - "true"
`)

	if code, out := runIn(t, workdir, "install", "-f", cfg); code != 0 {
		t.Fatalf("安装退出码 = %d，输出：\n%s", code, out)
	}

	code, out := runIn(t, workdir, "status")
	if code != 0 {
		t.Fatalf("status 退出码 = %d，输出：\n%s", code, out)
	}
	for _, want := range []string{"alpha", "1.2.3", "beta", "4.5.6"} {
		if !strings.Contains(out, want) {
			t.Errorf("status 输出缺少 %q，输出：\n%s", want, out)
		}
	}
}

// TestSC_E16_StatusRecordsPerHost 声明了 hosts 的资源按目标逐条记账。
//
// "哪个节点装了什么版本"是这个场景的原话：只记资源名与版本的话，
// 多节点下一台装成、一台失败会被记成"装好了"，而下次 install 会把失败那台一起跳过。
// 本机字面量 localhost 让这条判据在 local 组就能验，不必等 multinode。
func TestSC_E16_StatusRecordsPerHost(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
nodes:
  - host: localhost
    ip: 127.0.0.1
resources:
  - name: per-host
    version: "1.0"
    hosts: [localhost]
    post_install:
      - "true"
`)

	if code, out := runIn(t, workdir, "install", "-f", cfg); code != 0 {
		t.Fatalf("安装退出码 = %d，输出：\n%s", code, out)
	}

	code, out := runIn(t, workdir, "status")
	if code != 0 {
		t.Fatalf("status 退出码 = %d，输出：\n%s", code, out)
	}
	if !strings.Contains(out, "localhost") {
		t.Errorf("status 没有记下实施目标，输出：\n%s", out)
	}
}

// TestSC_E16_StatusWithoutRecordsSaysSo 没有任何记录时要明说，而不是打印一张空表。
func TestSC_E16_StatusWithoutRecordsSaysSo(t *testing.T) {
	code, out := run(t, "status")
	if code != 0 {
		t.Fatalf("status 退出码 = %d，输出：\n%s", code, out)
	}
	if !strings.Contains(out, "没有任何安装记录") {
		t.Errorf("空状态下的输出无法与「装了但没显示」区分，输出：\n%s", out)
	}
}

// TestSC_E16_StateFileLivesInWorkdir 状态文件落在 workdir 下，跟着 --workdir 走。
//
// 位置不能是家目录或写死的路径：同一台操作机上管理多套环境是常态，
// 两套环境共用一份状态会互相跳过对方的资源。
func TestSC_E16_StateFileLivesInWorkdir(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: recorded
    version: "1.0"
    post_install:
      - "true"
`)

	if code, out := runIn(t, workdir, "install", "-f", cfg); code != 0 {
		t.Fatalf("安装退出码 = %d，输出：\n%s", code, out)
	}

	statePath := filepath.Join(workdir, "state.json")
	got := readFile(t, statePath)
	if !strings.Contains(got, "recorded") {
		t.Errorf("状态文件 %s 里没有这次安装的记录，内容：\n%s", statePath, got)
	}
}

// TestSC_E16_CorruptStateFileDegradesToStateless 状态文件损坏时降级执行，不报错退出。
//
// 提案把这条列为强制用例：状态是新增产物，它自己绝不能成为"装不上"的新原因。
// 判据有两条 —— 装得上（退出 0 且产物落地）、并且出声（否则用户不知道账本被丢了）。
func TestSC_E16_CorruptStateFileDegradesToStateless(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: after-corruption
    version: "1.0"
    post_install:
      - "echo installed > {{.WorkDir}}/ran.txt"
`)

	if err := os.WriteFile(filepath.Join(workdir, "state.json"), []byte("{not json at all"), 0o644); err != nil {
		t.Fatalf("写坏状态文件失败: %v", err)
	}

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("状态文件损坏导致安装失败，退出码 = %d，输出：\n%s", code, out)
	}
	if got := strings.TrimSpace(readFile(t, filepath.Join(workdir, "ran.txt"))); got != "installed" {
		t.Errorf("状态损坏后没有照常安装，产物 = %q。输出：\n%s", got, out)
	}
	if !strings.Contains(out, "[WARNING]") {
		t.Errorf("丢掉了整份状态记录却没有任何警告，输出：\n%s", out)
	}

	// 降级之后要把状态修回来：留着坏文件的话每次运行都在降级，幂等永久失效
	if code, out := runIn(t, workdir, "status"); code != 0 || !strings.Contains(out, "after-corruption") {
		t.Errorf("损坏的状态文件没有被这次安装重建，status 退出码 = %d，输出：\n%s", code, out)
	}
}

// TestSC_E16_UninstallDropsStateRecord 卸载后状态记录要消失。
//
// 不删的话重装会撞上幂等判据，打印"已记录为已安装，跳过"，而机器上什么都没有 ——
// 这是状态参与决策带来的新失败模式，必须钉住。
func TestSC_E16_UninstallDropsStateRecord(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: round-trip
    version: "1.0"
    post_install:
      - "echo ran >> {{.WorkDir}}/runs.txt"
    remove_scripts:
      - "rm -f {{.WorkDir}}/runs.txt"
`)

	if code, out := runIn(t, workdir, "install", "-f", cfg); code != 0 {
		t.Fatalf("安装退出码 = %d，输出：\n%s", code, out)
	}
	if code, out := runIn(t, workdir, "uninstall", "-f", cfg); code != 0 {
		t.Fatalf("卸载退出码 = %d，输出：\n%s", code, out)
	}

	code, out := runIn(t, workdir, "status")
	if code != 0 {
		t.Fatalf("status 退出码 = %d，输出：\n%s", code, out)
	}
	if strings.Contains(out, "round-trip") {
		t.Fatalf("卸载后状态里仍有记录，输出：\n%s", out)
	}

	// 真正的后果在这里：卸载完必须还能装回来
	if code, out := runIn(t, workdir, "install", "-f", cfg); code != 0 {
		t.Fatalf("卸载后重装退出码 = %d，输出：\n%s", code, out)
	}
	if got := lines(t, filepath.Join(workdir, "runs.txt")); len(got) != 1 {
		t.Errorf("卸载后重装没有真正执行，runs.txt = %v", got)
	}
}
