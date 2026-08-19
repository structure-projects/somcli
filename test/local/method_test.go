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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// archiveEntry 归档里的一个文件。mode 决定它有没有可执行位 ——
// method: binary 在没有 files: 时正是靠这一位来挑要装什么。
type archiveEntry struct {
	name string
	mode int64
	body string
}

// tarGzServer 起一个本地源，提供一份现造的 tar.gz。
// 在用例里造归档而不是塞一个二进制进仓库：内容、权限、目录层级都要能按格调整。
func tarGzServer(t *testing.T, urlPath string, entries ...archiveEntry) string {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.name,
			Mode:     e.mode,
			Size:     int64(len(e.body)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("写归档头 %s 失败: %v", e.name, err)
		}
		if _, err := tw.Write([]byte(e.body)); err != nil {
			t.Fatalf("写归档内容 %s 失败: %v", e.name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("关闭 tar 失败: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("关闭 gzip 失败: %v", err)
	}

	payload := buf.Bytes()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)
	return srv.URL + urlPath
}

// fakeBinDir 造一个假命令目录：把 name -> 脚本内容 写成可执行文件，并把 sh 链进来。
//
// sh 必须链进来：RunScripts 是用 sh -c 跑每条命令的，而 sh 本身也是从 PATH 找的 ——
// 把 PATH 缩到只剩假命令目录之后，不链 sh 就连命令都起不来，
// 于是用例会因为"没有 sh"而红，看起来像被测行为出了问题。
func fakeBinDir(t *testing.T, cmds map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, script := range cmds {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatalf("写假命令 %s 失败: %v", name, err)
		}
	}
	if err := os.Symlink("/bin/sh", filepath.Join(dir, "sh")); err != nil {
		t.Fatalf("链接 sh 失败: %v", err)
	}
	return dir
}

// recorderScript 造一个把自己的调用参数追加到 logPath 的假命令。
// 断言"包管理器被调用了、而且参数是这些"只能靠它 —— 真去装一个包既慢又会改坏本机。
func recorderScript(logPath string) string {
	return "#!/bin/sh\nprintf '%s\\n' \"$(basename \"$0\") $*\" >> " + logPath + "\n"
}

// TestSC_M01_MethodBinaryBareFile method: binary 把裸下载物装到 install_dir 并置可执行。
//
// E1：method 字段过去零消费 —— 配置里写 method: binary，实际只有 pre/post_install 在跑，
// 退出码还是 0。所以判据不能停在"文件在不在"：post_install 直接**执行**装好的那个文件，
// 权限没置对就会 Permission denied，整条用例红。
func TestSC_M01_MethodBinaryBareFile(t *testing.T) {
	url, _ := fileServerAt(t, "/tool-bin", "#!/bin/sh\necho i-am-installed\n")

	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: tool
    version: "1.0"
    method: "binary"
    install_dir: "{{.WorkDir}}/bin"
    urls:
      - "`+url+`"
    target: "{{.Filename}}"
    post_install:
      - "{{.WorkDir}}/bin/tool-bin > {{.WorkDir}}/ran.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("method: binary 安装退出码 = %d，输出：\n%s", code, out)
	}

	if got := strings.TrimSpace(readFile(t, filepath.Join(workdir, "ran.txt"))); got != "i-am-installed" {
		t.Errorf("装好的二进制执行结果 = %q，期望 %q。输出：\n%s", got, "i-am-installed", out)
	}
	assertExecutable(t, filepath.Join(workdir, "bin", "tool-bin"))
}

// TestSC_M01_MethodBinaryArchive 归档下载物会被解开，只有可执行文件被安装。
//
// 负向断言是重点：归档里的 README 不许出现在 install_dir。
// 否则一个"把解压目录整个拷过去"的实现也能让正向断言全绿，而 PATH 里会多出一堆杂物。
func TestSC_M01_MethodBinaryArchive(t *testing.T) {
	url := tarGzServer(t, "/tool-1.0.tar.gz",
		archiveEntry{"bin/alpha", 0o755, "#!/bin/sh\necho alpha-ran\n"},
		archiveEntry{"bin/beta", 0o755, "#!/bin/sh\necho beta-ran\n"},
		archiveEntry{"README", 0o644, "not an executable\n"},
	)

	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: tool
    version: "1.0"
    method: "binary"
    install_dir: "{{.WorkDir}}/bin"
    urls:
      - "`+url+`"
    target: "{{.Filename}}"
    post_install:
      - "{{.WorkDir}}/bin/alpha >> {{.WorkDir}}/ran.txt"
      - "{{.WorkDir}}/bin/beta >> {{.WorkDir}}/ran.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("归档安装退出码 = %d，输出：\n%s", code, out)
	}

	want := []string{"alpha-ran", "beta-ran"}
	if got := lines(t, filepath.Join(workdir, "ran.txt")); !reflect.DeepEqual(got, want) {
		t.Errorf("归档里的可执行文件未全部装好，执行结果 = %v，期望 %v。输出：\n%s", got, want, out)
	}
	if _, err := os.Stat(filepath.Join(workdir, "bin", "README")); !os.IsNotExist(err) {
		t.Errorf("归档里的非可执行文件也被装进了 install_dir。输出：\n%s", out)
	}
}

// TestSC_M01_MethodBinaryFilesSelects files: 指定装哪几个，其余不许装。
//
// 一个 tar 里带几十个文件是常态（containerd 的发布包就是），
// 全装进 PATH 会污染环境，所以 files: 必须真的起筛选作用，而不是只当注释。
func TestSC_M01_MethodBinaryFilesSelects(t *testing.T) {
	url := tarGzServer(t, "/tool-1.0.tar.gz",
		archiveEntry{"bin/alpha", 0o755, "#!/bin/sh\necho alpha-ran\n"},
		archiveEntry{"bin/beta", 0o755, "#!/bin/sh\necho beta-ran\n"},
	)

	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: tool
    version: "1.0"
    method: "binary"
    install_dir: "{{.WorkDir}}/bin"
    urls:
      - "`+url+`"
    target: "{{.Filename}}"
    files:
      - "bin/beta"
    post_install:
      - "{{.WorkDir}}/bin/beta > {{.WorkDir}}/ran.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("files: 筛选安装退出码 = %d，输出：\n%s", code, out)
	}

	if got := strings.TrimSpace(readFile(t, filepath.Join(workdir, "ran.txt"))); got != "beta-ran" {
		t.Errorf("被点名的文件执行结果 = %q，期望 %q", got, "beta-ran")
	}
	if _, err := os.Stat(filepath.Join(workdir, "bin", "alpha")); !os.IsNotExist(err) {
		t.Errorf("files: 未被点名的 alpha 也装进了 install_dir。输出：\n%s", out)
	}
}

// TestSC_M01_MethodBinaryWithoutURLsFails method: binary 没给 urls 必须报错。
// 没有下载物就没有要装的东西，静默成功等于骗人。
func TestSC_M01_MethodBinaryWithoutURLsFails(t *testing.T) {
	cfg := writeConfig(t, `
resources:
  - name: tool
    version: "1.0"
    method: "binary"
`)

	code, out := run(t, "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("method: binary 缺 urls 却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "urls") {
		t.Errorf("错误信息未指出缺的是 urls，输出：\n%s", out)
	}
	if strings.Contains(out, "[SUCCESS]") {
		t.Errorf("失败路径上打印了 [SUCCESS]，输出：\n%s", out)
	}
}

// TestSC_M02_MethodPackageUsesPackageManager method: package 调用目标机器上的包管理器。
//
// 手法：把 PATH 缩到只剩假命令目录，里面放会记录调用参数的 apt-get 与 yum。
// 断言 apt-get 收到了 install jq、且 yum 一次都没被调用 —— 优先级表要真的是优先级，
// 而不是"谁都试一遍"（那会在 Debian 机器上顺带跑一次 yum）。
//
// 不真装包：装包要 root、会改坏本机、还依赖网络。这里要验的是"命令有没有发出去、发的对不对"。
func TestSC_M02_MethodPackageUsesPackageManager(t *testing.T) {
	logDir := t.TempDir()
	aptLog := filepath.Join(logDir, "apt.log")
	yumLog := filepath.Join(logDir, "yum.log")

	bin := fakeBinDir(t, map[string]string{
		"apt-get": recorderScript(aptLog),
		"yum":     recorderScript(yumLog),
	})

	cfg := writeConfig(t, `
resources:
  - name: jq
    method: "package"
`)

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin}, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("method: package 退出码 = %d，输出：\n%s", code, out)
	}

	got := strings.TrimSpace(readFile(t, aptLog))
	if !strings.Contains(got, "install") || !strings.Contains(got, "jq") {
		t.Errorf("apt-get 收到的参数 = %q，期望包含 install 与包名 jq。输出：\n%s", got, out)
	}
	if _, err := os.Stat(yumLog); !os.IsNotExist(err) {
		t.Errorf("apt-get 已可用却还调用了 yum，优先级没生效。输出：\n%s", out)
	}
}

// TestSC_M02_MethodPackageUsesPackageField package: 覆盖包名，缺省才取 name。
// 命令名与包名常常不同（cri-tools 提供 crictl），没有这个字段就只能改 name，
// 而 name 还要当日志标识与 method: container 的命令名用。
func TestSC_M02_MethodPackageUsesPackageField(t *testing.T) {
	aptLog := filepath.Join(t.TempDir(), "apt.log")
	bin := fakeBinDir(t, map[string]string{"apt-get": recorderScript(aptLog)})

	cfg := writeConfig(t, `
resources:
  - name: crictl
    method: "package"
    package: "cri-tools"
`)

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin}, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("退出码 = %d，输出：\n%s", code, out)
	}

	got := strings.TrimSpace(readFile(t, aptLog))
	if !strings.Contains(got, "cri-tools") {
		t.Errorf("apt-get 收到的参数 = %q，期望用 package: 声明的 cri-tools", got)
	}
	if strings.Contains(got, "crictl") {
		t.Errorf("apt-get 收到的参数 = %q，package: 没有覆盖 name", got)
	}
}

// TestSC_M02_MethodPackageWithoutManagerFails 一个包管理器都没有必须失败。
//
// 这是 E1 病症的另一面：探测不到就当装好了，用户拿到 exit 0 而机器上什么都没有。
// PATH 里只留 sh，连 brew 都摸不到。
func TestSC_M02_MethodPackageWithoutManagerFails(t *testing.T) {
	bin := fakeBinDir(t, nil)

	cfg := writeConfig(t, `
resources:
  - name: jq
    method: "package"
`)

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin}, "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("没有任何包管理器却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "包管理器") {
		t.Errorf("错误信息没说清是找不到包管理器，输出：\n%s", out)
	}
	if strings.Contains(out, "[SUCCESS]") {
		t.Errorf("失败路径上打印了 [SUCCESS]，输出：\n%s", out)
	}
}

// TestSC_M03_MethodContainerPullsAndWraps method: container 拉镜像并留下可直接敲的包装脚本。
//
// image 字段过去也是零消费（E1）：配置里写了镜像，somcli 一次都没用过它。
// 只拉镜像还不够 —— 用户写 method: container 装 helm，期望的是之后能直接敲 helm。
// 所以判据有两条：假 docker 收到了 pull <image>；以及 post_install 里敲包装脚本时，
// 参数被原样传给了容器运行时（"$@" 不能在生成脚本时就被展开掉）。
func TestSC_M03_MethodContainerPullsAndWraps(t *testing.T) {
	dockerLog := filepath.Join(t.TempDir(), "docker.log")
	bin := fakeBinDir(t, map[string]string{"docker": recorderScript(dockerLog)})

	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: helm
    version: "3.12.0"
    method: "container"
    install_dir: "{{.WorkDir}}/bin"
    image: "alpine/helm:{{.Version}}"
    post_install:
      - "{{.WorkDir}}/bin/helm version --short"
`)

	// PATH 前置假 docker，其余真命令（mkdir / chmod / cat）保留
	env := []string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH")}
	code, out := runEnvIn(t, workdir, env, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("method: container 退出码 = %d，输出：\n%s", code, out)
	}

	log := readFile(t, dockerLog)
	if !strings.Contains(log, "docker pull alpine/helm:3.12.0") {
		t.Errorf("docker 调用记录 = %q，期望拉取渲染后的镜像 alpine/helm:3.12.0。输出：\n%s", log, out)
	}
	if !strings.Contains(log, "run --rm -i alpine/helm:3.12.0 version --short") {
		t.Errorf("docker 调用记录 = %q，包装脚本没把参数原样传给运行时。输出：\n%s", log, out)
	}
	assertExecutable(t, filepath.Join(workdir, "bin", "helm"))
}

// TestSC_M03_MethodContainerWithoutImageFails method: container 缺 image 必须报错。
func TestSC_M03_MethodContainerWithoutImageFails(t *testing.T) {
	cfg := writeConfig(t, `
resources:
  - name: helm
    version: "1.0"
    method: "container"
`)

	code, out := run(t, "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("method: container 缺 image 却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "image") {
		t.Errorf("错误信息未指出缺的是 image，输出：\n%s", out)
	}
}

// TestSC_M03_MethodContainerWithoutRuntimeFails 目标机器上没有容器运行时必须失败。
// 没有 docker / podman 却报告"装好了"，用户之后敲 helm 只会得到一句 command not found。
func TestSC_M03_MethodContainerWithoutRuntimeFails(t *testing.T) {
	bin := fakeBinDir(t, nil)

	cfg := writeConfig(t, `
resources:
  - name: helm
    version: "1.0"
    method: "container"
    install_dir: "{{.WorkDir}}/bin"
    image: "alpine/helm:{{.Version}}"
`)

	workdir := t.TempDir()
	code, out := runEnvIn(t, workdir, []string{"PATH=" + bin}, "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("没有容器运行时却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "容器运行时") {
		t.Errorf("错误信息没说清是找不到容器运行时，输出：\n%s", out)
	}
	// 拉镜像就失败了，包装脚本不该留在 install_dir 里冒充"装好了"
	if _, err := os.Stat(filepath.Join(workdir, "bin", "helm")); !os.IsNotExist(err) {
		t.Errorf("拉镜像失败却仍然留下了包装脚本。输出：\n%s", out)
	}
}

// TestSC_M04_MethodSourceBuildsInSrcDir method: source 把源码解到 {{.SrcDir}} 并在那里执行 build:。
//
// 两条判据：build: 里的命令真的跑了（而且按声明顺序），以及执行时的工作目录就是解压出来的
// 源码目录 —— 后者是这个 method 的全部意义，否则用户得在每条 build 命令前自己 cd。
func TestSC_M04_MethodSourceBuildsInSrcDir(t *testing.T) {
	url := tarGzServer(t, "/src-1.0.tar.gz",
		archiveEntry{"build.sh", 0o755, "#!/bin/sh\necho built-here >> \"$1/build.txt\"\npwd >> \"$1/build.txt\"\n"},
	)

	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: fromsource
    version: "1.0"
    method: "source"
    urls:
      - "`+url+`"
    target: "{{.Filename}}"
    build:
      - "sh build.sh {{.WorkDir}}"
      - "echo second-step >> {{.WorkDir}}/build.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("method: source 退出码 = %d，输出：\n%s", code, out)
	}

	got := lines(t, filepath.Join(workdir, "build.txt"))
	if len(got) != 3 {
		t.Fatalf("build 产物 = %v，期望三行（标记 / 工作目录 / 第二步）。输出：\n%s", got, out)
	}
	if got[0] != "built-here" || got[2] != "second-step" {
		t.Errorf("build 步骤未按声明顺序执行，产物 = %v", got)
	}

	// build 的工作目录必须是源码目录，而不是 somcli 自己的当前目录
	wantSrc := filepath.Join(workdir, "download", "fromsource", "1.0", "src")
	if !strings.HasSuffix(got[1], "src") {
		t.Errorf("build 的工作目录 = %q，期望源码目录 %q", got[1], wantSrc)
	}
	if _, err := os.Stat(filepath.Join(wantSrc, "build.sh")); err != nil {
		t.Errorf("源码没有解到 %s: %v。输出：\n%s", wantSrc, err, out)
	}
}

// TestSC_M04_MethodSourceWithoutBuildFails method: source 不写 build: 必须报错。
//
// 有意不猜构建方式（不去试 ./configure / make / cargo）：猜错的代价是
// "报告装好了、其实什么都没编出来"，而 build: 写两行就说清了。
func TestSC_M04_MethodSourceWithoutBuildFails(t *testing.T) {
	url := tarGzServer(t, "/src-1.0.tar.gz",
		archiveEntry{"build.sh", 0o755, "#!/bin/sh\ntrue\n"},
	)

	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: fromsource
    version: "1.0"
    method: "source"
    urls:
      - "`+url+`"
    target: "{{.Filename}}"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("method: source 缺 build 却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "build") {
		t.Errorf("错误信息未指出缺的是 build，输出：\n%s", out)
	}
}

// TestSC_M04_MethodSourceNonArchiveFails 下载物不是归档必须报错，而不是当没事发生。
func TestSC_M04_MethodSourceNonArchiveFails(t *testing.T) {
	url, _ := fileServerAt(t, "/plain.txt", "not an archive\n")

	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: fromsource
    version: "1.0"
    method: "source"
    urls:
      - "`+url+`"
    target: "{{.Filename}}"
    build:
      - "true"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("源码不是归档却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "归档") {
		t.Errorf("错误信息没说清问题在归档格式，输出：\n%s", out)
	}
}

// TestSC_M05_MethodScriptIsDefault method: script 与不写 method 行为一致：动作就是脚本本身。
//
// 这条守住的是兼容性：仓库里与用户手上的既有配置都没写 method，
// 分发逻辑一上来就不能改变它们的行为。
func TestSC_M05_MethodScriptIsDefault(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
	}{
		{"SC-M05/显式 script", "    method: \"script\"\n"},
		{"SC-M05/不写 method", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workdir := t.TempDir()
			cfg := writeConfig(t, `
resources:
  - name: scripted
    version: "1.0"
`+tc.method+`    pre_install:
      - "echo pre >> {{.WorkDir}}/order.txt"
    post_install:
      - "echo post >> {{.WorkDir}}/order.txt"
`)

			code, out := runIn(t, workdir, "install", "-f", cfg)
			if code != 0 {
				t.Fatalf("退出码 = %d，输出：\n%s", code, out)
			}

			want := []string{"pre", "post"}
			if got := lines(t, filepath.Join(workdir, "order.txt")); !reflect.DeepEqual(got, want) {
				t.Errorf("脚本执行结果 = %v，期望 %v。输出：\n%s", got, want, out)
			}
		})
	}
}

// TestSC_M05_UnknownMethodFails 认不出来的 method 必须报错，并列出可用值。
//
// 这正是 E1 的病症本身：method 字段过去谁都不读，写 method: binry（拼错）
// 或 method: helm（想当然）都会静默当脚本跑完，退出码 0，而二进制根本没装上。
func TestSC_M05_UnknownMethodFails(t *testing.T) {
	for _, method := range []string{"binry", "helm"} {
		t.Run("SC-M05/"+method, func(t *testing.T) {
			workdir := t.TempDir()
			cfg := writeConfig(t, `
resources:
  - name: typo
    version: "1.0"
    method: "`+method+`"
    post_install:
      - "echo ran > {{.WorkDir}}/ran.txt"
`)

			code, out := runIn(t, workdir, "install", "-f", cfg)
			if code == 0 {
				t.Fatalf("未知 method %q 却退出码 0，输出：\n%s", method, out)
			}
			if !strings.Contains(out, method) {
				t.Errorf("错误信息未回显写错的 method，输出：\n%s", out)
			}
			if !strings.Contains(out, "binary") || !strings.Contains(out, "container") {
				t.Errorf("错误信息未列出可用的 method，用户无从纠正，输出：\n%s", out)
			}
			// 已经认定配置有错，就不该再往下跑 post_install
			if _, err := os.Stat(filepath.Join(workdir, "ran.txt")); !os.IsNotExist(err) {
				t.Errorf("method 无效却仍然执行了 post_install。输出：\n%s", out)
			}
		})
	}
}

// TestSC_M06_MethodManifestNotImplemented method: manifest 还没实现，必须明说。
//
// 与未知 method 分开报：manifest 是路线图上的合法取值（随集群编排落地），
// 报"未知的 method"会让用户以为自己拼错了，反复去查文档。
func TestSC_M06_MethodManifestNotImplemented(t *testing.T) {
	cfg := writeConfig(t, `
resources:
  - name: app
    version: "1.0"
    method: "manifest"
`)

	code, out := run(t, "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("method: manifest 却退出码 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "manifest") || !strings.Contains(out, "未实现") {
		t.Errorf("错误信息没说清 manifest 是未实现而非拼错，输出：\n%s", out)
	}
}

// assertExecutable 断言路径上是个带可执行位的文件。
func assertExecutable(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("产物 %s 不存在: %v", path, err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("产物 %s 权限 = %v，没有可执行位", path, info.Mode().Perm())
	}
}
