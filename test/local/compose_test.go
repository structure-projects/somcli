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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// composeAssetServer 冒充 GitHub release：记录被请求的路径，返回一个能报版本的假 compose。
//
// 用 --github-proxy 指向它，就不必联外网也不必真下 docker compose ——
// 断言的是"要的是哪个文件、装到了哪儿、装成什么样"，这些与文件内容是否真是 compose 无关。
func composeAssetServer(t *testing.T, reportVersion string, status int) (*httptest.Server, func() []string) {
	t.Helper()

	var (
		mu    sync.Mutex
		paths []string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()

		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		fmt.Fprintf(w, "#!/bin/sh\necho %s\n", reportVersion)
	}))
	t.Cleanup(srv.Close)

	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), paths...)
	}
}

// unameArch 是 uname -m 风格的架构名。用例自己算一遍，不去读被测代码的实现。
func unameArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	case "386":
		return "i386"
	default:
		return runtime.GOARCH
	}
}

// SC-C02：装 compose 必须真的落盘、可执行、版本是入参那个、架构是本机那个。
//
// 四条断言各自对应一个缺陷：产物存在（D8：报告成功但从不安装）、可执行位（F13）、
// 请求的是 v2.25.0（忽略入参版本，永远装 defaultVersion）、请求的是本机架构
// （URL 里硬编码 x86_64，arm64 上必然 404）。
func TestSC_C02_InstallLandsExecutableOfRequestedVersion(t *testing.T) {
	const version = "2.25.0"

	srv, requested := composeAssetServer(t, version, http.StatusOK)
	installPath := filepath.Join(t.TempDir(), "docker-compose")

	code, out := run(t, "--github-proxy", srv.URL+"/",
		"docker-compose", "install", version, "--path", installPath)
	if code != 0 {
		t.Fatalf("安装退出码 = %d，期望 0，输出：\n%s", code, out)
	}

	info, err := os.Stat(installPath)
	if err != nil {
		t.Fatalf("安装后 %s 不存在: %v\n输出：\n%s", installPath, err, out)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("安装后 %s 没有可执行权限（mode=%v）", installPath, info.Mode())
	}

	wantAsset := fmt.Sprintf("/v%s/docker-compose-%s-%s", version, runtime.GOOS, unameArch())
	got := requested()
	if len(got) == 0 {
		t.Fatalf("安装过程没有发起任何下载请求，输出：\n%s", out)
	}
	hit := false
	for _, p := range got {
		if strings.HasSuffix(p, wantAsset) {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("下载的资产不对：期望以 %q 结尾，实际请求了 %v", wantAsset, got)
	}
}

// SC-C02：somcli 算出来的下载地址在 GitHub 上真实存在（amd64 与 arm64 各一次）。
//
// 上面那几条用假服务器，验的是"URL 里的架构名对不对"，验不了"这个 URL 真的存在" ——
// 架构名的映射表写错一个字母（aarch64 写成 arm64v8），假服务器照样返回 200。
// F13 的本质就是"在 arm64 机器上必然 404"，那只有真的去问一次 GitHub 才知道。
//
// 地址不是用例自己拼的：先让 somcli 对着假服务器跑一次，把它请求的**整条路径**记下来，
// 再把主机名换回 github.com。这样架构映射依然完全出自被测程序。
//
// 只在 CI 上跑：本机可能没有外网，跳过比让人莫名其妙地红一条更好。CI 的 test 矩阵
// 同时有 amd64 与 arm64 的机器，两台都绿就等于两种架构都验过了。
// 只发 HEAD 不下载正文，失败重试一次 —— GitHub 限流不该让整条流水线红。
func TestSC_C02_ReleaseAssetExistsForThisArch(t *testing.T) {
	if os.Getenv("CI") == "" {
		t.Skip("这条要访问 GitHub，只在 CI 上跑；本机由假服务器那几条覆盖")
	}

	const version = "2.24.0"

	srv, requested := composeAssetServer(t, version, http.StatusOK)
	installPath := filepath.Join(t.TempDir(), "docker-compose")

	code, out := run(t, "--github-proxy", srv.URL+"/",
		"docker-compose", "install", version, "--path", installPath)
	if code != 0 {
		t.Fatalf("对着假服务器安装失败，退出码 = %d，输出：\n%s", code, out)
	}

	realURL := ""
	for _, p := range requested() {
		// 代理地址的形式是 <proxy>/<原主机>/<原路径>，所以路径首段就是被改写掉的主机名
		if rest := strings.TrimPrefix(p, "/"); strings.HasPrefix(rest, "github.com/") {
			realURL = "https://" + rest
		}
	}
	if realURL == "" {
		t.Fatalf("从请求记录里还原不出真实地址，记录：%v\n输出：\n%s", requested(), out)
	}

	status, err := headWithRetry(realURL)
	if err != nil {
		t.Fatalf("请求 %s 失败（网络问题，不是代码问题）：%v", realURL, err)
	}
	if status == http.StatusNotFound {
		t.Fatalf("somcli 算出的地址在 GitHub 上不存在（404）：%s\n"+
			"本机架构 %s，这正是 F13 在 arm64 上的表现", realURL, runtime.GOARCH)
	}
	if status < 200 || status > 299 {
		t.Fatalf("请求 %s 得到 %d，既不是 404 也不是成功 —— 当作网络异常看，重跑一次", realURL, status)
	}
}

// headWithRetry 发一次 HEAD，失败重试一次。只要状态码，不读正文。
func headWithRetry(url string) (int, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			time.Sleep(3 * time.Second)
		}
		resp, err := client.Head(url)
		if err != nil {
			lastErr = err
			continue
		}
		resp.Body.Close()
		return resp.StatusCode, nil
	}
	return 0, lastErr
}

// SC-C02：下载不到就必须失败，且不留下半个产物。
//
// 这条是 F13 的正面：原实现丢掉了 installer.Install 的返回值，
// 404 也照样打印"安装成功"并以 0 退出。
func TestSC_C02_InstallFailsWhenAssetMissing(t *testing.T) {
	srv, _ := composeAssetServer(t, "2.25.0", http.StatusNotFound)
	installPath := filepath.Join(t.TempDir(), "docker-compose")

	code, out := run(t, "--github-proxy", srv.URL+"/",
		"docker-compose", "install", "2.25.0", "--path", installPath)
	if code == 0 {
		t.Fatalf("资产 404 时退出码 = 0，输出：\n%s", out)
	}
	if _, err := os.Stat(installPath); err == nil {
		t.Fatalf("安装失败却在 %s 留下了产物", installPath)
	}
}

// SC-C02：装出来的东西版本不对也算失败。
//
// 校验的意义在于把"装错了"就地暴露，而不是等用户下次跑 compose 命令时才发现。
func TestSC_C02_InstallRejectsVersionMismatch(t *testing.T) {
	srv, _ := composeAssetServer(t, "1.29.2", http.StatusOK)
	installPath := filepath.Join(t.TempDir(), "docker-compose")

	code, out := run(t, "--github-proxy", srv.URL+"/",
		"docker-compose", "install", "2.25.0", "--path", installPath)
	if code == 0 {
		t.Fatalf("版本不符时退出码 = 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "2.25.0") || !strings.Contains(out, "1.29.2") {
		t.Fatalf("错误信息没有同时点出期望与实际版本，输出：\n%s", out)
	}
}

// SC-C02：version 子命令读的是刚装上的那个，uninstall 之后就读不到了。
func TestSC_C02_VersionThenUninstall(t *testing.T) {
	const version = "2.25.0"

	srv, _ := composeAssetServer(t, version, http.StatusOK)
	installPath := filepath.Join(t.TempDir(), "docker-compose")

	if code, out := run(t, "--github-proxy", srv.URL+"/",
		"docker-compose", "install", version, "--path", installPath); code != 0 {
		t.Fatalf("安装退出码 = %d，输出：\n%s", code, out)
	}

	code, out := run(t, "docker-compose", "version", "--path", installPath)
	if code != 0 {
		t.Fatalf("version 退出码 = %d，输出：\n%s", code, out)
	}
	if !strings.Contains(out, version) {
		t.Fatalf("version 没有报出 %s，输出：\n%s", version, out)
	}

	if code, out := run(t, "docker-compose", "uninstall", "--path", installPath); code != 0 {
		t.Fatalf("卸载退出码 = %d，输出：\n%s", code, out)
	}
	if _, err := os.Stat(installPath); err == nil {
		t.Fatalf("卸载后 %s 仍然存在", installPath)
	}
}
