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

// TestSource_OfficialRunsNoSetup mode: official 等价于不配置：装包前不做任何换源动作。
//
// 判据：apt-get 只收到 install，没有 update；mount/sed 一次都没被调用。
// 否则"留空按 official 处理"就是句空话。
func TestSource_OfficialRunsNoSetup(t *testing.T) {
	aptLog := filepath.Join(t.TempDir(), "apt.log")
	mountLog := filepath.Join(t.TempDir(), "mount.log")
	bin := fakeBinDir(t, map[string]string{
		"apt-get": recorderScript(aptLog),
		"mount":   recorderScript(mountLog),
	})

	cfg := writeConfig(t, `
source:
  mode: "official"
resources:
  - name: jq
    method: "package"
`)

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin}, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("official 源下退出码 = %d，输出：\n%s", code, out)
	}

	got := strings.TrimSpace(readFile(t, aptLog))
	if !strings.Contains(got, "install") || !strings.Contains(got, "jq") {
		t.Errorf("apt-get 收到 %q，期望 install jq", got)
	}
	if strings.Contains(got, "update") {
		t.Errorf("official 不应触发 apt-get update，apt-get 收到：%q", got)
	}
	if _, err := os.Stat(mountLog); !os.IsNotExist(err) {
		t.Errorf("official 不应调用 mount，收到：%q", strings.TrimSpace(readFile(t, mountLog)))
	}
}

// TestSource_AliyunAptRefreshesMetadata mode: aliyun 在 apt 系上先换源再 apt-get update，
// 之后才装包。update 必须出现在 install 之前。
func TestSource_AliyunAptRefreshesMetadata(t *testing.T) {
	logDir := t.TempDir()
	aptLog := filepath.Join(logDir, "apt.log")
	// sed 给个空实现：万一本机真有 /etc/apt/sources.list，也不能让它去改真文件。
	bin := fakeBinDir(t, map[string]string{
		"apt-get": recorderScript(aptLog),
		"sed":     "#!/bin/sh\nexit 0\n",
	})

	cfg := writeConfig(t, `
source:
  mode: "aliyun"
resources:
  - name: jq
    method: "package"
`)

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin}, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("aliyun 源下退出码 = %d，输出：\n%s", code, out)
	}

	got := readFile(t, aptLog)
	updateIdx := strings.Index(got, "update")
	installIdx := strings.Index(got, "install")
	if updateIdx < 0 {
		t.Fatalf("aliyun 应先 apt-get update，apt-get 收到：%q。输出：\n%s", got, out)
	}
	if installIdx < 0 || updateIdx > installIdx {
		t.Errorf("update 必须在 install 之前，apt-get 收到：%q", got)
	}
}

// TestSource_AliyunDnfMakecache mode: aliyun 在 dnf 系上跑 dnf makecache。
// apt-get 不在 PATH 里，确保走的是 dnf 分支而不是默认落到 apt。
func TestSource_AliyunDnfMakecache(t *testing.T) {
	dnfLog := filepath.Join(t.TempDir(), "dnf.log")
	bin := fakeBinDir(t, map[string]string{
		"dnf": recorderScript(dnfLog),
		"sed": "#!/bin/sh\nexit 0\n",
	})

	cfg := writeConfig(t, `
source:
  mode: "aliyun"
resources:
  - name: make
    method: "package"
`)

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin}, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("aliyun+dnf 退出码 = %d，输出：\n%s", code, out)
	}

	got := readFile(t, dnfLog)
	if !strings.Contains(got, "makecache") {
		t.Errorf("aliyun dnf 分支应 makecache，dnf 收到：%q", got)
	}
	if !strings.Contains(got, "install") || !strings.Contains(got, "make") {
		t.Errorf("换源后仍应装包，dnf 收到：%q", got)
	}
}

// TestSource_ISOMountsAndRefreshes mode: iso 挂载目标节点上的 ISO、写 repo 文件、刷新元数据。
//
// 用 --sudo + 假 sudo（记完参数再 exec）让所有命令都落到假命令上，这样不碰真 /etc、
// 不需要 root 就能断言：mount 带 loop,ro 与给定路径、dnf makecache 被调用。
func TestSource_ISOMountsAndRefreshes(t *testing.T) {
	logDir := t.TempDir()
	mountLog := filepath.Join(logDir, "mount.log")
	dnfLog := filepath.Join(logDir, "dnf.log")
	teeLog := filepath.Join(logDir, "tee.log")

	bin := fakeBinDir(t, map[string]string{
		// mountpoint 默认返回 1（未挂载），于是挂载分支真的执行到 mount。
		"mountpoint": "#!/bin/sh\nexit 1\n",
		"mount":      recorderScript(mountLog),
		"mkdir":      "#!/bin/sh\nexit 0\n",
		"tee":        recorderScript(teeLog) + "cat >/dev/null\n",
		"dnf":        recorderScript(dnfLog),
		"sudo":       recorderScript(filepath.Join(logDir, "sudo.log")) + "exec \"$@\"\n",
	})

	cfg := writeConfig(t, `
source:
  mode: "iso"
  iso: "/data/CentOS-7-x86_64.iso"
  mount: "/mnt/myiso"
resources:
  - name: make
    method: "package"
`)

	code, out := runEnvIn(t, t.TempDir(),
		[]string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH")},
		"install", "-f", cfg, "--sudo")
	if code != 0 {
		t.Fatalf("iso 源退出码 = %d，输出：\n%s", code, out)
	}

	mountGot := readFile(t, mountLog)
	if !strings.Contains(mountGot, "loop,ro") ||
		!strings.Contains(mountGot, "/data/CentOS-7-x86_64.iso") ||
		!strings.Contains(mountGot, "/mnt/myiso") {
		t.Errorf("mount 调用 = %q，期望带 loop,ro 与 iso/mount 路径", mountGot)
	}

	teeGot := readFile(t, teeLog)
	if !strings.Contains(teeGot, "somcli-iso.repo") {
		t.Errorf("tee 应写出 somcli-iso.repo，收到：%q", teeGot)
	}

	dnfGot := readFile(t, dnfLog)
	if !strings.Contains(dnfGot, "makecache") {
		t.Errorf("iso dnf 分支应 makecache，dnf 收到：%q", dnfGot)
	}
	if !strings.Contains(dnfGot, "install") || !strings.Contains(dnfGot, "make") {
		t.Errorf("配好本地源后仍应装包，dnf 收到：%q", dnfGot)
	}
}

// TestSource_ISOAptExpandsVariables apt 分支写 sources.list 时，$MOUNT/$VERSION_CODENAME/
// $COMPONENTS 必须在写入前展开，不能把字面量 $VAR 写进文件。
func TestSource_ISOAptExpandsVariables(t *testing.T) {
	// 生成的 shell 在目标节点上 source /etc/os-release 取发行版代号；非交互 sh 下
	// 该文件不存在会直接中止。真 Debian/Ubuntu 必有此文件，Linux CI 会跑到；
	// macOS 开发机没有，无法在不碰真系统、不为可测性改生产代码的前提下模拟，故跳过。
	if _, err := os.Stat("/etc/os-release"); err != nil {
		t.Skip("主机无 /etc/os-release（非 Linux），apt ISO 分支的变量展开交由 Linux CI 验证")
	}
	logDir := t.TempDir()
	mountLog := filepath.Join(logDir, "mount.log")
	aptLog := filepath.Join(logDir, "apt.log")
	repoContent := filepath.Join(logDir, "repo.txt")

	bin := fakeBinDir(t, map[string]string{
		"mountpoint": "#!/bin/sh\nexit 1\n",
		"mount":      recorderScript(mountLog),
		"mkdir":      "#!/bin/sh\nexit 0\n",
		// tee 把渲染后的源文件内容记下来，供断言"变量已展开"。
		"tee":     "#!/bin/sh\ncat >> " + repoContent + "\n",
		"apt-get": recorderScript(aptLog),
		"sudo":    "#!/bin/sh\nexec \"$@\"\n",
	})

	cfg := writeConfig(t, `
source:
  mode: "iso"
  iso: "/data/ubuntu.iso"
  mount: "/mnt/ubuiso"
resources:
  - name: jq
    method: "package"
`)

	code, out := runEnvIn(t, t.TempDir(),
		[]string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH")},
		"install", "-f", cfg, "--sudo")
	if code != 0 {
		t.Fatalf("iso+apt 退出码 = %d，输出：\n%s", code, out)
	}

	content := readFile(t, repoContent)
	if strings.Contains(content, "$VERSION_CODENAME") || strings.Contains(content, "$MOUNT") || strings.Contains(content, "$COMPONENTS") {
		t.Errorf("apt 源文件里出现未展开的变量：\n%s", content)
	}
	if !strings.Contains(content, "/mnt/ubuiso") {
		t.Errorf("apt 源文件未含挂载路径：\n%s", content)
	}
	if !strings.Contains(content, "main") {
		t.Errorf("apt 源文件未含组件 main：\n%s", content)
	}
}

// TestSource_ISOWithoutPathFails mode: iso 必须给 iso 路径，否则报错而不是去挂载空串。
func TestSource_ISOWithoutPathFails(t *testing.T) {
	bin := fakeBinDir(t, map[string]string{"dnf": "#!/bin/sh\nexit 0\n"})

	cfg := writeConfig(t, `
source:
  mode: "iso"
resources:
  - name: make
    method: "package"
`)

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin}, "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("mode: iso 缺 iso 路径却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "source.iso") {
		t.Errorf("错误信息应指出需要 source.iso，输出：\n%s", out)
	}
}

// TestSource_UnknownModeFails 拼错的 mode 必须报错，不能静默当 official。
func TestSource_UnknownModeFails(t *testing.T) {
	bin := fakeBinDir(t, map[string]string{"apt-get": "#!/bin/sh\nexit 0\n"})

	cfg := writeConfig(t, `
source:
  mode: "mirrors"
resources:
  - name: jq
    method: "package"
`)

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin}, "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("未知 source.mode 却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "official") || !strings.Contains(out, "aliyun") || !strings.Contains(out, "iso") {
		t.Errorf("错误信息应列出三种可用模式，输出：\n%s", out)
	}
}
