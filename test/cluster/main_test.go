//go:build cluster

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

// Package cluster 在真节点上验证 `somcli cluster create` 装出来的集群是否真的能用。
//
// 这一组回答的是别处回答不了的问题：k8s 装成功了没有。test/local 只能验"装不成的配置
// 被拦住"，而"装得成"必须真的装一遍才知道 —— 节点 Ready、Pod 起得来、跨节点通。
//
// 用 build tag cluster 隔离（标签与目录名、与 test/matrix.yaml 的 env: [cluster] 一致），
// 默认 `go test ./...` 不跑。需要 Linux + docker，节点是 privileged 的 systemd 容器
// （见 test/fixtures/k8s-nodes/）。载体是 .github/workflows/e2e.yml。
//
// 与其他测试组一样是黑盒：不 import github.com/structure-projects/somcli/...。
package cluster

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
	fixtureDir = "../fixtures/k8s-nodes"
	sshUser    = "root"

	// systemd 从容器启动到 sshd 接受连接之间有空档；镜像还要现 build，第一次更慢。
	bootWaitStep = 3 * time.Second
	bootWaitMax  = 180 * time.Second
)

// 节点地址与 compose.yaml 一致。master2 与 lbEndpoint 只有多 master 用例用得到。
var (
	master  = node{host: "k8s-master", ip: "172.29.0.11", role: "master"}
	worker  = node{host: "k8s-worker", ip: "172.29.0.12", role: "worker"}
	master2 = node{host: "k8s-master2", ip: "172.29.0.13", role: "master"}
)

// lbEndpoint 是 fixture 里那台 haproxy，充当多 master 的稳定 apiserver 入口。
const lbEndpoint = "172.29.0.10:6443"

type node struct {
	host string
	ip   string
	role string
}

var (
	binPath string
	keyPath string
	// workdir 全包共用一个。集群用例每条都要装 containerd + runc + cni-plugins +
	// kube 三件套，一条用例一个 workdir 的话每条都要重新下一遍（几百兆），
	// 一晚上的额度全花在下载上。共用之后"装过的不再装"正是引擎的幂等能力，
	// 用例之间只重置 k8s（resetCluster），不重装二进制。
	workdir string
)

// TestMain 起停 fixture。
//
// 前置不满足时的处置与 remote / multinode 一致：本机缺 docker 属于环境不具备，跳过整包；
// CI 里则是准备步骤坏了，必须非 0 退出 —— 不让"跳过"冒充"通过"。
func TestMain(m *testing.M) {
	if err := requireDocker(); err != nil {
		if os.Getenv("CI") != "" {
			fmt.Fprintf(os.Stderr, "CI 里 docker 必须可用: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "跳过 cluster 组: %v\n", err)
		os.Exit(0)
	}

	if err := setup(); err != nil {
		dumpNodeLogs()
		teardown()
		fmt.Fprintf(os.Stderr, "准备 k8s 节点失败: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	// SOMCLI_CLUSTER_KEEP=1 时保留容器。集群类失败几乎都得进节点看 kubelet 日志，
	// 容器一删现场就没了。
	if os.Getenv("SOMCLI_CLUSTER_KEEP") == "" {
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
	dir, err := os.MkdirTemp("", "somcli-cluster-work")
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
	dir, err := os.MkdirTemp("", "somcli-cluster")
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
	out, err := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-f", keyPath, "-C", "somcli-cluster").CombinedOutput()
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

// waitForNodes 等两个节点都能 SSH 上、且 systemd 已经接管。
//
// 光能 SSH 不够：systemd 还在起服务时 `systemctl enable --now containerd` 会以
// "Transaction is destructive" 之类的理由失败，那种失败与 somcli 无关，
// 却会被记在集群安装头上。
func waitForNodes() error {
	deadline := time.Now().Add(bootWaitMax)
	for _, n := range []node{master, worker, master2} {
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

// systemdReady 判断节点上的 systemd 是否已经跑完启动。
// degraded 也算就绪：容器里总有几个单元起不来（udev 之类已经 mask 掉了，但不保证全清），
// 只要 systemd 本身在正常接受请求就够了。
func systemdReady(n node) bool {
	out, err := ssh(n, "systemctl is-system-running || true")
	if err != nil {
		return false
	}
	state := strings.TrimSpace(out)
	return state == "running" || state == "degraded"
}

// clusterConfig 写一份 k8s 集群配置，返回路径。
//
// cni 传空串表示配置里不写这个键 —— 走 somcli 自己的默认值，
// 那也是绝大多数用户实际会用的形态，得有用例走过。
//
// 不做 hack/mkclusterconfig 那种独立生成器：test/ 不能 import 本仓库的包，
// 生成器与用例只能各写一份，两份必然漂移。要手工复现时把用例打印出来的这份配置
// 直接喂给 somcli 即可（失败时用例会把路径与内容一并输出）。
func clusterConfig(t *testing.T, name, version, runtime, cni string, nodes ...node) string {
	t.Helper()
	return clusterConfigWithEndpoint(t, name, version, runtime, cni, "", nodes...)
}

// clusterConfigWithEndpoint 同上，另外写一个 controlPlaneEndpoint。
// 多 master 必须有它（没有的话 somcli 在连节点之前就拒绝，见 SC-K05）。
func clusterConfigWithEndpoint(t *testing.T, name, version, runtime, cni, endpoint string, nodes ...node) string {
	t.Helper()

	var b strings.Builder
	fmt.Fprintf(&b, "cluster:\n  - type: \"k8s\"\n    name: %q\n    nodes:\n", name)
	for _, n := range nodes {
		fmt.Fprintf(&b, "      - host: %q\n        ip: %q\n        role: %q\n        user: %q\n        sshKey: %q\n",
			n.host, n.ip, n.role, sshUser, keyPath)
	}
	fmt.Fprintf(&b, `    k8sConfig:
      version: %q
      containerRuntime: %q
      podNetworkCidr: "10.244.0.0/16"
      serviceCidr: "10.96.0.0/12"
`, version, runtime)
	if cni != "" {
		fmt.Fprintf(&b, "      cni: %q\n", cni)
	}
	if endpoint != "" {
		fmt.Fprintf(&b, "      controlPlaneEndpoint: %q\n", endpoint)
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
// 集群安装动辄几分钟，超时交给 `go test -timeout` 统一管，这里不再叠一层。
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
// 合并的话它会被当成命令输出，"节点上读到的内容等于什么"这类断言全部落空。
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

// kubectl 在 master 上执行 kubectl。
//
// 走节点上的 admin.conf 而不是把 kubeconfig 拉回操作机：kubeconfig 里的 server
// 地址是节点视角的，拉回来往往连不上，那种失败与集群本身无关。
func kubectl(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return ssh(master, "KUBECONFIG=/etc/kubernetes/admin.conf kubectl "+strings.Join(args, " "))
}

// waitFor 反复跑 check 直到它返回 true 或超时。
// 集群里几乎没有立刻成立的断言：kubeadm init 返回之后节点还要等 CNI 才 Ready。
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

// resetCluster 把节点上的 k8s 抹掉，但保留已经装好的二进制。
//
// 用 kubeadm reset 而不是 `somcli cluster remove`：用例之间的清场必须与被测行为无关，
// 否则 remove 有问题时，坏掉的是下一条用例，报错指向的地方全是错的。
// `cluster remove` 自身的正确性由 lifecycle 那条用例单独验（SC-K11）。
func resetCluster(t *testing.T, nodes ...node) {
	t.Helper()

	for _, n := range nodes {
		for _, cmd := range []string{
			"kubeadm reset -f --cri-socket unix:///var/run/containerd/containerd.sock || true",
			"rm -rf /etc/kubernetes /etc/cni/net.d $HOME/.kube",
			// CNI 留下的网桥不删掉的话，下一次装出来的 Pod 会拿到上一次网段里的地址
			"ip link delete cni0 2>/dev/null || true",
			"ip link delete flannel.1 2>/dev/null || true",
		} {
			if out, err := ssh(n, cmd); err != nil {
				t.Logf("清场命令在 %s 上失败（继续）：%s\n%v\n%s", n.host, cmd, err, out)
			}
		}
	}
}

// applyManifest 把一段清单喂给 master 上的 kubectl。
//
// 走 stdin 而不是先落文件：清单是用例自己的夹具，不该在节点上留下需要清理的东西。
func applyManifest(t *testing.T, manifest string) {
	t.Helper()

	cmd := "KUBECONFIG=/etc/kubernetes/admin.conf kubectl apply -f - <<'SOMCLI_TEST_EOF'\n" +
		manifest + "\nSOMCLI_TEST_EOF"
	if out, err := ssh(master, cmd); err != nil {
		t.Fatalf("apply 清单失败: %v\n输出：\n%s\n清单：\n%s", err, out, manifest)
	}
}

// dumpNodeLogs 把现场打到测试输出里。
//
// 集群失败光看 somcli 的输出基本判断不了原因 —— 真正的线索在节点的 kubelet 日志、
// containerd 配置和 kubeadm 的 preflight 里。CI 上容器随 runner 一起消失，
// 不在失败时捞出来就永远看不到了。
func dumpNodeLogs() {
	if logs, err := compose("logs", "--no-color", "--tail", "200"); err == nil {
		fmt.Fprintf(os.Stderr, "=== 容器日志 ===\n%s\n", logs)
	}
	for _, n := range []node{master, worker, master2} {
		for _, cmd := range []string{
			"journalctl -u kubelet --no-pager -n 100",
			"journalctl -u containerd --no-pager -n 50",
			"cat /etc/containerd/config.toml 2>/dev/null",
			"cat /etc/sysctl.d/k8s.conf 2>/dev/null",
			"ls -l /etc/kubernetes 2>/dev/null",
		} {
			out, _ := ssh(n, cmd+" || true")
			fmt.Fprintf(os.Stderr, "=== %s: %s ===\n%s\n", n.host, cmd, out)
		}
	}
}
