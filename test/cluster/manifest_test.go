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
package cluster

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestSC_C06_Apply somcli apply 必须真的把清单应用进集群。
//
// 这条只能在真集群上跑：apply 走的是"先认出当前机器上是什么集群，再转给 kubectl"，
// 认不出集群时直接退 1 —— 也就是说在操作机上永远只能验到"没有集群"这一个分支。
//
// 而 somcli 是在**节点上**才看得到集群的（kubeconfig 在 master 的 $HOME/.kube 下），
// 所以这条用例把二进制送进 master 再在那儿执行，与用户登上 master 敲命令是同一回事。
//
// 判据取"集群里真的多了这个对象"，不取退出码：apply 成功与否是 kubectl 说的，
// 而"打印 Resources applied successfully"这件事本身不需要任何东西真的生效。
func TestSC_C06_Apply(t *testing.T) {
	t.Cleanup(func() {
		if t.Failed() {
			dumpNodeLogs()
		}
	})

	resetCluster(t, master, worker)

	cfg := clusterConfig(t, "sc-c06", "1.28.2", "containerd", "", master)
	if code, out := runSomcli(t, "cluster", "create", "-f", cfg); code != 0 {
		t.Fatalf("cluster create 退出码 = %d\n输出：\n%s", code, out)
	}

	// apiserver 得先能应答，否则失败的是"集群还没起来"而不是 apply。
	waitFor(t, "apiserver 可用", 5*time.Minute, func() (bool, string) {
		got, err := kubectl(t, "get", "nodes", "--no-headers")
		if err != nil {
			return false, err.Error()
		}
		return strings.TrimSpace(got) != "", got
	})

	nodeBin := deploySomcliToNode(t, master)

	t.Run("清单被应用进集群", func(t *testing.T) {
		const manifest = `apiVersion: v1
kind: ConfigMap
metadata:
  name: sc-c06-probe
  namespace: default
data:
  from: "somcli-apply"
`
		path := writeFileOnNode(t, master, "/tmp/sc-c06-probe.yaml", manifest)

		out, err := ssh(master, nodeBin+" apply "+path)
		if err != nil {
			t.Fatalf("somcli apply 失败: %v\n输出：\n%s", err, out)
		}

		got, err := kubectl(t, "get", "configmap", "sc-c06-probe", "-o", "jsonpath={.data.from}")
		if err != nil {
			t.Fatalf("apply 报成功，集群里却查不到这个对象: %v\n输出：\n%s\napply 的输出：\n%s",
				err, got, out)
		}
		if strings.TrimSpace(got) != "somcli-apply" {
			t.Errorf("对象内容不对，期望 somcli-apply，实际 %q", strings.TrimSpace(got))
		}
	})

	// 成对的反面：只验"应用成功"的话，一个不管清单内容一律报成功的实现同样能过。
	t.Run("清单有问题时不许报成功", func(t *testing.T) {
		path := writeFileOnNode(t, master, "/tmp/sc-c06-bad.yaml", "这不是一份清单\n")

		out, err := ssh(master, nodeBin+" apply "+path)
		if err == nil {
			t.Fatalf("喂了一份不是清单的文件，somcli apply 却退出 0，输出：\n%s", out)
		}
	})
}

// deploySomcliToNode 把 somcli 送到节点上并返回它在节点上的路径。
//
// 不复用 TestMain 里编好的那份：那份是给操作机用的，操作机可能是 macOS，
// 而节点一定是 Linux。按节点的目标平台单独编一份，免得这条用例在 macOS 上
// 挂在"二进制格式不对"这种与被测行为无关的地方。
func deploySomcliToNode(t *testing.T, n node) string {
	t.Helper()

	local := filepath.Join(t.TempDir(), "somcli")
	cmd := exec.Command("go", "build", "-o", local, ".")
	cmd.Dir = repoRoot
	// 节点是容器，与操作机共享内核架构。
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH="+runtime.GOARCH, "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("为节点编译 somcli 失败: %v\n%s", err, out)
	}

	remote := "/usr/local/bin/somcli"
	scp := exec.Command("scp",
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "BatchMode=yes",
		local, fmt.Sprintf("%s@%s:%s", sshUser, n.ip, remote))
	if out, err := scp.CombinedOutput(); err != nil {
		t.Fatalf("把 somcli 拷到 %s 失败: %v\n%s", n.host, err, out)
	}
	if out, err := ssh(n, "chmod +x "+remote+" && "+remote+" version"); err != nil {
		t.Fatalf("节点上的 somcli 跑不起来: %v\n输出：\n%s", err, out)
	}
	return remote
}

// writeFileOnNode 在节点上落一个文件，返回路径。
func writeFileOnNode(t *testing.T, n node, path, content string) string {
	t.Helper()

	cmd := fmt.Sprintf("cat > %s <<'SOMCLI_TEST_EOF'\n%s\nSOMCLI_TEST_EOF", path, content)
	if out, err := ssh(n, cmd); err != nil {
		t.Fatalf("在 %s 上写 %s 失败: %v\n%s", n.host, path, err, out)
	}
	return path
}
