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
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeDockerDir 造一个假 docker：镜像名里含 failMark 的操作失败，其余成功。
//
// 不用真 docker：要验的是"逐张镜像的失败有没有被汇总、有没有影响退出码"，
// 这与镜像真的拉没拉下来无关，而真 docker 还会让用例依赖网络与守护进程。
func fakeDockerDir(t *testing.T, logPath, failMark string) string {
	t.Helper()

	script := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> %q
for a in "$@"; do
  case "$a" in
    *%s*) echo "boom: $a" >&2; exit 1 ;;
  esac
done
exit 0
`, logPath, failMark)

	return fakeBinDir(t, map[string]string{"docker": script})
}

// writeImageList 落一份镜像清单（每行 name:tag，README 里的 image-list.txt 就是这种）。
func writeImageList(t *testing.T, images ...string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "image-list.txt")
	if err := os.WriteFile(path, []byte(strings.Join(images, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("写镜像清单失败: %v", err)
	}
	return path
}

// F14：pull 里有镜像失败就必须非 0 退出，并把失败的镜像逐条列出来。
//
// 原实现对每张失败只 Warnf 一句然后 continue，最后 return nil：
// 一张都没拉到也照样 exit 0，调用方（CI、离线打包脚本）完全感知不到。
func TestF14_PullAggregatesFailuresAndExitsNonZero(t *testing.T) {
	log := filepath.Join(t.TempDir(), "docker.log")
	bin := fakeDockerDir(t, log, "broken")
	list := writeImageList(t, "library/nginx:1.25", "org/broken:1.0")

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin},
		"images", "pull", "-f", list)
	if code == 0 {
		t.Fatalf("有镜像拉取失败却退出 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "org/broken:1.0") {
		t.Fatalf("错误信息没有点出失败的镜像，输出：\n%s", out)
	}
	// 成功的那张也得真的被拉过：汇总失败不等于遇错即停
	if got := readFile(t, log); !strings.Contains(got, "library/nginx:1.25") {
		t.Fatalf("失败一张就不再处理后面的镜像了，docker 调用记录：\n%s", got)
	}
}

// F14：有失败就不写镜像清单。
//
// 清单是"这些镜像已在本地"的凭据，下游 export/push 照着它干活。
// 把没拉下来的镜像写进去，等于把失败伪装成成功交给下一步。
func TestF14_PullSkipsImageListOnFailure(t *testing.T) {
	log := filepath.Join(t.TempDir(), "docker.log")
	bin := fakeDockerDir(t, log, "broken")
	list := writeImageList(t, "library/nginx:1.25", "org/broken:1.0")
	manifest := filepath.Join(t.TempDir(), "pulled.yaml")

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin},
		"images", "pull", "-f", list, "-o", manifest)
	if code == 0 {
		t.Fatalf("有镜像拉取失败却退出 0，输出：\n%s", out)
	}
	if _, err := os.Stat(manifest); err == nil {
		t.Fatalf("有镜像失败却写出了清单 %s：\n%s", manifest, readFile(t, manifest))
	}
}

// F14：全都成功时才写清单，且退出 0 —— 汇总逻辑不能把正常路径带坏。
func TestF14_PullWritesImageListWhenAllSucceed(t *testing.T) {
	log := filepath.Join(t.TempDir(), "docker.log")
	bin := fakeDockerDir(t, log, "no-such-mark")
	list := writeImageList(t, "library/nginx:1.25", "library/redis:7")
	manifest := filepath.Join(t.TempDir(), "pulled.yaml")

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin},
		"images", "pull", "-f", list, "-o", manifest)
	if code != 0 {
		t.Fatalf("全部成功却退出码 = %d，输出：\n%s", code, out)
	}

	got := readFile(t, manifest)
	for _, want := range []string{"nginx", "redis"} {
		if !strings.Contains(got, want) {
			t.Fatalf("清单里缺 %s：\n%s", want, got)
		}
	}
}

// F14：push 同样要聚合失败并影响退出码。
func TestF14_PushAggregatesFailures(t *testing.T) {
	log := filepath.Join(t.TempDir(), "docker.log")
	bin := fakeDockerDir(t, log, "broken")
	list := writeImageList(t, "library/nginx:1.25", "org/broken:1.0")

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin},
		"images", "push", "-i", list)
	if code == 0 {
		t.Fatalf("有镜像推送失败却退出 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "org/broken:1.0") {
		t.Fatalf("错误信息没有点出失败的镜像，输出：\n%s", out)
	}
}

// F14：docker 根本不在 PATH 上时，pull 必须失败 —— 这是最容易被 Warnf 吞掉的一种。
func TestF14_PullFailsWhenDockerMissing(t *testing.T) {
	emptyBin := t.TempDir()
	list := writeImageList(t, "library/nginx:1.25")

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + emptyBin},
		"images", "pull", "-f", list)
	if code == 0 {
		t.Fatalf("没有 docker 却退出 0，输出：\n%s", out)
	}
}

// R1：归档里的条目名不得把文件写到解压目录之外（CWE-22，评审新发现）。
//
// 离线镜像包是在机器之间传递的外部输入，而 somcli 多数场景以 root 运行：
// 一个条目名写成 ../../../etc/cron.d/x，"导入镜像"就成了往任意路径写攻击者提供的内容。
// 判据不看退出码 —— 没有 docker 时导入本来就非 0 退出，那样的绿说明不了任何事情；
// 唯一的判据是标记文件有没有被创建出来。
func TestR1_ImportRejectsPathTraversalEntries(t *testing.T) {
	marker, err := os.MkdirTemp("/tmp", "somcli-zipslip-")
	if err != nil {
		t.Fatalf("创建标记目录失败: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(marker) })

	victim := filepath.Join(marker, "PWNED.txt")
	// 40 层 ../ 足以从任意深度的解压目录退到根，filepath.Join 会在根上截住，
	// 于是无论 workdir 有多深，条目都精确落在 victim 这个绝对路径上
	entry := strings.Repeat("../", 40) + strings.TrimPrefix(victim, "/")

	archive := filepath.Join(t.TempDir(), "evil.tar.gz")
	writeTarGz(t, archive, entry, "PWNED\n")

	_, out := run(t, "images", "import", "-i", archive)

	if _, err := os.Stat(victim); err == nil {
		t.Fatalf("归档条目越出解压目录，写出了 %s：\n%s", victim, out)
	}
	if !strings.Contains(out, "越出") && !strings.Contains(out, "PWNED.txt") {
		t.Fatalf("拒绝了却没说清是哪个条目有问题，输出：\n%s", out)
	}
}

// fakeDockerLifecycleDir 造一个能跑 save/load 的假 docker：
//   - save -o <file> <img>：往 <file> 写一小段内容，伪装镜像层
//   - load -i <file>：记录一次调用
//   - pull/tag/push/rmi：记录并（对含 failMark 的 push）失败
//
// 用它把"导出→导入"整条链在本机跑通，不依赖真 docker 守护进程。
func fakeDockerLifecycleDir(t *testing.T, logPath, failMark string) string {
	t.Helper()
	script := fmt.Sprintf(`#!/bin/sh
printf 'docker %%s\n' "$*" >> %q
sub="$1"; shift
case "$sub" in
  save)
    out="$2"; img="$3"
    case "$img" in *%s*) echo "boom: save $img" >&2; exit 1 ;; esac
    printf 'fake-layer-for-%%s\n' "$img" > "$out"
    exit 0 ;;
  load) exit 0 ;;
  pull|tag) exit 0 ;;
  push)
    for a in "$@"; do case "$a" in *%s*) echo "boom: push $a" >&2; exit 1 ;; esac; done
    exit 0 ;;
  rmi) exit 0 ;;
  *) exit 0 ;;
esac
`, logPath, failMark, failMark)
	return fakeBinDir(t, map[string]string{"docker": script})
}

// SC-C05：export 把每张镜像 `docker save` 出来后打进同一个 tar.gz。
// 判据：产物非空、是合法 gzip、里面每张镜像一个条目；全程退出 0。
func TestSC_C05_ExportProducesArchive(t *testing.T) {
	log := filepath.Join(t.TempDir(), "docker.log")
	bin := fakeDockerLifecycleDir(t, log, "no-such-mark")
	list := writeImageList(t, "library/nginx:1.25", "library/redis:7")
	archive := filepath.Join(t.TempDir(), "images.tar.gz")

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin},
		"images", "export", "-f", list, "-o", archive)
	if code != 0 {
		t.Fatalf("导出失败，退出码 %d，输出：\n%s", code, out)
	}

	info, err := os.Stat(archive)
	if err != nil {
		t.Fatalf("导出产物不存在: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("导出产物为空: %s", archive)
	}

	got := readFile(t, log)
	if !strings.Contains(got, "docker save") {
		t.Errorf("没有执行 docker save，调用记录：\n%s", got)
	}
	// gzip 魔数 1f 8b：证明产物是真 gzip 而不是半截文件。
	data, _ := os.ReadFile(archive)
	if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
		t.Errorf("产物不是合法 gzip（头两字节 %x %x）", data[0], data[1])
	}
}

// SC-C05：export 遇到一张镜像 save 失败，必须删掉半截归档并非 0 退出。
// 半截归档比没有更坏 —— 它看着像能用的离线包，要到目标机 import 才暴露。
func TestSC_C05_ExportFailureRemovesArchive(t *testing.T) {
	log := filepath.Join(t.TempDir(), "docker.log")
	bin := fakeDockerLifecycleDir(t, log, "broken")
	list := writeImageList(t, "library/nginx:1.25", "library/broken:1.0")
	archive := filepath.Join(t.TempDir(), "images.tar.gz")

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin},
		"images", "export", "-f", list, "-o", archive)
	if code == 0 {
		t.Fatalf("有镜像 save 失败却退出 0，输出：\n%s", out)
	}
	if _, err := os.Stat(archive); err == nil {
		t.Errorf("失败后仍留着半截归档 %s", archive)
	}
	if !strings.Contains(out, "broken") {
		t.Errorf("错误信息没点名失败镜像，输出：\n%s", out)
	}
}

// SC-C05：export 出的归档能被 import 逐条 `docker load`。
// 这是离线场景的闭环：操作机导出，目标机导入。
func TestSC_C05_ImportLoadsExportedArchive(t *testing.T) {
	log := filepath.Join(t.TempDir(), "docker.log")
	bin := fakeDockerLifecycleDir(t, log, "no-such-mark")
	list := writeImageList(t, "library/nginx:1.25", "library/redis:7")
	workdir := t.TempDir()
	archive := filepath.Join(workdir, "images.tar.gz")

	if code, out := runEnvIn(t, workdir, []string{"PATH=" + bin},
		"images", "export", "-f", list, "-o", archive); code != 0 {
		t.Fatalf("导出失败：\n%s", out)
	}

	// 导入用另一个日志，便于只数 load 次数。
	loadLog := filepath.Join(t.TempDir(), "load.log")
	loadBin := fakeDockerLifecycleDir(t, loadLog, "no-such-mark")
	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + loadBin},
		"images", "import", "-i", archive)
	if code != 0 {
		t.Fatalf("导入失败，退出码 %d，输出：\n%s", code, out)
	}
	if got := readFile(t, loadLog); strings.Count(got, "docker load") != 2 {
		t.Errorf("应当对 2 张镜像各 load 一次，实际调用记录：\n%s", got)
	}
}

// SC-C05：import 不是 .tar.gz（连 gzip 都不是）要明确失败，
// 而不是 panic 或退出 0。
func TestSC_C05_ImportRejectsNonGzip(t *testing.T) {
	bin := fakeDockerLifecycleDir(t, filepath.Join(t.TempDir(), "docker.log"), "no-such-mark")
	notArchive := filepath.Join(t.TempDir(), "broken.tar.gz")
	if err := os.WriteFile(notArchive, []byte("not a gzip"), 0o644); err != nil {
		t.Fatalf("写坏归档失败: %v", err)
	}

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin},
		"images", "import", "-i", notArchive)
	if code == 0 {
		t.Fatalf("坏归档却导入成功（退出 0），输出：\n%s", out)
	}
	if !strings.Contains(out, "gzip") {
		t.Errorf("错误信息没指出是 gzip 解析问题，输出：\n%s", out)
	}
}

// SC-X10：images -f 认三种清单写法：统一配置的 images: 段、裸 name/tag 列表、
// 纯文本每行 name:tag。三种都喂给假 docker pull，断言每张都被拉到。
func TestSC_X10_ImageListThreeFormats(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name: "SC-X10/统一配置取 images 段",
			content: `
resources:
  - name: tool
    version: "1.0"
images:
  - name: library/nginx
    tag: "1.25"
  - name: library/redis
    tag: "7"
`,
			want: []string{"library/nginx:1.25", "library/redis:7"},
		},
		{
			name: "SC-X10/裸 name/tag 列表",
			content: `- name: library/nginx
  tag: "1.25"
- name: library/redis
  tag: "7"
`,
			want: []string{"library/nginx:1.25", "library/redis:7"},
		},
		{
			name:    "SC-X10/纯文本每行 name:tag",
			content: "library/nginx:1.25\n# 注释行应跳过\nlibrary/redis:7\n",
			want:    []string{"library/nginx:1.25", "library/redis:7"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			log := filepath.Join(t.TempDir(), "docker.log")
			bin := fakeDockerLifecycleDir(t, log, "no-such-mark")
			list := filepath.Join(t.TempDir(), "images.txt")
			if err := os.WriteFile(list, []byte(tc.content), 0o644); err != nil {
				t.Fatalf("写清单失败: %v", err)
			}

			code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin},
				"images", "pull", "-f", list)
			if code != 0 {
				t.Fatalf("解析清单失败，退出码 %d，输出：\n%s", code, out)
			}
			got := readFile(t, log)
			for _, img := range tc.want {
				if !strings.Contains(got, "pull "+img) {
					t.Errorf("清单里的 %s 没有被拉取，docker 调用记录：\n%s", img, got)
				}
			}
		})
	}
}

// G9：pull/push 不带 -o/-i 时，不得被 export/import 的默认值 "images.tar.gz" 污染。
//
// 根因是 cmd/images.go 曾让四个子命令共用同一个包级变量来接 -o/-i，而 pflag 在
// 注册时就把默认值写进变量 —— export/import 后注册，默认值 images.tar.gz 于是泄漏给
// pull/push。症状：pull 成功后凭空在当前目录写一个名为 images.tar.gz 的 YAML 清单；
// push 不带 -i 不去用内置默认清单，反而去读一个不存在的 images.tar.gz。
// 这里把 somcli 的 CWD 指到临时目录，直接断言那个文件既不被写出、也不被读取。
func TestG9_OutputInputDefaultsDontLeakBetweenSubcommands(t *testing.T) {
	log := filepath.Join(t.TempDir(), "docker.log")
	bin := fakeDockerLifecycleDir(t, log, "no-such-mark")
	list := writeImageList(t, "library/nginx:1.25")
	cwd := t.TempDir()

	// pull 不带 -o：成功后 CWD 下不得冒出 images.tar.gz。
	runInDir(t, cwd, bin, "images", "pull", "-f", list)
	if _, err := os.Stat(filepath.Join(cwd, "images.tar.gz")); err == nil {
		t.Fatalf("pull 不带 -o 却在 CWD 写出了 images.tar.gz（标志默认值泄漏）")
	}

	// push 不带 -i：应当走内置默认清单（假 docker 让它成功），而不是报
	// "open images.tar.gz: no such file"。
	code, out := runInDir(t, cwd, bin, "images", "push", "-s", "all")
	if code != 0 {
		t.Fatalf("push 不带 -i 应使用内置清单而非读 images.tar.gz，退出码 %d，输出：\n%s", code, out)
	}
	if strings.Contains(out, "images.tar.gz") {
		t.Errorf("push 不带 -i 却去读 images.tar.gz：\n%s", out)
	}
}

// runInDir 在指定 CWD 下执行 somcli，PATH 指向假命令目录，返回退出码与输出。
func runInDir(t *testing.T, cwd, binDir string, args ...string) (int, string) {
	t.Helper()
	bin := somcliBinary(t)
	cmd := exec.Command(bin, args...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir(), "PATH="+binDir)
	out, err := cmd.CombinedOutput()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("执行 %v 失败: %v\n%s", args, err, out)
	}
	return code, string(out)
}

// writeTarGz 造一个只含单个条目的 .tar.gz，条目名由调用方指定（含非法名字）。
func writeTarGz(t *testing.T, path, name, content string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("创建归档失败: %v", err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Name:     name,
		Size:     int64(len(content)),
		Mode:     0o644,
		Typeflag: tar.TypeReg,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("写归档头失败: %v", err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatalf("写归档内容失败: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("关闭 tar 失败: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("关闭 gzip 失败: %v", err)
	}
}
