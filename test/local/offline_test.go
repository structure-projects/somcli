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
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// TestSC_D08_OfflineCacheHit 离线模式命中缓存 → 成功，且不碰网络。
//
// 判据是**把源关掉**再跑：源还活着的话，一个压根没实现离线的实现照样会绿。
// 关掉之后能成功，就只可能是拿的缓存。
func TestSC_D08_OfflineCacheHit(t *testing.T) {
	const body = "cached-payload\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))

	workdir := t.TempDir()
	cfg := writeConfig(t, `
resources:
  - name: payload
    version: "1.0"
    urls:
      - "`+srv.URL+`/payload.sh"
    target: "{{.Filename}}"
`)

	if code, out := runIn(t, workdir, "download", "-f", cfg); code != 0 {
		srv.Close()
		t.Fatalf("预热缓存失败，退出码 = %d，输出：\n%s", code, out)
	}
	srv.Close()

	code, out := runIn(t, workdir, "--offline", "download", "-f", cfg)
	if code != 0 {
		t.Fatalf("源已关闭、缓存已存在，离线下载仍失败，退出码 = %d，输出：\n%s", code, out)
	}
	if got := readFile(t, filepath.Join(workdir, "download", "payload", "1.0", "payload.sh")); got != body {
		t.Errorf("缓存内容 = %q，期望 %q", got, body)
	}
}

// TestSC_D09_OfflineCacheMissFails 离线模式缺缓存 → 明确报错，不偷偷联网。
//
// 这里故意让源**活着**：离线模式的意义就是"就算能连也不连"。实现要是只把离线当成
// "先看缓存、没有就下载"，这条会绿在网络上 —— 于是真正断网的现场才第一次暴露问题。
// 错误信息里必须出现缺的那个文件路径，否则用户不知道该往哪儿放离线包。
func TestSC_D09_OfflineCacheMissFails(t *testing.T) {
	const body = "should-not-be-fetched\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
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

	code, out := runIn(t, workdir, "--offline", "download", "-f", cfg)
	if code == 0 {
		t.Fatalf("离线模式下缓存缺失却退出码 0（说明偷偷联网了），输出：\n%s", out)
	}
	if strings.Contains(out, "[SUCCESS]") {
		t.Errorf("失败路径上打印了 [SUCCESS]，输出：\n%s", out)
	}
	if !strings.Contains(out, "离线") {
		t.Errorf("错误信息没说清是离线模式导致的，输出：\n%s", out)
	}
	want := filepath.Join(workdir, "download", "payload", "1.0", "payload.sh")
	if !strings.Contains(out, want) {
		t.Errorf("错误信息未指出缺失的文件 %s，输出：\n%s", want, out)
	}
}

// TestSC_X03_OfflineSwitches 三种打开离线模式的写法必须等效。
//
// --offline / SOMCLI_OFFLINE=true / 配置里的 offline: true 三处各有各的代码路径，
// 谁失效都表现为"以为断网了其实在联网"，而这件事只有真断网时才会暴露。
// 统一用"源活着但必须失败"作判据。
func TestSC_X03_OfflineSwitches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("reachable\n"))
	}))
	t.Cleanup(srv.Close)

	resource := `
resources:
  - name: payload
    version: "1.0"
    urls:
      - "` + srv.URL + `/payload.sh"
    target: "{{.Filename}}"
`

	t.Run("SC-X03/--offline 标志", func(t *testing.T) {
		cfg := writeConfig(t, resource)
		code, out := runIn(t, t.TempDir(), "--offline", "download", "-f", cfg)
		if code == 0 {
			t.Fatalf("--offline 未生效，输出：\n%s", out)
		}
	})

	t.Run("SC-X03/SOMCLI_OFFLINE 环境变量", func(t *testing.T) {
		cfg := writeConfig(t, resource)
		code, out := runEnvIn(t, t.TempDir(), []string{"SOMCLI_OFFLINE=true"}, "download", "-f", cfg)
		if code == 0 {
			t.Fatalf("SOMCLI_OFFLINE=true 未生效，输出：\n%s", out)
		}
	})

	t.Run("SC-X03/配置里的 offline", func(t *testing.T) {
		cfg := writeConfig(t, "offline: true\n"+resource)
		code, out := runIn(t, t.TempDir(), "download", "-f", cfg)
		if code == 0 {
			t.Fatalf("配置里的 offline: true 未生效，输出：\n%s", out)
		}
	})

	// 反证：不开任何开关时同一份配置必须能下载成功。
	// 少了这一格，上面三条在"下载永远失败"的实现上也全绿。
	t.Run("SC-X03/不开离线则正常下载", func(t *testing.T) {
		workdir := t.TempDir()
		cfg := writeConfig(t, resource)
		code, out := runIn(t, workdir, "download", "-f", cfg)
		if code != 0 {
			t.Fatalf("未开离线却下载失败，退出码 = %d，输出：\n%s", code, out)
		}
	})
}
