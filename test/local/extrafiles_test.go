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

// TestSC_E11_ExtraFilesRendered extra_files 渲染落盘：内容、父目录、权限。
//
// D3：这个字段过去连 yaml tag 都是错的（extraFiles 而非 extra_files），
// 于是配置里写了也解析不出来，更没有任何代码去消费它 —— 而 daemon.json、
// containerd.service 这类"装之前得先把配置文件放好"是安装编排的日常需求。
//
// 判据落在三处：文件内容（模板必须渲染，包括 vars）、父目录（不存在要自己建）、
// 权限（0644，配置文件不该带可执行位）。
func TestSC_E11_ExtraFilesRendered(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
vars:
  mirror: "registry.example.com"
resources:
  - name: docker
    version: "24.0.7"
    extra_files:
      "{{.WorkDir}}/etc/docker/daemon.json": |
        {
          "registry-mirrors": ["https://{{.Vars.mirror}}"],
          "installed-version": "{{.Version}}"
        }
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("extra_files 安装退出码 = %d，输出：\n%s", code, out)
	}

	dest := filepath.Join(workdir, "etc", "docker", "daemon.json")
	got := readFile(t, dest)
	for _, want := range []string{`"https://registry.example.com"`, `"installed-version": "24.0.7"`} {
		if !strings.Contains(got, want) {
			t.Errorf("附加文件内容缺少 %q，实际内容：\n%s", want, got)
		}
	}
	if strings.Contains(got, "{{") {
		t.Errorf("附加文件内容里还留着未渲染的模板：\n%s", got)
	}

	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("附加文件不存在: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("附加文件权限 = %v，期望 0644（配置文件不该可执行）", perm)
	}
}

// TestSC_E11_ExtraFilesReadyBeforePreInstall 附加文件在 pre_install 之前就位。
//
// 顺序是这个字段能不能用的关键："先放配置文件、再启服务"是它的主要用途，
// 反过来的话 pre_install 里的 test -f 一定失败，用户只能把内容重新用 echo 写一遍。
func TestSC_E11_ExtraFilesReadyBeforePreInstall(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: ordered
    version: "1.0"
    extra_files:
      "{{.WorkDir}}/conf/app.conf": "key=value\n"
    pre_install:
      - "test -f {{.WorkDir}}/conf/app.conf"
      - "cat {{.WorkDir}}/conf/app.conf > {{.WorkDir}}/seen-by-pre.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("pre_install 没看到附加文件，退出码 = %d，输出：\n%s", code, out)
	}
	if got := strings.TrimSpace(readFile(t, filepath.Join(workdir, "seen-by-pre.txt"))); got != "key=value" {
		t.Errorf("pre_install 读到的附加文件内容 = %q，期望 %q", got, "key=value")
	}
}

// TestSC_E11_ExtraFilesTemplateErrorFails 附加文件里引用不存在的变量必须报错。
//
// 与 SC-E12/SC-F06 同一个理由：渲染成空串会落下一份少了字段的配置文件，
// 而"服务起来了但行为不对"比"装不上"难查得多。
func TestSC_E11_ExtraFilesTemplateErrorFails(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: broken
    version: "1.0"
    extra_files:
      "{{.WorkDir}}/conf/app.conf": "mirror={{.Vars.nosuch}}\n"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("附加文件引用未声明的变量却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "nosuch") {
		t.Errorf("错误信息未指出变量名，输出：\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(workdir, "conf", "app.conf")); !os.IsNotExist(err) {
		t.Errorf("渲染失败却仍然落下了附加文件。输出：\n%s", out)
	}
}
