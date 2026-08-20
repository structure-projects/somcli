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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSC_F03_TildeSSHKeyWorksOnBothPaths 配置里 sshKey 写成 ~/... 时，执行与分发都要认。
//
// 这是 D13 的回归。当时只有远程执行调了展开，分发路径拿的是配置里的原样字符串；
// scp / ssh 是被 exec 直接拉起的，没有 shell 帮忙展开 ~，于是"能远程执行、但一分发就失败"。
// 所有示例配置写的都是 sshKey: "~/.ssh/id_rsa"，等于对所有人都踩得到。
//
// 两条路径必须都断言：只验执行的话，D13 原样存活也能过；只验分发的话，
// 把展开写在分发里而漏掉执行同样过不了关。
func TestSC_F03_TildeSSHKeyWorksOnBothPaths(t *testing.T) {
	tg := sshTarget(t)
	key := tildeKey(t, tg)

	dir := fmt.Sprintf("/tmp/somcli-f03-%d", time.Now().UnixNano())
	marker := dir + "/exec.txt"
	conf := dir + "/dispatched.json"
	remoteCleanup(t, tg, dir)

	// 一份配置里两个资源：一个只跑脚本（执行路径），一个带 extra_files（分发路径）。
	// 分开写是为了失败时能一眼看出断在哪条路上。
	cfg := writeConfig(t, fmt.Sprintf(`
nodes:
  - host: %q
    ip: %q
    user: %q
    role: "worker"
    sshKey: %q
resources:
  - name: "f03-exec"
    version: "1.0"
    hosts:
      - %q
    pre_install:
      - "mkdir -p %s"
    post_install:
      - "echo conn=$SSH_CONNECTION > %s"
  - name: "f03-dispatch"
    version: "1.0"
    hosts:
      - %q
    extra_files:
      %q: "{\"tilde\":\"ok\"}\n"
`, tg.host, tg.ip, tg.user, key, tg.host, dir, marker, tg.host, conf))

	code, out := runIn(t, t.TempDir(), "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("sshKey 写成 %s 时安装失败，退出码 = %d（D13 复现）\n输出：\n%s", key, code, out)
	}

	// 执行路径：$SSH_CONNECTION 只有 sshd 会注入，非空才证明命令真的经 SSH 到达了节点。
	if !remoteExists(t, tg, marker) {
		t.Fatalf("执行路径没有产物 %s，输出：\n%s", marker, out)
	}
	if v := strings.TrimSpace(strings.TrimPrefix(remoteRead(t, tg, marker), "conn=")); v == "" {
		t.Errorf("执行路径的 $SSH_CONNECTION 为空 —— 命令跑在操作机上，没走 SSH\n输出：\n%s", out)
	}

	// 分发路径：附加文件的暂存件要先经 CopyToRemote 才能就位。~ 没展开的话，
	// CopyToRemote 的第一个 ssh 探测就会以"检查远程文件失败"告终，整条安装非 0 退出。
	if !remoteExists(t, tg, conf) {
		t.Fatalf("分发路径没有把附加文件放到 %s（D13 复现）\n输出：\n%s", conf, out)
	}
	if got := strings.TrimSpace(remoteRead(t, tg, conf)); got != `{"tilde":"ok"}` {
		t.Errorf("附加文件内容是 %q，期望 %q", got, `{"tilde":"ok"}`)
	}
	// 分发路径失败时 somcli 会把这句写进错误；出现它就说明这条断言不是蒙过去的。
	if strings.Contains(out, "分发附加文件") && strings.Contains(out, "失败") {
		t.Errorf("分发报了失败却仍然退出 0，输出：\n%s", out)
	}
}

// tildeKey 把探测用的私钥换算成 ~ 形式写进配置。
//
// 不直接用 tg.key：SOMCLI_TEST_SSH_KEY 可以被设成绝对路径，那样这条用例会悄悄退化成
// "绝对路径能用"，与要验的东西无关。钥匙不在 HOME 下就不装作验过了。
//
// CI 上按 sshTarget 的同一条规矩硬失败而不是跳过：那里的私钥由流水线自己生成在
// ~/.ssh/id_rsa，构造不出 ~ 形式只可能是准备步骤坏了。默默跳过的话，
// 一条 matrix 里标成 done 的场景实际什么都没跑。
func tildeKey(t *testing.T, tg target) string {
	t.Helper()

	giveUp := func(format string, args ...any) {
		t.Helper()
		if os.Getenv("CI") != "" {
			t.Fatalf(format+"\nCI 里这不是环境不具备，而是准备步骤坏了", args...)
		}
		t.Skipf(format, args...)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		giveUp("取不到 HOME，无法构造 ~ 形式的 sshKey: %v", err)
	}
	rel, err := filepath.Rel(home, expand(tg.key))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		giveUp("私钥 %s 不在 HOME(%s) 下，构造不出 ~ 形式；本条要验的正是 ~ 展开",
			expand(tg.key), home)
	}
	return "~/" + filepath.ToSlash(rel)
}
