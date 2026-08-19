//go:build multinode

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

// Package multinode 在三个真正独立的节点上验证编排：谁该装、谁不该装。
//
// 只有文件系统真正分离，"文件到了目标节点、且没到操作机"才断言得出来 —— 这是 D1
// （SetNode 断链导致远程安装悄悄落在操作机上）唯一彻底的回归方式。
//
// 用 build tag multinode 隔离，默认 go test ./... 不跑。需要 docker 与 Linux 网桥直连
// （见 test/fixtures/multinode/compose.yaml）。载体是 .github/workflows/integration.yml。
//
// 与 test/local 一样是黑盒：不 import github.com/structure-projects/somcli/...。
package multinode

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	repoRoot    = "../.."
	fixtureDir  = "../fixtures/multinode"
	sshUser     = "root"
	sshWaitStep = 2 * time.Second
	sshWaitMax  = 90 * time.Second
)

// nodes 是 fixture 里的三个节点，IP 与 compose.yaml 一致。
var nodes = []node{
	{host: "node-a", ip: "172.28.0.11"},
	{host: "node-b", ip: "172.28.0.12"},
	{host: "node-c", ip: "172.28.0.13"},
}

type node struct {
	host string
	ip   string
}

var (
	binPath string
	keyPath string
)

// TestMain 起停 fixture。
//
// 前置不满足时的处置和 remote 组一致：本机缺 docker 属于环境不具备，直接跳过整包；
// CI 里则是准备步骤坏了，必须非 0 退出 —— 不让"跳过"冒充"通过"。
func TestMain(m *testing.M) {
	if err := requireDocker(); err != nil {
		if os.Getenv("CI") != "" {
			fmt.Fprintf(os.Stderr, "CI 里 docker 必须可用: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "跳过 multinode 组: %v\n", err)
		os.Exit(0)
	}

	if err := setup(); err != nil {
		teardown()
		fmt.Fprintf(os.Stderr, "准备 multinode fixture 失败: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	// SOMCLI_MULTINODE_KEEP=1 时保留容器，便于失败后进去看现场。
	if os.Getenv("SOMCLI_MULTINODE_KEEP") == "" {
		teardown()
	}
	os.Exit(code)
}

func requireDocker() error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("找不到 docker")
	}
	if out, err := exec.Command("docker", "info", "--format", "{{.ServerVersion}}").CombinedOutput(); err != nil {
		return fmt.Errorf("docker 守护进程不可用: %v\n%s", err, out)
	}
	return nil
}

func setup() error {
	if err := buildBinary(); err != nil {
		return err
	}
	if err := generateKey(); err != nil {
		return err
	}
	if out, err := compose("up", "-d", "--build"); err != nil {
		return fmt.Errorf("起容器失败: %v\n%s", err, out)
	}
	return waitForSSH()
}

func teardown() {
	if out, err := compose("down", "-v", "--remove-orphans"); err != nil {
		fmt.Fprintf(os.Stderr, "清理容器失败: %v\n%s\n", err, out)
	}
}

func buildBinary() error {
	dir, err := os.MkdirTemp("", "somcli-multinode")
	if err != nil {
		return err
	}
	binPath = filepath.Join(dir, "somcli")

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("编译 somcli 失败: %v\n%s", err, out)
	}
	return nil
}

// generateKey 生成一次性密钥。公钥落在 fixture 的 .ssh/ 下由 compose 挂进容器，
// 私钥用绝对路径写进配置 —— 用例要验的是编排，不是 ~ 展开（那条由 test/local 与 remote 覆盖）。
func generateKey() error {
	dir, err := filepath.Abs(filepath.Join(fixtureDir, ".ssh"))
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	keyPath = filepath.Join(dir, "id_rsa")
	out, err := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-f", keyPath, "-C", "somcli-multinode").CombinedOutput()
	if err != nil {
		return fmt.Errorf("生成密钥失败: %v\n%s", err, out)
	}
	return nil
}

func compose(args ...string) (string, error) {
	full := append([]string{"compose", "-f", "compose.yaml"}, args...)
	cmd := exec.Command("docker", full...)
	cmd.Dir = fixtureDir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// waitForSSH 等三个节点都能连上。容器起来到 sshd 接受连接之间有空档，
// 不等就会得到一串与编排无关的连接失败。
func waitForSSH() error {
	deadline := time.Now().Add(sshWaitMax)
	for _, n := range nodes {
		for {
			if _, err := ssh(n, "true"); err == nil {
				break
			}
			if time.Now().After(deadline) {
				logs, _ := compose("logs", "--no-color")
				return fmt.Errorf("等 %s (%s) 的 sshd 超时\n容器日志：\n%s", n.host, n.ip, logs)
			}
			time.Sleep(sshWaitStep)
		}
	}
	return nil
}

// nodesYAML 生成配置里的 nodes: 段，三个节点全声明。
func nodesYAML() string {
	var b strings.Builder
	b.WriteString("nodes:\n")
	for _, n := range nodes {
		fmt.Fprintf(&b, "  - host: %q\n    ip: %q\n    user: %q\n    role: \"worker\"\n    sshKey: %q\n",
			n.host, n.ip, sshUser, keyPath)
	}
	return b.String()
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "install.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写配置失败: %v", err)
	}
	return path
}

// runIn 在指定 workdir 下执行一次 somcli，返回退出码与合并输出。
func runIn(t *testing.T, workdir string, args ...string) (int, string) {
	t.Helper()

	full := append([]string{"--workdir", workdir}, args...)
	cmd := exec.Command(binPath, full...)
	out, err := cmd.CombinedOutput()

	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("执行 %v 失败: %v\n%s", args, err, out)
	}
	return code, string(out)
}

// ssh 在节点上跑一条命令，只返回**标准输出**，stderr 折进 error。
//
// 两条流必须分开：ssh 自己会往 stderr 写 "Warning: Permanently added ... to the list of
// known hosts."，而 UserKnownHostsFile=/dev/null 让这条警告每次连接都出现。合并输出的话
// 它会被当成文件内容，"节点上的产物等于该节点主机名"这类断言全部落空。
func ssh(n node, cmd string) (string, error) {
	c := exec.Command("ssh",
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("%s@%s", sshUser, n.ip),
		cmd)

	out, err := c.Output()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(out), fmt.Errorf("%v: %s", err, exitErr.Stderr)
	}
	return string(out), err
}

// nodeFile 读回某个节点上的文件内容；不存在时返回 ""，因为负向断言也要用它。
func nodeFile(t *testing.T, n node, path string) string {
	t.Helper()

	out, err := ssh(n, fmt.Sprintf("cat %s 2>/dev/null || true", shellQuote(path)))
	if err != nil {
		t.Fatalf("在 %s 上读 %s 失败: %v\n%s", n.host, path, err, out)
	}
	return strings.TrimSpace(out)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// localFileExists 操作机上是否有这个文件。"不该出现在操作机上"是 D1 的核心判据。
func localFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
