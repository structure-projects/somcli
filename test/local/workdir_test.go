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

// TestSC_X01_WorkdirIsolatesAllDerivedPaths --workdir 一改，全部派生目录随之改变。
//
// workdir 是所有产物的根：download / scripts / data / apps / logs 都从它派生。
// 两次不同 --workdir 的运行必须互不可见，否则并行装两套环境会互相踩，
// 清理一套还会连带毁掉另一套。
func TestSC_X01_WorkdirIsolatesAllDerivedPaths(t *testing.T) {
	cfg := writeConfig(t, `
resources:
  - name: derived
    version: "1.0"
    post_install:
      - "mkdir -p {{.DownloadDir}} {{.ScriptDir}} {{.DataDir}}"
      - "echo marker > {{.WorkDir}}/marker.txt"
      - "echo {{.WorkDir}} > {{.ScriptDir}}/which-workdir.txt"
`)

	first, second := t.TempDir(), t.TempDir()

	for _, workdir := range []string{first, second} {
		code, out := runIn(t, workdir, "install", "-f", cfg)
		if code != 0 {
			t.Fatalf("workdir=%s 安装退出码 = %d，输出：\n%s", workdir, code, out)
		}
		if got := strings.TrimSpace(readFile(t, filepath.Join(workdir, "marker.txt"))); got != "marker" {
			t.Fatalf("workdir=%s 下没有产物，产物内容 = %q", workdir, got)
		}
		// 派生目录里的内容必须指回本次的 workdir，而不是上一次的。
		if got := strings.TrimSpace(readFile(t, filepath.Join(workdir, "scripts", "which-workdir.txt"))); got != workdir {
			t.Errorf("ScriptDir 未跟随 --workdir：文件记录的 workdir = %q，实际传入 %q", got, workdir)
		}
	}

	// 两个 workdir 的产物结构应当一致且各自独立：第二次运行不得往第一个目录里补东西。
	firstTree, secondTree := tree(t, first), tree(t, second)
	if strings.Join(firstTree, "\n") != strings.Join(secondTree, "\n") {
		t.Errorf("两次运行的产物结构不一致：\n%v\n%v", firstTree, secondTree)
	}
}

// TestSC_X01_NoSomworkLeaksIntoRepo 不传 --workdir 时产物默认落到 ./somwork，
// 所以用例一律显式传。这条守的是用例自己不许污染仓库。
//
// 判据是本次运行前后仓库侧无变化，而不是 somwork 不存在：
// 开发机上可能早就有一个手工跑出来的 somwork，那不是本次运行的锅。
func TestSC_X01_NoSomworkLeaksIntoRepo(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: derived
    version: "1.0"
    post_install:
      - "echo marker > {{.WorkDir}}/marker.txt"
`)

	watched := []string{filepath.Join(repoRoot, "somwork"), "somwork"}
	before := make([]string, len(watched))
	for i, dir := range watched {
		before[i] = snapshot(t, dir)
	}

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("安装退出码 = %d，输出：\n%s", code, out)
	}

	for i, dir := range watched {
		if after := snapshot(t, dir); after != before[i] {
			t.Errorf("传了 --workdir 却动了仓库侧的 %s：\n之前 %s\n之后 %s", dir, before[i], after)
		}
	}
}

// snapshot 把一个可能不存在的目录压成一个可比较的字符串。
func snapshot(t *testing.T, dir string) string {
	t.Helper()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "<不存在>"
	}
	return strings.Join(tree(t, dir), "\n")
}