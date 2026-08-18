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
	"os/exec"
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

// somcliBinary 编译一份真实的二进制。
//
// SC-X06 断言的是**进程退出码**，这是 CI 与外层脚本唯一能感知的信号，
// 只有跑真二进制才测得到——直接调函数看返回值绕过了 cmd 层的 os.Exit。
func somcliBinary(t *testing.T) string {
	t.Helper()

	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "somcli-exitcode")
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

// run 执行一次 somcli，返回退出码与合并输出。
// 每次都指定 --workdir，避免用例把 somwork 写进仓库。
func run(t *testing.T, args ...string) (int, string) {
	t.Helper()

	bin := somcliBinary(t)
	full := append([]string{"--workdir", t.TempDir()}, args...)

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

// writeConfig 落一份临时配置，返回路径。
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "install.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写配置失败: %v", err)
	}
	return path
}

// TestSC_X06_InstallFailureExitsNonZero 是整条信号链的终点断言。
//
// 曾经的行为：post_install 脚本非零退出 → RunScripts 吞掉错误 → Install 返回 nil →
// 打印 "✓ 成功安装" → 进程退出码 0。于是流水线里这一步永远是绿的，
// 故障要到下一个依赖它的步骤才暴露。这条用例同时钉住三件事：
// 退出码非 0、输出里不出现 [SUCCESS]、错误信息能定位到出错的资源。
func TestSC_X06_InstallFailureExitsNonZero(t *testing.T) {
	cfg := writeConfig(t, `
resources:
  - name: broken-tool
    version: "1.0"
    post_install:
      - exit 7
`)

	code, out := run(t, "install", "-f", cfg)

	if code == 0 {
		t.Errorf("post_install 失败但退出码为 0，CI 无法感知。输出：\n%s", out)
	}
	if strings.Contains(out, "[SUCCESS]") {
		t.Errorf("失败路径上打印了 [SUCCESS]，输出：\n%s", out)
	}
	if !strings.Contains(out, "broken-tool") {
		t.Errorf("输出未指明失败的资源名，输出：\n%s", out)
	}
}

// TestSC_X06_PreInstallFailureExitsNonZero 前置脚本失败同样要传到退出码。
// 两个阶段各有一条独立的返回路径，漏包一个就会留下静默成功的缺口。
func TestSC_X06_PreInstallFailureExitsNonZero(t *testing.T) {
	cfg := writeConfig(t, `
resources:
  - name: broken-tool
    version: "1.0"
    pre_install:
      - exit 3
    post_install:
      - "true"
`)

	code, out := run(t, "install", "-f", cfg)

	if code == 0 {
		t.Errorf("pre_install 失败但退出码为 0，输出：\n%s", out)
	}
	if strings.Contains(out, "[SUCCESS]") {
		t.Errorf("失败路径上打印了 [SUCCESS]，输出：\n%s", out)
	}
}

// TestSC_X06_UnknownHostExitsNonZero 与 D1 的交叉点：
// hosts 指向未声明的节点时必须以非 0 退出，绝不能退化成在操作机上执行后报成功。
func TestSC_X06_UnknownHostExitsNonZero(t *testing.T) {
	cfg := writeConfig(t, `
nodes:
  - host: node-a
    ip: 192.168.10.11
    user: root
resources:
  - name: needs-remote
    version: "1.0"
    hosts:
      - node-does-not-exist
    post_install:
      - "true"
`)

	code, out := run(t, "install", "-f", cfg)

	if code == 0 {
		t.Errorf("hosts 指向未声明节点却退出码 0，输出：\n%s", out)
	}
	if strings.Contains(out, "[SUCCESS]") {
		t.Errorf("失败路径上打印了 [SUCCESS]，输出：\n%s", out)
	}
}

// TestSC_X06_BadConfigExitsNonZero 配置本身有问题时也要非 0：
// 文件不存在、未知字段（严格解析）都属于"用户还没开始装就该被拦住"。
func TestSC_X06_BadConfigExitsNonZero(t *testing.T) {
	t.Run("SC-X06/配置文件不存在", func(t *testing.T) {
		code, out := run(t, "install", "-f", filepath.Join(t.TempDir(), "missing.yaml"))
		if code == 0 {
			t.Errorf("配置文件不存在却退出码 0，输出：\n%s", out)
		}
	})

	t.Run("SC-X06/未知字段", func(t *testing.T) {
		cfg := writeConfig(t, `
resources:
  - name: jq
    version: "1.7.1"
    postinstall:
      - "true"
`)
		code, out := run(t, "install", "-f", cfg)
		if code == 0 {
			t.Errorf("配置含未知字段 postinstall 却退出码 0（会静默不执行），输出：\n%s", out)
		}
	})

	t.Run("SC-X06/缺少必填标志", func(t *testing.T) {
		code, out := run(t, "install")
		if code == 0 {
			t.Errorf("未指定 -f 却退出码 0，输出：\n%s", out)
		}
	})
}

// TestSC_X06_SuccessPathExitsZero 反向对照。
// 少了它，上面几条用"永远返回非 0"也能通过——那种实现同样毫无信号价值。
func TestSC_X06_SuccessPathExitsZero(t *testing.T) {
	t.Run("SC-X06/version", func(t *testing.T) {
		code, out := run(t, "version")
		if code != 0 {
			t.Errorf("version 退出码 = %d，期望 0。输出：\n%s", code, out)
		}
	})

	t.Run("SC-X06/install 全部成功", func(t *testing.T) {
		cfg := writeConfig(t, `
resources:
  - name: ok-tool
    version: "1.0"
    pre_install:
      - "true"
    post_install:
      - "true"
`)
		code, out := run(t, "install", "-f", cfg)
		if code != 0 {
			t.Errorf("全部成功却退出码 = %d，输出：\n%s", code, out)
		}
		if !strings.Contains(out, "[SUCCESS]") {
			t.Errorf("成功路径上没有成功信号，输出：\n%s", out)
		}
	})
}
