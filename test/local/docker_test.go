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

// writeNodesFile 落一份只含 nodes: 段的配置。
func writeNodesFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "nodes.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写节点配置失败: %v", err)
	}
	return path
}

// SC-C01：`docker install -f nodes.yaml` 必须真的解析出节点。
//
// 原实现的 loadNodesFromFile 是个占位：读完文件就 `return nil, "not implemented yet"`，
// 于是这条命令无论配置怎么写都失败在同一句话上（D7）。
// 判据取"错误信息里出现了配置声明的节点"—— 本机不可能真去装 docker，
// 但只有解析成功了才可能拿着那个 IP 去连接。
func TestSC_C01_InstallParsesNodesFromFile(t *testing.T) {
	// .invalid 是 RFC 2606 保留的 TLD，永远解析不出来：连接立刻失败，
	// 既不会打到真实主机，也不用等 TCP 超时
	nodes := writeNodesFile(t, `
nodes:
  - host: "doc-node"
    ip: "doc-node.invalid"
    user: "root"
`)

	code, out := run(t, "docker", "install", "-f", nodes, "--ssh-key", "/dev/null")
	if code == 0 {
		t.Fatalf("连不上的节点却退出 0，输出：\n%s", out)
	}
	if strings.Contains(out, "not implemented") {
		t.Fatalf("节点配置解析仍是占位实现，输出：\n%s", out)
	}
	if !strings.Contains(out, "doc-node.invalid") {
		t.Fatalf("失败信息里没有出现配置声明的节点地址，输出：\n%s", out)
	}
}

// SC-C01：节点配置走的是统一解析器，拼错的键要报错而不是被忽略。
//
// 这条同时守住"docker 子系统别再另起一套 schema"：统一解析器用的是 UnmarshalStrict。
func TestSC_C01_UnknownNodeKeyRejected(t *testing.T) {
	nodes := writeNodesFile(t, `
nodes:
  - host: "doc-node"
    ip: "doc-node.invalid"
    userr: "root"
`)

	code, out := run(t, "docker", "install", "-f", nodes)
	if code == 0 {
		t.Fatalf("配置里有拼错的键却退出 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "userr") {
		t.Fatalf("错误信息没有点出拼错的键，输出：\n%s", out)
	}
}

// SC-C01：文件里没有 nodes: 段要说清楚，而不是当成"没有目标"静默成功。
func TestSC_C01_EmptyNodesFileFailsLoudly(t *testing.T) {
	nodes := writeNodesFile(t, "nodes: []\n")

	code, out := run(t, "docker", "install", "-f", nodes)
	if code == 0 {
		t.Fatalf("没有节点却退出 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "nodes") {
		t.Fatalf("错误信息没有提到 nodes，输出：\n%s", out)
	}
}

// SC-C01：--node 与 -f 都能指定目标，且都能带上 --user / --ssh-key。
//
// 这条守的是命令行那条路径没被文件路径的修复带坏。
func TestSC_C01_NodeFlagAlsoResolves(t *testing.T) {
	code, out := run(t, "docker", "install", "--node", "doc-node.invalid", "--user", "root")
	if code == 0 {
		t.Fatalf("连不上的节点却退出 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "doc-node.invalid") {
		t.Fatalf("失败信息里没有出现 --node 指定的地址，输出：\n%s", out)
	}
}
