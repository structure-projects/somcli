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
	"path/filepath"
	"strings"
	"testing"
)

// fileServer 起一个本地 HTTP 源，返回 URL 与内容的 sha256。
// 用本地服务而不是真实 URL：用例不能依赖外网，也不该给上游制造流量。
func fileServer(t *testing.T, body string) (url, sum string) {
	t.Helper()
	return fileServerAt(t, "/payload.sh", body)
}

// fileServerAt 同 fileServer，但由调用方指定路径（用于钉住 {{.Filename}} / {{.Ext}}）。
func fileServerAt(t *testing.T, path, body string) (url, sum string) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	raw := sha256.Sum256([]byte(body))
	return srv.URL + path, hex.EncodeToString(raw[:])
}

// TestSC_D01_DownloadWithChecksum 下载 + 校验和通过 → 产物落盘且内容一致。
//
// 断言必须落到文件内容上：只看退出码的话，一个根本没写盘的实现也能通过，
// 而"以为下载好了、实际目录是空的"是离线安装最常见的坑。
func TestSC_D01_DownloadWithChecksum(t *testing.T) {
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

// TestSC_D02_ChecksumMismatchFails 校验和不匹配必须失败，且不留下坏文件。
// 留着一个校验失败的产物比没有更危险：下一次运行可能直接拿它去装。
//
// 绝对 target 那一半是 F3 的回归：要删的残留路径过去被算成
// <cacheDir>/<绝对路径>，于是 os.Remove 删了个不存在的路径而真产物原地留着 ——
// 报错了、退出码也对，坏文件却还在缓存里等着被下一次运行拿去用。
func TestSC_D02_ChecksumMismatchFails(t *testing.T) {
	url, _ := fileServer(t, "real-content")
	wrong := strings.Repeat("0", 64)

	t.Run("SC-D02/相对 target", func(t *testing.T) {
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
		assertAbsent(t, filepath.Join(workdir, "download", "payload", "1.0", "payload.sh"), out)
	})

	t.Run("SC-D02/绝对 target", func(t *testing.T) {
		workdir := t.TempDir()
		cfg := writeConfig(t, `
resources:
  - name: payload
    version: "1.0"
    urls:
      - "`+url+`"
    target: "{{.WorkDir}}/scripts/{{.Filename}}"
    checksum: "sha256:`+wrong+`"
`)

		code, out := runIn(t, workdir, "download", "-f", cfg)
		if code == 0 {
			t.Fatalf("校验和不匹配却退出码 0，输出：\n%s", out)
		}
		assertAbsent(t, filepath.Join(workdir, "scripts", "payload.sh"), out)
	})
}

// assertAbsent 断言路径上没有文件，失败时把命令输出一并带上便于定位。
func assertAbsent(t *testing.T, path, out string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("校验失败的产物没有被清理，残留在 %s。输出：\n%s", path, out)
	}
}

// TestSC_D03_SourceErrorFailsLoudly 源返回 500 → 重试耗尽后非 0 退出且不打印成功。
//
// F4/G8 的回归：PrintSuccess 曾经在错误分支之外，DownloadResources 恒返回 nil，
// 于是"下载全失败"的运行也是 exit 0 + 满屏 ✓ 成功。
//
// 这条用例慢（约 20 秒），慢在下载器对 5xx 重试 5 次、每次间隔 5 秒 —— 这是有意的：
// 5xx 可能是瞬时故障。4xx 则不重试，见 SC-D03/地址写错立即失败。
func TestSC_D03_SourceErrorFailsLoudly(t *testing.T) {
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

// TestSC_D03_NotFoundFailsFast 404 不重试。
//
// 地址写错不是瞬时故障，重试 5 次只是把"你的 URL 是错的"推迟 20 秒才说。
// 判据取输出里没有重试提示，而不是掐表 —— 计时断言在 CI 上不稳。
func TestSC_D03_NotFoundFailsFast(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: payload
    version: "1.0"
    urls:
      - "`+srv.URL+`/nope.sh"
    target: "{{.Filename}}"
`)

	code, out := runIn(t, workdir, "download", "-f", cfg)
	if code == 0 {
		t.Fatalf("源返回 404 却退出码 0，输出：\n%s", out)
	}
	if strings.Contains(out, "秒后重试") {
		t.Errorf("404 走了重试，输出：\n%s", out)
	}
	if !strings.Contains(out, "404") {
		t.Errorf("错误信息未带上 HTTP 状态，输出：\n%s", out)
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

	t.Run("SC-D04/相对 target 可带子目录", func(t *testing.T) {
		workdir := t.TempDir()
		cfg := writeConfig(t, `
resources:
  - name: payload
    version: "2.0"
    urls:
      - "`+url+`"
    target: "bin/nested/tool.sh"
`)

		code, out := runIn(t, workdir, "download", "-f", cfg)
		if code != 0 {
			t.Fatalf("带子目录的 target 退出码 = %d，输出：\n%s", code, out)
		}
		if got := readFile(t, filepath.Join(workdir, "download", "payload", "2.0", "bin", "nested", "tool.sh")); got != body {
			t.Errorf("子目录 target 的产物内容 = %q，期望 %q", got, body)
		}
	})
}

// TestSC_D05_AbsoluteTargetLocalPath 绝对 target 时报出的产物路径必须是真路径。
//
// F3 的正向回归：LocalPath 曾经无条件 filepath.Join(cacheDir, target)，绝对 target 下
// 得到 <cacheDir>/<绝对路径> 这种不存在的路径。文件本身写对了，所以只看"产物在不在"
// 是发现不了的（SC-D04 那条在缺陷版本上照样绿）—— 必须断言工具**自己报出来的**那个路径。
// 它错了，后续拿 LocalPath 去分发、去校验、去删残留的三条路径就全错。
func TestSC_D05_AbsoluteTargetLocalPath(t *testing.T) {
	body := "abs-target-body\n"
	url, sum := fileServer(t, body)

	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: payload
    version: "3.0"
    urls:
      - "`+url+`"
    target: "{{.WorkDir}}/scripts/{{.Filename}}"
    checksum: "sha256:`+sum+`"
`)

	code, out := runIn(t, workdir, "download", "-f", cfg)
	if code != 0 {
		t.Fatalf("绝对 target + checksum 退出码 = %d，输出：\n%s", code, out)
	}

	// checksum 通过本身就说明校验读到了真文件：路径算错的话 CalculateLocalHash 打不开文件。
	want := filepath.Join(workdir, "scripts", "payload.sh")
	if got := readFile(t, want); got != body {
		t.Fatalf("产物内容 = %q，期望 %q", got, body)
	}
	if !strings.Contains(out, want) {
		t.Errorf("输出里报的产物路径不是 %s，输出：\n%s", want, out)
	}
	if strings.Contains(out, filepath.Join(workdir, "download", "payload", "3.0", workdir)) {
		t.Errorf("产物路径把绝对 target 拼到了缓存目录后面，输出：\n%s", out)
	}
}

// TestSC_D06_FilenameAndExtVars {{.Filename}} 与 {{.Ext}} 在 target 与脚本两处都可用。
//
// E5 的回归：两个模板上下文过去各写一份结构体，Filename/Ext 只有 target 那份有。
// 在 post_install 里写 {{.Filename}} 会直接报 can't evaluate field —— 而"拿文件名去
// chmod / mv / tar"恰恰是脚本里最常见的写法。
func TestSC_D06_FilenameAndExtVars(t *testing.T) {
	body := "archive-body\n"
	url, _ := fileServerAt(t, "/tool-1.2.3.tar.gz", body)

	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: payload
    version: "1.2.3"
    urls:
      - "`+url+`"
    target: "{{.Name}}{{.Ext}}"
    post_install:
      - "echo filename={{.Filename}} >> {{.WorkDir}}/vars.txt"
      - "echo ext={{.Ext}} >> {{.WorkDir}}/vars.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("退出码 = %d，输出：\n%s", code, out)
	}

	// target 用 {{.Ext}} 拼出来的名字
	if got := readFile(t, filepath.Join(workdir, "download", "payload", "1.2.3", "payload.gz")); got != body {
		t.Errorf("{{.Ext}} 渲染出的 target 不对，产物内容 = %q", got)
	}

	// 脚本上下文里同样能拿到
	rendered := readFile(t, filepath.Join(workdir, "vars.txt"))
	for _, want := range []string{"filename=tool-1.2.3.tar.gz", "ext=.gz"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("脚本上下文缺少 %q，实际内容：\n%s", want, rendered)
		}
	}
}

// TestSC_D07_GitHubProxyScope github_proxy 只改写 GitHub 系地址。
//
// 判据不是"日志里说了没走代理"，而是让代理地址指向一个**会拒绝一切请求**的服务：
// 非 GitHub 的下载要是被误改写过去，就一定拿不到内容。
//
// 两格分别挡住两种误判：主机根本无关，以及主机无关但**路径里带 github.com**。
// 后者在旧实现里踩到两道不一致的判据：下载器用整条 URL 做 Contains，改写函数用主机名做
// Contains。结果是这种地址会打出"用户使用代理"却并没有真的走代理 —— 没有误路由，
// 但信号是错的，排查"内网地址为什么慢"的人会被这行日志带去查代理。
// 所以除了产物内容，还断言输出里不许出现这句。
//
// 走 install 而不是 download：旧实现里 download 读的是另一个配置键，代理压根没生效，
// 用 download 的话这两格在缺陷版本上也是绿的。download 那侧由 SC-D07/走代理 覆盖。
func TestSC_D07_GitHubProxyScope(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "proxy should not be used here", http.StatusTeapot)
	}))
	t.Cleanup(proxy.Close)

	for _, tc := range []struct {
		name    string
		srvPath string
	}{
		{"SC-D07/无关主机不走代理", "/payload.sh"},
		{"SC-D07/路径里带 github.com 也不走代理", "/github.com/org/repo/payload.sh"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := "not-from-proxy\n"
			url, _ := fileServerAt(t, tc.srvPath, body)

			workdir := t.TempDir()
			cfg := writeConfig(t, `
github_proxy: "`+proxy.URL+`/"
resources:
  - name: payload
    version: "1.0"
    urls:
      - "`+url+`"
    target: "{{.Filename}}"
`)

			code, out := runIn(t, workdir, "install", "-f", cfg)
			if code != 0 {
				t.Fatalf("非 GitHub 地址被代理改写了，退出码 = %d，输出：\n%s", code, out)
			}
			if got := readFile(t, filepath.Join(workdir, "download", "payload", "1.0", "payload.sh")); got != body {
				t.Errorf("产物内容 = %q，期望 %q", got, body)
			}
			if strings.Contains(out, "使用代理") {
				t.Errorf("没走代理却报告用了代理，输出：\n%s", out)
			}
		})
	}
}

// TestSC_D07_GitHubProxyApplied GitHub 地址确实走代理。
//
// 与上一条互为反证：只断言"非 GitHub 不走代理"的话，一个从不应用代理的实现也能通过。
// 这里让代理服务返回自己的内容，产物内容对得上就说明请求真的落在代理上；
// 真实的 github.com 地址在用例里不可达，所以不走代理就必然失败。
func TestSC_D07_GitHubProxyApplied(t *testing.T) {
	t.Parallel()

	const body = "served-by-proxy\n"
	var gotPath string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(proxy.Close)

	workdir := t.TempDir()
	cfg := writeConfig(t, `
github_proxy: "`+proxy.URL+`/"
resources:
  - name: payload
    version: "1.0"
    urls:
      - "https://github.com/structure-projects/somcli/releases/download/v1.0/payload.sh"
    target: "{{.Filename}}"
`)

	code, out := runIn(t, workdir, "download", "-f", cfg)
	if code != 0 {
		t.Fatalf("GitHub 地址未走代理，退出码 = %d，输出：\n%s", code, out)
	}
	if got := readFile(t, filepath.Join(workdir, "download", "payload", "1.0", "payload.sh")); got != body {
		t.Errorf("产物不是代理提供的内容：%q", got)
	}
	if !strings.HasPrefix(gotPath, "/github.com/structure-projects/somcli/") {
		t.Errorf("代理收到的路径 = %q，未保留原始主机与路径", gotPath)
	}
}

// TestSC_D11_DownloadWithoutWget 操作机上没有 wget / curl 也要能下载。
//
// F5 的回归，也是 somcli 的立身之本：它是"装工具的工具"，典型输入就是一台什么都没装的
// 机器。过去下载唯一的实现是 exec wget，于是在那台机器上 somcli 装不了任何东西 ——
// 包括装不了 wget 本身。
//
// 手法是把 PATH 缩到一个空目录：二进制自己是用绝对路径起的，不受影响，但它 exec 出去的
// 任何外部命令都找不到。这样"其实还在偷偷 shell out"的实现会当场失败。
func TestSC_D11_DownloadWithoutWget(t *testing.T) {
	body := "no-wget-needed\n"
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

	emptyBin := t.TempDir()
	code, out := runEnvIn(t, workdir, []string{"PATH=" + emptyBin}, "download", "-f", cfg)
	if code != 0 {
		t.Fatalf("PATH 里没有任何外部命令时下载失败，退出码 = %d，输出：\n%s", code, out)
	}
	if got := readFile(t, filepath.Join(workdir, "download", "payload", "1.0", "payload.sh")); got != body {
		t.Errorf("产物内容 = %q，期望 %q", got, body)
	}
}
