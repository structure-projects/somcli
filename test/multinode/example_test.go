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

package multinode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSC_E07_RemoteExampleIsExecutable configs/examples/remote-3node.yaml 必须真能跑通。
//
// 文档里的示例配置如果只是"长得像"，读者照着改一定踩坑。这里直接把仓库里那份示例喂给
// 三节点 fixture：唯一的替换是 sshKey —— 示例里写的是 ~/.ssh/id_rsa（给人看的真实默认值），
// 测试换成 fixture 临时生成的私钥。节点、IP、hosts 点名、脚本内容一字不改。
func TestSC_E07_RemoteExampleIsExecutable(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot, "configs", "examples", "remote-3node.yaml"))
	if err != nil {
		t.Fatalf("读取示例配置失败: %v", err)
	}
	rendered := strings.ReplaceAll(string(raw), "~/.ssh/id_rsa", keyPath)

	cfg := filepath.Join(t.TempDir(), "remote-3node.yaml")
	if err := os.WriteFile(cfg, []byte(rendered), 0o600); err != nil {
		t.Fatalf("写临时配置失败: %v", err)
	}

	code, out := runIn(t, t.TempDir(), "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("示例配置安装失败，退出码 = %d，输出：\n%s", code, out)
	}

	for _, n := range nodes {
		if got := nodeFile(t, n, "/opt/somcli-demo/node.txt"); got != n.host {
			t.Errorf("节点 %s 上的 node.txt = %q，期望 %q；脚本没有分别落到各自节点\n输出：\n%s",
				n.host, got, n.host, out)
		}
	}

	// 第二个资源只点名 node-a（用 IP），target.txt 只能在 node-a 上。
	if got := nodeFile(t, nodes[0], "/opt/somcli-demo/target.txt"); got != "installed-on-a" {
		t.Errorf("node-a 上 target.txt = %q，期望 installed-on-a\n输出：\n%s", got, out)
	}
	for _, n := range nodes[1:] {
		if got := nodeFile(t, n, "/opt/somcli-demo/target.txt"); got != "" {
			t.Errorf("节点 %s 不该有 target.txt，却读到 %q（hosts 点名没有排他）", n.host, got)
		}
	}
}
