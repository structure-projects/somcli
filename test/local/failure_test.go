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

// TestSC_F04_PreInstallFailureAborts pre_install 失败即中止，post_install 一步都不许跑。
//
// D2 的回归。曾经 RunScripts 把错误 PrintDebug 掉就 return nil，于是前置脚本失败后
// 后置脚本照样执行、最后照样打印 ✓ 成功。对安装流程来说这是最坏的一种：前置条件没满足
// 却继续往下装，故障被推到更远的地方才暴露。
func TestSC_F04_PreInstallFailureAborts(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: aborted-tool
    version: "1.0"
    pre_install:
      - "echo pre-ran > {{.WorkDir}}/pre.txt"
      - "exit 3"
      - "echo after-failure > {{.WorkDir}}/never-pre.txt"
    post_install:
      - "echo post-ran > {{.WorkDir}}/never-post.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)

	if code == 0 {
		t.Fatalf("pre_install 退出 3 却退出码 0，输出：\n%s", out)
	}
	if strings.Contains(out, "[SUCCESS]") {
		t.Errorf("失败路径上打印了 [SUCCESS]，输出：\n%s", out)
	}
	// 失败之前的那一步应当已经执行，用来证明用例确实走到了脚本阶段。
	if got := strings.TrimSpace(readFile(t, filepath.Join(workdir, "pre.txt"))); got != "pre-ran" {
		t.Errorf("失败前的脚本没执行，用例没有真正走到脚本阶段，产物 = %q", got)
	}
	for _, name := range []string{"never-pre.txt", "never-post.txt"} {
		if _, err := os.Stat(filepath.Join(workdir, name)); !os.IsNotExist(err) {
			t.Errorf("pre_install 失败后仍继续执行，产生了 %s。输出：\n%s", name, out)
		}
	}
}

// TestSC_F04_MultiResourceAbortsAtFirstFailure 前一个资源失败后不再装后面的。
// 数组顺序表达的是依赖关系，前置资源没装成还继续往下，只会得到一堆更难诊断的错误。
func TestSC_F04_MultiResourceAbortsAtFirstFailure(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: first
    version: "1.0"
    post_install:
      - "echo first >> {{.WorkDir}}/order.txt"
  - name: second
    version: "1.0"
    post_install:
      - "exit 4"
  - name: third
    version: "1.0"
    post_install:
      - "echo third >> {{.WorkDir}}/order.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)

	if code == 0 {
		t.Fatalf("中间的资源失败却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "second") {
		t.Errorf("错误信息未指出失败的资源名，输出：\n%s", out)
	}
	got := lines(t, filepath.Join(workdir, "order.txt"))
	if len(got) != 1 || got[0] != "first" {
		t.Errorf("失败后仍继续安装后续资源，order.txt = %v，期望只有 [first]。输出：\n%s", got, out)
	}
}

// TestSC_F02_UnreachableNodeReportsNodeUserIP SSH 不可达时的错误信息要够用。
//
// 192.0.2.1 属 TEST-NET-1（RFC 5737），保证不可路由，因此这条用例不依赖外网可达性。
// 排查 SSH 问题要的三样东西是节点名、登录用户、IP —— 缺一样就得回去翻配置。
// 注意断言里还要求不出现 127.0.0.1：那意味着又兜底回本机了。
func TestSC_F02_UnreachableNodeReportsNodeUserIP(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
nodes:
  - host: unreachable-node
    ip: 192.0.2.1
    user: deploy
    sshKey: ~/.ssh/id_rsa
resources:
  - name: remote-tool
    version: "1.0"
    hosts:
      - unreachable-node
    post_install:
      - "echo should-not-run-here > {{.WorkDir}}/local-leak.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)

	if code == 0 {
		t.Fatalf("节点不可达却退出码 0，输出：\n%s", out)
	}
	for _, want := range []string{"unreachable-node", "192.0.2.1"} {
		if !strings.Contains(out, want) {
			t.Errorf("错误信息缺少 %q，排查时无从定位，输出：\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(workdir, "local-leak.txt")); !os.IsNotExist(err) {
		t.Errorf("远程节点不可达时脚本落到了操作机上执行。输出：\n%s", out)
	}
	if strings.Contains(out, "[SUCCESS]") {
		t.Errorf("失败路径上打印了 [SUCCESS]，输出：\n%s", out)
	}
}