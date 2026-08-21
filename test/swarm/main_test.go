//go:build swarm

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

// Package swarm 在真节点上验证 `somcli cluster create` 装出来的 Docker Swarm 是否真的能用。
//
// 与 test/cluster（k8s）分成两组，理由写在 test/fixtures/swarm-nodes/compose.yaml 里：
// 装 docker 会覆盖 k8s 用例装出来的 containerd 配置，共用容器的话失败会出现在
// 另一组用例上。
//
// build tag 是 swarm（与目录名、与 test/matrix.yaml 的 env: [swarm] 一致），
// 默认 `go test ./...` 不跑。需要 Linux + docker，节点是 privileged 的 systemd 容器。
//
// 与其他测试组一样是黑盒：不 import github.com/structure-projects/somcli/...。
package swarm

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
	repoRoot   = "../.."
	fixtureDir = "../fixtures/swarm-nodes"
	sshUser    = "root"

	bootWaitStep = 3 * time.Second
	bootWaitMax  = 180 * time.Second
)

// 节点地址与 compose.yaml 一致。
var (
	manager1 = node{host: "swarm-m1", ip: "172.30.0.11", role: "manager"}
	manager2 = node{host: "swarm-m2", ip: "172.30.0.12", role: "manager"}
	worker1  = node{host: "swarm-w1", ip: "172.30.0.13", role: "worker"}
)

var allNodes = []node{manager1, manager2, worker1}

type node struct {
	host string
	ip   string
	role string
}

var (
	binPath string
	keyPath string
	// 全包共用一个 workdir：每条用例都要装 docker，一条一个 workdir 的话
	// 每条都要重新下一遍。共用之后"装过的不再装"正是引擎的幂等能力。
	workdir string
)

func TestMain(m *testing.M) {
	if err := requireDocker(); err != nil {
		if os.Getenv("CI") != "" {
			fmt.Fprintf(os.Stderr, "CI 里 docker 必须可用: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "跳过 swarm 组: %v\n", err)
		os.Exit(0)
	}

	if err := setup(); err != nil {
		dumpNodeLogs()
		teardown()
		fmt.Fprintf(os.Stderr, "准备 swarm 节点失败: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	if os.Getenv("SOMCLI_SWARM_KEEP") == "" {
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
	dir, err := os.MkdirTemp("", "somcli-swarm-work")
	if err != nil {
		return err
	}
	workdir = dir
	if out, err := compose("up", "-d", "--build"); err != nil {
		return fmt.Errorf("起容器失败: %v\n%s", err, out)
	}
	return waitForNodes()
}

func teardown() {
	if out, err := compose("down", "-v", "--remove-orphans"); err != nil {
		fmt.Fprintf(os.Stderr, "清理容器失败: %v\n%s\n", err, out)
	}
}

func buildBinary() error {
	dir, err := os.MkdirTemp("", "somcli-swarm")
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
// 私钥用绝对路径写进配置 —— 这一组要验的是集群，不是 ~ 展开。
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
	out, err := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-f", keyPath, "-C", "somcli-swarm").CombinedOutput()
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

// waitForNodes 等节点都能 SSH 上、且 systemd 已经跑完启动。
// 光能 SSH 不够：systemd 还在起服务时 `systemctl start docker` 会以
// "Transaction is destructive" 之类的理由失败，那种失败与 somcli 无关。
func waitForNodes() error {
	deadline := time.Now().Add(bootWaitMax)
	for _, n := range allNodes {
		for {
			if systemdReady(n) {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("等 %s (%s) 就绪超时", n.host, n.ip)
			}
			time.Sleep(bootWaitStep)
		}
	}
	return nil
}

func systemdReady(n node) bool {
	out, err := ssh(n, "systemctl is-system-running || true")
	if err != nil {
		return false
	}
	state := strings.TrimSpace(out)
	return state == "running" || state == "degraded"
}

// swarmConfig 写一份 swarm 集群配置，返回路径。
//
// advertiseAddr 传空串表示配置里不写这个键 —— 那也是多数用户的写法，得有用例走过。
func swarmConfig(t *testing.T, name, advertiseAddr string, nodes ...node) string {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "cluster:\n  - type: \"swarm\"\n    name: %q\n    nodes:\n", name)
	for _, n := range nodes {
		fmt.Fprintf(&b, "      - host: %q\n        ip: %q\n        role: %q\n        user: %q\n        sshKey: %q\n",
			n.host, n.ip, n.role, sshUser, keyPath)
	}
	if advertiseAddr != "" {
		fmt.Fprintf(&b, "    swarmConfig:\n      advertiseAddr: %q\n", advertiseAddr)
	}

	path := filepath.Join(t.TempDir(), "cluster.yaml")
	content := b.String()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写配置失败: %v", err)
	}
	t.Logf("集群配置 %s：\n%s", path, content)
	return path
}

// runSomcli 执行一次 somcli，返回退出码与合并输出。
func runSomcli(t *testing.T, args ...string) (int, string) {
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

// ssh 在节点上跑一条命令，只返回标准输出，stderr 折进 error。
//
// 两条流必须分开：ssh 自己会往 stderr 写 "Warning: Permanently added ..."，
// 合并的话它会被当成命令输出。
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

// nodeLs 在 manager 上跑 docker node ls，按 --format 取一列或几列。
func nodeLs(t *testing.T, format string) string {
	t.Helper()

	out, err := ssh(manager1, "docker node ls --format '"+format+"'")
	if err != nil {
		t.Fatalf("docker node ls 失败: %v\n输出：\n%s", err, out)
	}
	return out
}

// swarmState 取一台节点自己看到的 swarm 状态：active / inactive / pending。
func swarmState(t *testing.T, n node) string {
	t.Helper()

	out, err := ssh(n, "docker info --format '{{.Swarm.LocalNodeState}}' 2>/dev/null || echo no-docker")
	if err != nil {
		t.Fatalf("查 %s 的 swarm 状态失败: %v\n输出：\n%s", n.host, err, out)
	}
	return strings.TrimSpace(out)
}

// resetSwarm 把节点上的 swarm 抹掉，但保留已经装好的 docker。
//
// 用 docker swarm leave 而不是 `somcli cluster remove`：用例之间的清场必须与被测行为无关，
// 否则 remove 有问题时坏掉的是下一条用例，报错指向的地方全是错的。
// `cluster remove` 自身的正确性由 SC-S04 单独验。
func resetSwarm(t *testing.T, nodes ...node) {
	t.Helper()

	for _, n := range nodes {
		if out, err := ssh(n, "docker swarm leave --force 2>/dev/null || true"); err != nil {
			t.Logf("清场在 %s 上失败（继续）：%v\n%s", n.host, err, out)
		}
	}
}

// waitFor 反复跑 check 直到它返回 true 或超时。
func waitFor(t *testing.T, what string, timeout time.Duration, check func() (bool, string)) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var last string
	for {
		ok, detail := check()
		if ok {
			return
		}
		last = detail
		if time.Now().After(deadline) {
			t.Fatalf("等「%s」超时（%v）。最后一次看到的是：\n%s", what, timeout, last)
		}
		time.Sleep(5 * time.Second)
	}
}

// dumpNodeLogs 把现场打到测试输出里。
// CI 上容器随 runner 一起消失，不在失败时捞出来就永远看不到了。
func dumpNodeLogs() {
	if logs, err := compose("logs", "--no-color", "--tail", "200"); err == nil {
		fmt.Fprintf(os.Stderr, "=== 容器日志 ===\n%s\n", logs)
	}
	for _, n := range allNodes {
		for _, cmd := range []string{
			"journalctl -u docker --no-pager -n 100",
			"docker info 2>&1 | head -40",
			"docker node ls 2>&1",
		} {
			out, _ := ssh(n, cmd+" || true")
			fmt.Fprintf(os.Stderr, "=== %s: %s ===\n%s\n", n.host, cmd, out)
		}
	}
}
