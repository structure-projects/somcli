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
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// requireDownloader 下载器是 shell out 到 wget 实现的，既没有 curl 回退也没走 net/http。
// macOS 默认不带 wget，这里显式跳过并说明原因 —— 与其让用例在开发机上假绿，
// 不如把这个依赖摊开：它是 M1「下载器修正」要处理的事。
func requireDownloader(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("wget"); err != nil {
		t.Skip("下载器依赖 wget（无 curl 回退、未用 net/http），本机没有 wget，跳过；M1 修正下载器后本条恢复")
	}
}

// fileServer 起一个本地 HTTP 源，返回 URL 与内容的 sha256。
// 用本地服务而不是真实 URL：用例不能依赖外网，也不该给上游制造流量。
func fileServer(t *testing.T, body string) (url, sum string) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	raw := sha256.Sum256([]byte(body))
	return srv.URL + "/payload.sh", hex.EncodeToString(raw[:])
}

// TestSC_D01_DownloadWithChecksum 下载 + 校验和通过 → 产物落盘且内容一致。
//
// 断言必须落到文件内容上：只看退出码的话，一个根本没写盘的实现也能通过，
// 而"以为下载好了、实际目录是空的"是离线安装最常见的坑。
func TestSC_D01_DownloadWithChecksum(t *testing.T) {
	requireDownloader(t)

	body := "#!/bin/sh\necho hello-from-source\n"
	url, sum := fileServer(t, body)

	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: payload
    version: "1.0"
    urls:
      - "`+url+`"
    target: "{{.Filename}}"
    checksum: "sha256:`+sum+`"
`)

	code, out := runIn(t, workdir, "download", "-f", cfg)
	if code != 0 {
		t.Fatalf("下载退出码 = %d，输出：\n%s", code, out)
	}

	got := readFile(t, filepath.Join(workdir, "download", "payload", "1.0", "payload.sh"))
	if got != body {
		t.Errorf("产物内容与源不一致：\n实际 %q\n期望 %q", got, body)
	}
}

// TestSC_D01_ChecksumMismatchFails 校验和不匹配必须失败，且不留下坏文件。
// 留着一个校验失败的产物比没有更危险：下一次运行可能直接拿它去装。
func TestSC_D01_ChecksumMismatchFails(t *testing.T) {
	requireDownloader(t)

	url, _ := fileServer(t, "real-content")
	wrong := strings.Repeat("0", 64)

	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: payload
    version: "1.0"
    urls:
      - "`+url+`"
    target: "{{.Filename}}"
    checksum: "sha256:`+wrong+`"
`)

	code, out := runIn(t, workdir, "download", "-f", cfg)
	if code == 0 {
		t.Fatalf("校验和不匹配却退出码 0，输出：\n%s", out)
	}
	if strings.Contains(out, "[SUCCESS]") {
		t.Errorf("失败路径上打印了 [SUCCESS]，输出：\n%s", out)
	}
	bad := filepath.Join(workdir, "download", "payload", "1.0", "payload.sh")
	if _, err := os.Stat(bad); !os.IsNotExist(err) {
		t.Errorf("校验失败的产物没有被清理，残留在 %s", bad)
	}
}

// TestSC_D03_SourceErrorFailsLoudly 源返回 500 → 非 0 退出且不打印成功。
//
// F4/G8 的回归：PrintSuccess 曾经在错误分支之外，DownloadResources 恒返回 nil，
// 于是"下载全失败"的运行也是 exit 0 + 满屏 ✓ 成功。
//
// 这条用例慢（约 20 秒）：下载器对任何失败都重试 5 次、每次间隔 5 秒，不区分 5xx。
// 这是工具当前的真实行为，M1 重写下载器时会一并改成对 4xx/5xx 快速失败。
func TestSC_D03_SourceErrorFailsLoudly(t *testing.T) {
	requireDownloader(t)
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: payload
    version: "1.0"
    urls:
      - "`+srv.URL+`/payload.sh"
    target: "{{.Filename}}"
`)

	code, out := runIn(t, workdir, "download", "-f", cfg)
	if code == 0 {
		t.Fatalf("源返回 500 却退出码 0，输出：\n%s", out)
	}
	if strings.Contains(out, "[SUCCESS]") {
		t.Errorf("下载失败却打印了 [SUCCESS]，输出：\n%s", out)
	}
	if !strings.Contains(out, "payload") {
		t.Errorf("输出未指明失败的资源，输出：\n%s", out)
	}
}

// TestSC_D04_TargetPathSemantics target 决定产物落在哪儿。
//
// 两种写法各有语义，用户配置里都在用：
//   - 相对路径：相对该资源的缓存目录 <workdir>/download/<name>/<version>/
//   - 绝对路径：就落在那个绝对路径上（示例里普遍写成 {{.WorkDir}}/scripts/...）
//
// 顺带钉住 --workdir 对下载产物的作用：换一个 workdir，产物就该跟着走。
func TestSC_D04_TargetPathSemantics(t *testing.T) {
	requireDownloader(t)

	body := "payload-body\n"
	url, _ := fileServer(t, body)

	t.Run("SC-D04/相对 target 落在缓存目录下", func(t *testing.T) {
		workdir := t.TempDir()
		cfg := writeConfig(t, `
resources:
  - name: payload
    version: "2.0"
    urls:
      - "`+url+`"
    target: "renamed.sh"
`)

		code, out := runIn(t, workdir, "download", "-f", cfg)
		if code != 0 {
			t.Fatalf("下载退出码 = %d，输出：\n%s", code, out)
		}
		if got := readFile(t, filepath.Join(workdir, "download", "payload", "2.0", "renamed.sh")); got != body {
			t.Errorf("相对 target 的产物内容 = %q，期望 %q", got, body)
		}
	})

	t.Run("SC-D04/绝对 target 落在指定位置", func(t *testing.T) {
		workdir := t.TempDir()
		cfg := writeConfig(t, `
resources:
  - name: payload
    version: "2.0"
    urls:
      - "`+url+`"
    target: "{{.WorkDir}}/scripts/{{.Filename}}"
`)

		code, out := runIn(t, workdir, "download", "-f", cfg)
		if code != 0 {
			t.Fatalf("下载退出码 = %d，输出：\n%s", code, out)
		}
		if got := readFile(t, filepath.Join(workdir, "scripts", "payload.sh")); got != body {
			t.Errorf("绝对 target 的产物内容 = %q，期望 %q", got, body)
		}
	})
}
