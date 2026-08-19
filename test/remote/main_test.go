//go:build remote

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

// Package remote 验证真正走 SSH 的那条路：scp 分发 + 远程执行。
//
// 用 build tag 隔开，默认 `go test ./...` 不跑 —— 它需要一个能连的 SSH 目标，
// 而开发机通常没开 sshd。载体是 CI（见 .github/workflows/integration.yml）。
//
// 与 test/local 一样是黑盒：不 import github.com/structure-projects/somcli/...。
package remote

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
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

// target 是被当作"远程节点"的 SSH 目标。
type target struct {
	host string // 写进配置 nodes[].host 的名字
	ip   string // 写进配置 nodes[].ip
	user string
	key  string // 配置里原样写的路径，故意保留 ~ 形式
}

// sshTarget 解析 SSH 目标并探测可达性。
//
// 默认指向 127.0.0.2：somcli 把 host/ip 为 localhost 或 127.0.0.1 的节点当本机直接
// sh -c 执行，用回环别名才能逼它真的走 ssh。（Linux 上 127.0.0.0/8 整段都在 lo 上，
// sshd 监听 0.0.0.0 就能连；macOS 需要先 ifconfig lo0 alias。）
//
// 探测失败时的处置分两种：本机缺 sshd 属于环境不具备，skip；CI 里则是配置步骤坏了，
// 必须硬失败 —— 否则"跳过"会冒充"通过"，整个 remote 组变成恒绿的空壳。
func sshTarget(t *testing.T) target {
	t.Helper()

	tg := target{
		host: "remote-a",
		ip:   envOr("SOMCLI_TEST_SSH_IP", "127.0.0.2"),
		user: envOr("SOMCLI_TEST_SSH_USER", currentUser(t)),
		key:  envOr("SOMCLI_TEST_SSH_KEY", "~/.ssh/id_rsa"),
	}

	probe := exec.Command("ssh",
		"-i", expand(tg.key),
		"-o", "StrictHostKeyChecking=no",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("%s@%s", tg.user, tg.ip),
		"true")
	if out, err := probe.CombinedOutput(); err != nil {
		reason := fmt.Sprintf("连不上 SSH 目标 %s@%s（key %s）: %v\n%s",
			tg.user, tg.ip, tg.key, err, out)
		if os.Getenv("CI") != "" {
			t.Fatalf("%s\nCI 里这不是环境不具备，而是准备步骤坏了", reason)
		}
		t.Skipf("%s\n本机跑 remote 组需要：开启 sshd、ssh-keygen 后把公钥加进 authorized_keys，"+
			"macOS 还需 sudo ifconfig lo0 alias 127.0.0.2 up", reason)
	}
	return tg
}

// somcliBinary 编译一份真实的二进制，整个包只编一次。
func somcliBinary(t *testing.T) string {
	t.Helper()

	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "somcli-remote")
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
// HOME 保持真实值：ssh 要读 ~/.ssh 下的私钥与 known_hosts。
func runIn(t *testing.T, workdir string, args ...string) (int, string) {
	t.Helper()

	bin := somcliBinary(t)
	full := append([]string{"--workdir", workdir}, args...)

	cmd := exec.Command(bin, full...)
	out, err := cmd.CombinedOutput()

	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("执行 %v 失败: %v\n%s", args, err, out)
	}
	return code, string(out)
}

// writeConfig 落一份临时配置，返回路径。
func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "install.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写配置失败: %v", err)
	}
	return path
}

// remoteRead 通过 ssh 读回远端文件内容，读不到即失败。
// 断言必须站在远端看：本机看到文件不代表分发成功，也可能是根本没走 ssh。
func remoteRead(t *testing.T, tg target, path string) string {
	t.Helper()

	out, err := ssh(tg, "cat "+shellQuote(path))
	if err != nil {
		t.Fatalf("远端读取 %s 失败: %v\n%s", path, err, out)
	}
	return out
}

// remoteExists 远端文件是否存在。用于负向断言，因此不能像 remoteRead 那样直接失败。
func remoteExists(t *testing.T, tg target, path string) bool {
	t.Helper()

	out, err := ssh(tg, "test -e "+shellQuote(path)+" && echo yes || echo no")
	if err != nil {
		t.Fatalf("远端探测 %s 失败: %v\n%s", path, err, out)
	}
	return strings.TrimSpace(out) == "yes"
}

// remoteCleanup 用例结束时清掉远端残留，避免下一次运行读到上一次的产物。
func remoteCleanup(t *testing.T, tg target, paths ...string) {
	t.Helper()

	t.Cleanup(func() {
		for _, p := range paths {
			if _, err := ssh(tg, "rm -rf "+shellQuote(p)); err != nil {
				t.Logf("清理远端 %s 失败（不影响判定）: %v", p, err)
			}
		}
	})
}

// ssh 在远端跑一条命令，只返回**标准输出**，stderr 折进 error。
//
// 两条流必须分开：ssh 往 stderr 写的 "Warning: Permanently added ..." 会被当成文件内容。
// 这里目前碰不到那条警告（不禁 known_hosts，首连之后就不再提示），但依赖"警告恰好不出现"
// 等于把断言的正确性交给运行顺序。
func ssh(tg target, cmd string) (string, error) {
	c := exec.Command("ssh",
		"-i", expand(tg.key),
		"-o", "StrictHostKeyChecking=no",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("%s@%s", tg.user, tg.ip),
		cmd)

	out, err := c.Output()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(out), fmt.Errorf("%v: %s", err, exitErr.Stderr)
	}
	return string(out), err
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func expand(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// fileExistsLocally 用于负向断言："这个文件不该出现在操作机上"。
func fileExistsLocally(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func currentUser(t *testing.T) string {
	t.Helper()

	u, err := user.Current()
	if err != nil {
		t.Fatalf("取当前用户失败: %v", err)
	}
	return u.Username
}
