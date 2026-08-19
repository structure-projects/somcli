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

package remote

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestSC_E03_SingleRemoteNodeInstall 单个远程节点上的安装必须真的走 SSH。
//
// 判据不能只看"文件出现了"：SSH 自连接的场景下远端与本机是同一个文件系统，
// 脚本即便被误当作本机执行，文件照样出现 —— 那正是 D1（SetNode 断链导致远程安装
// 悄悄落在操作机上）当年能长期存活的原因。
//
// 所以这里让脚本把 $SSH_CONNECTION 写进产物：这个变量只有 sshd 会注入，
// 本机 sh -c 执行时是空的。它非空，才证明命令确实经由 SSH 到达了节点。
// 分发（scp）的判据放在 multinode 组 —— 只有文件系统真正分离，才能断言"传过去了"。
func TestSC_E03_SingleRemoteNodeInstall(t *testing.T) {
	tg := sshTarget(t)

	dir := fmt.Sprintf("/tmp/somcli-e03-%d", time.Now().UnixNano())
	marker := dir + "/marker.txt"
	remoteCleanup(t, tg, dir)

	cfg := writeConfig(t, fmt.Sprintf(`
nodes:
  - host: %q
    ip: %q
    user: %q
    role: "worker"
    sshKey: %q
resources:
  - name: "remote-marker"
    version: "1.0"
    hosts:
      - %q
    pre_install:
      - "mkdir -p %s"
    post_install:
      - "echo conn=$SSH_CONNECTION > %s"
      - "echo host=$(hostname) >> %s"
`, tg.host, tg.ip, tg.user, tg.key, tg.host, dir, marker, marker))

	code, out := runIn(t, t.TempDir(), "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("远程安装失败，退出码 = %d，输出：\n%s", code, out)
	}

	if !remoteExists(t, tg, marker) {
		t.Fatalf("远端没有产物 %s，脚本没在节点上跑起来，输出：\n%s", marker, out)
	}

	content := remoteRead(t, tg, marker)
	conn := ""
	for _, line := range strings.Split(content, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "conn="); ok {
			conn = v
		}
	}
	if conn == "" {
		t.Errorf("产物里 $SSH_CONNECTION 为空 —— 脚本是在操作机上跑的，没有走 SSH（D1 复现）\n"+
			"远端产物：\n%s\n命令输出：\n%s", content, out)
	}
}

// TestSC_E03_UnreachableNodeFails 节点连不上必须失败，且错误里带得上节点 / 用户 / IP。
//
// 与 test/local 的 SC-F02 是两回事：那条用不可达地址验的是错误信息成型，
// 这条在真能连的环境里把 IP 换成不可达的，验的是"能连"与"不能连"确实走出了不同结果 ——
// 否则上面那条用例即便在一切都坏掉的环境里也可能因为别的原因过掉。
func TestSC_E03_UnreachableNodeFails(t *testing.T) {
	tg := sshTarget(t)

	// 路径带时间戳：固定路径会被上一次运行的残留污染，那样这条负向断言就恒红或恒绿。
	ghostMarker := fmt.Sprintf("/tmp/somcli-e03-ghost-%d", time.Now().UnixNano())

	cfg := writeConfig(t, fmt.Sprintf(`
nodes:
  - host: "ghost"
    ip: "192.0.2.1"
    user: %q
    role: "worker"
    sshKey: %q
resources:
  - name: "remote-marker"
    version: "1.0"
    hosts:
      - "ghost"
    post_install:
      - "echo unreachable > %s"
`, tg.user, tg.key, ghostMarker))

	code, out := runIn(t, t.TempDir(), "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("节点不可达却退出码 0，输出：\n%s", out)
	}
	for _, want := range []string{"ghost", "192.0.2.1"} {
		if !strings.Contains(out, want) {
			t.Errorf("错误信息里没有 %q，用户无从定位是哪个节点，输出：\n%s", want, out)
		}
	}
	if strings.Contains(out, "[SUCCESS]") {
		t.Errorf("失败路径上打印了 [SUCCESS]，输出：\n%s", out)
	}
	// 连不上就该什么都没做，尤其不能退化成在操作机上执行。
	if fileExistsLocally(ghostMarker) {
		t.Error("节点不可达，脚本却在操作机上执行了（D1 复现）")
	}
}
