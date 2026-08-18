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
	"reflect"
	"strings"
	"testing"
)

// TestSC_F01_UnknownHostAbortsWithoutSideEffect hosts 引用未声明的主机时必须失败，
// 且**在操作机上不留任何痕迹**。
//
// 这是 D1 的本地回归。曾经 GetNode 查不到主机就返回 {IP: "127.0.0.1"}，于是声明给远程
// 节点的脚本被静默地在操作机上执行 —— 真实配置里那些脚本是 swapoff -a、yum install、
// systemctl enable kubelet。所以这里不只断言退出码：脚本一旦被执行就会留下标记文件，
// workdir 必须干净，错误信息里必须出现拼错的主机名和已声明的节点，用户才知道错在哪。
func TestSC_F01_UnknownHostAbortsWithoutSideEffect(t *testing.T) {
	workdir := t.TempDir()
	before := tree(t, workdir)

	cfg := writeConfig(t, `
nodes:
  - host: node-a
    ip: 192.0.2.10
    user: root
    sshKey: ~/.ssh/id_rsa
resources:
  - name: remote-tool
    version: "1.0"
    hosts:
      - node-x
    pre_install:
      - "echo leaked > {{.WorkDir}}/leaked.txt"
    post_install:
      - "echo leaked > {{.WorkDir}}/leaked.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)

	if code == 0 {
		t.Fatalf("hosts 指向未声明的 node-x 却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "node-x") {
		t.Errorf("错误信息未指出无法解析的主机名 node-x，输出：\n%s", out)
	}
	if !strings.Contains(out, "node-a") {
		t.Errorf("错误信息未列出已声明的节点，用户无从对照，输出：\n%s", out)
	}
	if strings.Contains(out, "[SUCCESS]") {
		t.Errorf("失败路径上打印了 [SUCCESS]，输出：\n%s", out)
	}

	if after := tree(t, workdir); !reflect.DeepEqual(before, after) {
		t.Errorf("节点解析失败却在操作机上产生了副作用：\n之前 %v\n之后 %v\n输出：\n%s", before, after, out)
	}
}

// TestSC_F01_NoNodesDeclaredIsAlsoAnError 一个节点都没声明时更要报错。
//
// 这正是 cluster create 曾经的处境：解析器根本没调用 SetNode，节点表恒为空，
// 于是所有主机都"解析"成了本机。错误信息要提示去补 nodes:，而不是抛一句查不到。
func TestSC_F01_NoNodesDeclaredIsAlsoAnError(t *testing.T) {
	workdir := t.TempDir()
	before := tree(t, workdir)

	cfg := writeConfig(t, `
resources:
  - name: remote-tool
    version: "1.0"
    hosts:
      - node-a
    post_install:
      - "echo leaked > {{.WorkDir}}/leaked.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)

	if code == 0 {
		t.Fatalf("未声明任何 nodes 却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "nodes") {
		t.Errorf("错误信息未提示补 nodes: 列表，输出：\n%s", out)
	}
	if after := tree(t, workdir); !reflect.DeepEqual(before, after) {
		t.Errorf("解析失败却产生了副作用：\n之前 %v\n之后 %v", before, after)
	}
}

// TestSC_F01_ExplicitLocalhostStillRunsLocally 显式写本机时仍然在本机执行。
//
// 去掉兜底不能把"本机编排"一起去掉：现有示例配置大量使用 hosts: ["127.0.0.1"]。
// 判据是它必须真的执行 —— 只看退出码 0 的话，一个跳过全部脚本的实现也能通过。
func TestSC_F01_ExplicitLocalhostStillRunsLocally(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "localhost"} {
		host := host
		t.Run("SC-F01/"+host, func(t *testing.T) {
			workdir := t.TempDir()
			cfg := writeConfig(t, `
resources:
  - name: local-tool
    version: "1.0"
    hosts:
      - "`+host+`"
    post_install:
      - "echo ran-on-local > {{.WorkDir}}/local.txt"
`)

			code, out := runIn(t, workdir, "install", "-f", cfg)
			if code != 0 {
				t.Fatalf("hosts 显式写 %s 却退出码 = %d，输出：\n%s", host, code, out)
			}
			if got := strings.TrimSpace(readFile(t, workdir+"/local.txt")); got != "ran-on-local" {
				t.Errorf("本机脚本未执行，产物 = %q，输出：\n%s", got, out)
			}
		})
	}
}
