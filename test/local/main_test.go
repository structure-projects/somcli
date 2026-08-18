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

// Package local 是 somcli 的黑盒功能验证：编译真二进制 → 喂真实配置 → 跑真实命令 →
// 断言退出码、输出、落盘产物。
//
// 这里不 import github.com/structure-projects/somcli/...（由 ci.yml 的 static job 守卫）。
// 理由：断言内部函数的返回值无法回答"这个工具能不能用"，且会随重构变红，反向锁死结构。
// 用例名带 test/matrix.yaml 的场景 ID，便于从验收清单定位到用例。
package local

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

const repoRoot = "../.."

var (
	buildOnce sync.Once
	binPath   string
	buildErr  error
	buildOut  string
)

// somcliBinary 编译一份真实的二进制，整个包只编一次。
//
// 必须跑二进制而不是调函数：退出码由 cmd 层的 os.Exit 决定，是 CI 与外层脚本唯一能
// 感知的信号；同样，"帮助有没有副作用""产物落在哪个目录"也只有真进程才观察得到。
func somcliBinary(t *testing.T) string {
	t.Helper()

	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "somcli-blackbox")
		if err != nil {
			buildErr = err
			return
		}
		out := filepath.Join(dir, "somcli")
		cmd := exec.Command("go", "build", "-o", out, ".")
		cmd.Dir = repoRoot
		combined, err := cmd.CombinedOutput()
		buildOut = string(combined)
		if err != nil {
			buildErr = err
			return
		}
		binPath = out
	})

	if buildErr != nil {
		t.Fatalf("编译 somcli 失败: %v\n%s", buildErr, buildOut)
	}
	return binPath
}

// runIn 在指定 workdir 下执行一次 somcli，返回退出码与合并输出。
//
// HOME 也指向临时目录：initConfig 会去 $HOME 找 .somcli.yaml，用开发机的真实 HOME
// 会让用例受本机配置影响，且 {{.HostDir}} 渲染出的路径会指向真实家目录。
func runIn(t *testing.T, workdir string, args ...string) (int, string) {
	t.Helper()

	bin := somcliBinary(t)
	full := append([]string{"--workdir", workdir}, args...)

	cmd := exec.Command(bin, full...)
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()

	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("执行 %v 失败: %v\n%s", args, err, out)
	}
	return code, string(out)
}

// run 用一个一次性的 workdir 执行 somcli。
// 不传 --workdir 的话产物会落到 ./somwork —— 即仓库目录，用例不得污染它。
func run(t *testing.T, args ...string) (int, string) {
	t.Helper()
	return runIn(t, t.TempDir(), args...)
}

// writeConfig 落一份临时配置，返回路径。
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	return writeConfigIn(t, t.TempDir(), content)
}

// writeConfigIn 把配置写到指定目录，用于需要配置与 workdir 分离的用例。
func writeConfigIn(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "install.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写配置失败: %v", err)
	}
	return path
}

// readFile 读取用例产物，文件不存在即失败并说明缺的是什么。
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取产物 %s 失败: %v", path, err)
	}
	return string(data)
}

// lines 把产物文件按行切开并去掉空行，用于断言脚本的执行顺序。
func lines(t *testing.T, path string) []string {
	t.Helper()
	var out []string
	for _, line := range strings.Split(readFile(t, path), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

// tree 给目录做一次文件树快照（相对路径，已排序）。
// 用于"某个操作前后不得有任何落盘变化"这类负向断言。
func tree(t *testing.T, dir string) []string {
	t.Helper()

	var paths []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			rel += "/"
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("快照目录 %s 失败: %v", dir, err)
	}
	sort.Strings(paths)
	return paths
}
