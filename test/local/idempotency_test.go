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
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestSC_E08_SecondRunProducesNoDuplicateSideEffects 连续两次 install 不重复产生副作用。
//
// E4：引擎过去完全无状态，每次 install 都把所有脚本从头再跑一遍。
// 判据用"追加写"而不是"覆盖写"：覆盖写的脚本跑两遍与跑一遍产物相同，
// 用例会在一个不幂等的实现上照样绿。追加写把执行次数直接暴露成行数。
func TestSC_E08_SecondRunProducesNoDuplicateSideEffects(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: appends
    version: "1.0"
    post_install:
      - "echo ran >> {{.WorkDir}}/runs.txt"
`)

	if code, out := runIn(t, workdir, "install", "-f", cfg); code != 0 {
		t.Fatalf("首次安装退出码 = %d，输出：\n%s", code, out)
	}
	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("重复安装退出码 = %d，输出：\n%s", code, out)
	}

	if got := lines(t, filepath.Join(workdir, "runs.txt")); len(got) != 1 {
		t.Errorf("脚本执行了 %d 次（%v），幂等要求只执行一次。第二次输出：\n%s", len(got), got, out)
	}
	if !strings.Contains(out, "跳过") {
		t.Errorf("第二次安装没有说明为什么什么都没做，输出：\n%s", out)
	}
}

// TestSC_E08_CheckProbeHitSkipsWholeResource check 退出 0 即跳过整个资源。
//
// 语义定为"退出 0 即跳过"，与 command -v foo / test -f /usr/local/bin/foo
// 这类惯用探针一致 —— 探针命中意味着东西已经在了。
// 跳过的是**整个资源**，pre_install 也不跑：check 的用途正是"这台机器不用动"，
// 只跳过 method 而照跑 pre/post 的话，那些脚本里的 mkdir / systemctl 仍会执行。
func TestSC_E08_CheckProbeHitSkipsWholeResource(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: guarded
    version: "1.0"
    check: "test -f {{.WorkDir}}/already-there"
    pre_install:
      - "echo pre >> {{.WorkDir}}/ran.txt"
    post_install:
      - "echo post >> {{.WorkDir}}/ran.txt"
`)

	if err := os.WriteFile(filepath.Join(workdir, "already-there"), []byte("x"), 0o644); err != nil {
		t.Fatalf("准备探针命中的标记文件失败: %v", err)
	}

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("探针命中时退出码 = %d，期望 0（跳过不是失败）。输出：\n%s", code, out)
	}
	if _, err := os.Stat(filepath.Join(workdir, "ran.txt")); !os.IsNotExist(err) {
		t.Errorf("check 命中却仍然执行了脚本，产生了 ran.txt。输出：\n%s", out)
	}
	if !strings.Contains(out, "check") {
		t.Errorf("输出没有说明是 check 让它跳过的，用户无从判断，输出：\n%s", out)
	}
}

// TestSC_E08_CheckProbeMissRunsResource check 非 0 退出就照常安装。
//
// 探针失败不是错误：它就是"还没装"。当成错误处理的话，第一次安装（探针必然失败）
// 会直接退出 —— 一个用来省事的字段反而让什么都装不上。
func TestSC_E08_CheckProbeMissRunsResource(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: guarded
    version: "1.0"
    check: "test -f {{.WorkDir}}/never-created"
    post_install:
      - "echo installed > {{.WorkDir}}/ran.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("探针未命中时退出码 = %d，输出：\n%s", code, out)
	}
	if got := strings.TrimSpace(readFile(t, filepath.Join(workdir, "ran.txt"))); got != "installed" {
		t.Errorf("探针未命中却没有安装，产物 = %q。输出：\n%s", got, out)
	}
}

// TestSC_E09_VersionChangeReapplies version 变更后重新实施。
//
// 幂等的判据是 name@version 而不是 name：只按名字记的话升级永远装不上，
// 用户改了 version 却看到"已安装，跳过"。
func TestSC_E09_VersionChangeReapplies(t *testing.T) {
	workdir := t.TempDir()
	cfgDir := t.TempDir()
	tmpl := `
resources:
  - name: upgrades
    version: "%s"
    post_install:
      - "echo {{.Version}} >> {{.WorkDir}}/versions.txt"
`

	cfg := writeConfigIn(t, cfgDir, fmt.Sprintf(tmpl, "1.0"))
	if code, out := runIn(t, workdir, "install", "-f", cfg); code != 0 {
		t.Fatalf("安装 1.0 退出码 = %d，输出：\n%s", code, out)
	}

	cfg = writeConfigIn(t, cfgDir, fmt.Sprintf(tmpl, "2.0"))
	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("安装 2.0 退出码 = %d，输出：\n%s", code, out)
	}

	want := []string{"1.0", "2.0"}
	if got := lines(t, filepath.Join(workdir, "versions.txt")); !reflect.DeepEqual(got, want) {
		t.Errorf("实施记录 = %v，期望 %v（版本变更必须重新实施）。输出：\n%s", got, want, out)
	}
	if !strings.Contains(out, "1.0") {
		t.Errorf("输出没有提到原先记录的版本，升级过程不可见，输出：\n%s", out)
	}
}

// TestSC_E08_ForceReappliesSameVersion --force 在不改 version 的前提下重装。
//
// 状态一旦参与决策就必须留出口：目标机上被人手工改坏了、或者只想重跑一遍，
// 没有 --force 的话用户只能去删状态文件 —— 那等于让用户维护 somcli 的内部账本。
func TestSC_E08_ForceReappliesSameVersion(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: appends
    version: "1.0"
    post_install:
      - "echo ran >> {{.WorkDir}}/runs.txt"
`)

	if code, out := runIn(t, workdir, "install", "-f", cfg); code != 0 {
		t.Fatalf("首次安装退出码 = %d，输出：\n%s", code, out)
	}
	code, out := runIn(t, workdir, "install", "-f", cfg, "--force")
	if code != 0 {
		t.Fatalf("--force 安装退出码 = %d，输出：\n%s", code, out)
	}

	if got := lines(t, filepath.Join(workdir, "runs.txt")); len(got) != 2 {
		t.Errorf("--force 后脚本执行了 %d 次（%v），期望 2 次。输出：\n%s", len(got), got, out)
	}
}

// TestSC_E08_ForceIgnoresCheckProbe --force 同样越过 check 探针。
//
// 只越过状态记录而听 check 的话，--force 在写了 check 的资源上就是个空标志，
// 而恰恰是这类资源最需要强制重装（探针只看"文件在不在"，看不出内容对不对）。
func TestSC_E08_ForceIgnoresCheckProbe(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: guarded
    version: "1.0"
    check: "test -f {{.WorkDir}}/already-there"
    post_install:
      - "echo forced > {{.WorkDir}}/ran.txt"
`)
	if err := os.WriteFile(filepath.Join(workdir, "already-there"), []byte("x"), 0o644); err != nil {
		t.Fatalf("准备探针命中的标记文件失败: %v", err)
	}

	code, out := runIn(t, workdir, "install", "-f", cfg, "--force")
	if code != 0 {
		t.Fatalf("--force 安装退出码 = %d，输出：\n%s", code, out)
	}
	if got := strings.TrimSpace(readFile(t, filepath.Join(workdir, "ran.txt"))); got != "forced" {
		t.Errorf("--force 没有越过 check，产物 = %q。输出：\n%s", got, out)
	}
}

// TestSC_F08_RerunAfterMidwayFailureContinues 中途失败后重跑能接着往下走。
//
// 这是状态文件真正要解决的场面：三个资源，第二个失败。
// 修好问题重跑时，两件事都得成立 ——
//  1. 第一个资源不能再装一遍（否则状态白记，重跑照样有重复副作用）；
//  2. 第二、第三个资源必须继续（否则状态成了拦路虎，卡在半装状态出不来）。
//
// 失败的资源绝不能记进状态，否则重跑会跳过它，用户看到"全部已安装"而它从没装成。
func TestSC_F08_RerunAfterMidwayFailureContinues(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: first
    version: "1.0"
    post_install:
      - "echo first >> {{.WorkDir}}/order.txt"
  - name: blocked
    version: "1.0"
    post_install:
      - "test -f {{.WorkDir}}/unblock"
      - "echo blocked >> {{.WorkDir}}/order.txt"
  - name: third
    version: "1.0"
    post_install:
      - "echo third >> {{.WorkDir}}/order.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("中途失败却退出码 0，输出：\n%s", out)
	}
	if got := lines(t, filepath.Join(workdir, "order.txt")); !reflect.DeepEqual(got, []string{"first"}) {
		t.Fatalf("首轮执行记录 = %v，期望只有 first（默认 abort 应停在 blocked）。输出：\n%s", got, out)
	}

	// 把阻塞条件解除，模拟"人把问题修好了"
	if err := os.WriteFile(filepath.Join(workdir, "unblock"), []byte("x"), 0o644); err != nil {
		t.Fatalf("解除阻塞失败: %v", err)
	}

	code, out = runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("重跑退出码 = %d，输出：\n%s", code, out)
	}

	want := []string{"first", "blocked", "third"}
	if got := lines(t, filepath.Join(workdir, "order.txt")); !reflect.DeepEqual(got, want) {
		t.Errorf("重跑后执行记录 = %v，期望 %v（first 不重跑、blocked 与 third 继续）。输出：\n%s",
			got, want, out)
	}
}
