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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRegistryDocker 造一个记录调用、并让镜像名含 failMark 的 pull/push 失败的假 docker。
//
// sync 的真 docker 依赖守护进程与网络，本机不可能真同步；要验的是编排契约：
// pull→tag→push→rmi 的顺序、单张失败是否中断其余、cleanup 是否在推送失败后仍执行。
// 这些只看"docker 被以什么参数调了几次"，所以用假进程足够。
func fakeRegistryDocker(t *testing.T, logPath, failMark string) string {
	t.Helper()
	script := fmt.Sprintf(`#!/bin/sh
printf 'docker %%s\n' "$*" >> %q
sub="$1"; shift
case "$sub" in
  login|pull|tag) exit 0 ;;
  push)
    for a in "$@"; do
      case "$a" in
        *%s*) echo "boom: push $a" >&2; exit 1 ;;
      esac
    done
    exit 0 ;;
  rmi) exit 0 ;;
  *) exit 0 ;;
esac
`, logPath, failMark)
	return fakeBinDir(t, map[string]string{"docker": script})
}

// SC-C03：harbor install 的入参校验是安装前的第一道闸。
// 这里逐条守住它：缺主机名、主机名不像域名、版本不带 v 前缀都要明确拒绝。
func TestSC_C03_HarborInstallValidation(t *testing.T) {
	t.Run("SC-C03/缺 hostname 被拒", func(t *testing.T) {
		code, out := run(t, "registry", "install", "-v", "v2.5.0")
		if code == 0 {
			t.Fatalf("缺 hostname 却退出 0，输出：\n%s", out)
		}
		if !strings.Contains(out, "required") && !strings.Contains(out, "hostname") {
			t.Errorf("错误信息没指出缺 hostname，输出：\n%s", out)
		}
	})

	t.Run("SC-C03/hostname 不像域名被拒", func(t *testing.T) {
		code, out := run(t, "registry", "install", "-H", "notadomain", "-v", "v2.5.0")
		if code == 0 {
			t.Fatalf("非法 hostname 却退出 0，输出：\n%s", out)
		}
		if !strings.Contains(out, "invalid hostname") {
			t.Errorf("错误信息没指出 hostname 格式问题，输出：\n%s", out)
		}
	})

	t.Run("SC-C03/localhost 作为 hostname 合法", func(t *testing.T) {
		// localhost 是显式放行的特例；能过校验意味着它会继续往下走到下载阶段，
		// 这里用一个不存在的 docker 让它在环境检查处停下，而不是死在 hostname 校验。
		bin := t.TempDir()
		code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin},
			"registry", "install", "-H", "localhost", "-v", "v2.5.0")
		if code == 0 {
			t.Fatalf("没有 docker 却退出 0，输出：\n%s", out)
		}
		if strings.Contains(out, "invalid hostname") {
			t.Errorf("localhost 被误判为非法 hostname，输出：\n%s", out)
		}
		if !strings.Contains(out, "docker") {
			t.Errorf("应当报 docker 未安装，输出：\n%s", out)
		}
	})

	t.Run("SC-C03/版本不带 v 前缀被拒", func(t *testing.T) {
		code, out := run(t, "registry", "install", "-H", "harbor.example.com", "-v", "2.5.0")
		if code == 0 {
			t.Fatalf("非法版本却退出 0，输出：\n%s", out)
		}
		if !strings.Contains(out, "must start with 'v'") {
			t.Errorf("错误信息没指出版本格式，输出：\n%s", out)
		}
	})
}

// SC-C03：docker / docker-compose 缺失时必须在环境检查处停下，
// 而不是走到下载几百 MB 的安装包才失败。
func TestSC_C03_HarborInstallRequiresDocker(t *testing.T) {
	cases := []struct {
		name string
		bin  map[string]string
		want string
	}{
		{name: "两个都没有", bin: nil, want: "docker"},
		{name: "有 docker 没 compose", bin: map[string]string{
			"docker": "#!/bin/sh\nexit 0\n",
		}, want: "docker-compose"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run("SC-C03/"+tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for n, s := range tc.bin {
				_ = os.WriteFile(filepath.Join(dir, n), []byte(s), 0o755)
			}
			code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + dir},
				"registry", "install", "-H", "harbor.example.com")
			if code == 0 {
				t.Fatalf("缺依赖却退出 0，输出：\n%s", out)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("错误信息应指出缺 %s，输出：\n%s", tc.want, out)
			}
		})
	}
}

// E6：uninstall 一个标志都没注册却蹭 install 的校验变量，
// 于是 `registry uninstall` 无论怎么写都先死在 "invalid hostname format"。
// 这里不带任何标志跑卸载：它必须越过参数校验，走到真正的卸载动作
// （假 docker-compose 让它成功），证明校验断链已修。
func TestSC_C04_UninstallWithoutFlagsRuns(t *testing.T) {
	log := filepath.Join(t.TempDir(), "compose.log")
	bin := fakeBinDir(t, map[string]string{
		"docker-compose": "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + log + "\nexit 0\n",
	})
	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin}, "registry", "uninstall")
	if code != 0 {
		t.Fatalf("无标志 uninstall 失败（E6 回归），退出码 %d，输出：\n%s", code, out)
	}
	if strings.Contains(out, "invalid hostname") {
		t.Fatalf("uninstall 仍在做 install 的 hostname 校验，输出：\n%s", out)
	}
	if got := readFile(t, log); !strings.Contains(got, "docker-compose.yml") {
		t.Errorf("没有真正执行 down，compose 调用记录：\n%s", got)
	}
}

// SC-C04：sync 的入参校验。target 必须带协议、并发度有界、镜像清单必须存在。
func TestSC_C04_SyncValidation(t *testing.T) {
	list := writeImageList(t, "library/nginx:1.25")

	t.Run("SC-C04/target 不带协议被拒", func(t *testing.T) {
		code, out := run(t, "registry", "sync",
			"-s", "docker.io", "-t", "harbor.example.com", "-f", list, "-p", "pw")
		if code == 0 {
			t.Fatalf("非法 target 却退出 0，输出：\n%s", out)
		}
		if !strings.Contains(out, "http://") && !strings.Contains(out, "https://") {
			t.Errorf("错误信息没指出协议要求，输出：\n%s", out)
		}
	})

	t.Run("SC-C04/并发度越界被拒", func(t *testing.T) {
		code, out := run(t, "registry", "sync",
			"-s", "docker.io", "-t", "https://harbor.example.com",
			"-f", list, "-p", "pw", "-c", "0")
		if code == 0 {
			t.Fatalf("并发度 0 却退出 0，输出：\n%s", out)
		}
		if !strings.Contains(out, "concurrency") {
			t.Errorf("错误信息没指出并发度问题，输出：\n%s", out)
		}
	})

	t.Run("SC-C04/镜像清单不存在被拒", func(t *testing.T) {
		code, out := run(t, "registry", "sync",
			"-s", "docker.io", "-t", "https://harbor.example.com",
			"-f", filepath.Join(t.TempDir(), "nope.txt"), "-p", "pw")
		if code == 0 {
			t.Fatalf("不存在的清单却退出 0，输出：\n%s", out)
		}
		if !strings.Contains(out, "image list") {
			t.Errorf("错误信息没指出清单问题，输出：\n%s", out)
		}
	})

	t.Run("SC-C04/密码缺失被拒", func(t *testing.T) {
		code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + t.TempDir(), "REGISTRY_PASSWORD="},
			"registry", "sync",
			"-s", "docker.io", "-t", "https://harbor.example.com", "-f", list)
		if code == 0 {
			t.Fatalf("缺密码却退出 0，输出：\n%s", out)
		}
		if !strings.Contains(out, "password") {
			t.Errorf("错误信息没指出密码缺失，输出：\n%s", out)
		}
	})
}

// SC-C04：同步成功路径。逐张走 pull→tag→push→rmi，最后退出 0。
// 用 -c 1 关闭并发，顺序才可断言。
func TestSC_C04_SyncHappyPath(t *testing.T) {
	log := filepath.Join(t.TempDir(), "docker.log")
	bin := fakeRegistryDocker(t, log, "no-such-mark")
	list := writeImageList(t, "library/nginx:1.25", "library/redis:7")

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin},
		"registry", "sync",
		"-s", "docker.io", "-t", "https://harbor.example.com",
		"-f", list, "-u", "admin", "-p", "pw", "-c", "1")
	if code != 0 {
		t.Fatalf("全部成功却退出码 %d，输出：\n%s", code, out)
	}

	got := readFile(t, log)
	for _, img := range []string{"library/nginx:1.25", "library/redis:7"} {
		for _, sub := range []string{"pull", "tag", "push", "rmi"} {
			want := fmt.Sprintf("docker %s", sub)
			if !strings.Contains(got, want) {
				t.Errorf("镜像 %s 没有执行 %s，docker 调用记录：\n%s", img, sub, got)
			}
		}
	}
	// 目标镜像必须是重写到 target 主机后的名字，而不是源名字。
	if !strings.Contains(got, "harbor.example.com/library/nginx:1.25") {
		t.Errorf("没有把镜像重写到目标 registry，调用记录：\n%s", got)
	}
	if !strings.Contains(out, "completed successfully") {
		t.Errorf("成功输出缺失，输出：\n%s", out)
	}
}

// SC-C04：单张失败不得中断其余镜像，但整批必须非 0 退出并点名失败项。
//
// 这是 sync 的核心契约：一次同步几十张，第三张挂掉就停等于让用户重跑全部；
// 但 return nil 会让 CI 以为全成功了。失败要聚合，成功的要继续推完。
func TestSC_C04_SyncAggregatesFailuresWithoutAborting(t *testing.T) {
	log := filepath.Join(t.TempDir(), "docker.log")
	bin := fakeRegistryDocker(t, log, "broken")
	list := writeImageList(t, "library/nginx:1.25", "library/broken:1.0", "library/redis:7")

	code, out := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin},
		"registry", "sync",
		"-s", "docker.io", "-t", "https://harbor.example.com",
		"-f", list, "-u", "admin", "-p", "pw", "-c", "1")
	if code == 0 {
		t.Fatalf("有镜像失败却退出 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "library/broken:1.0") {
		t.Errorf("错误信息没点名失败的镜像，输出：\n%s", out)
	}

	got := readFile(t, log)
	// 失败的是第二张，第一张与第三张都必须被推过 —— 没有遇错即停。
	for _, img := range []string{"library/nginx:1.25", "library/redis:7"} {
		if !strings.Contains(got, "pull "+img) {
			t.Errorf("失败一张后不再处理 %s，调用记录：\n%s", img, got)
		}
	}
}

// SC-C04：cleanup（docker rmi）只在推送成功后执行。
//
// 复核结论：rmi 删的是**本地**镜像标签，已推送到目标 registry 的镜像不受影响，
// 所以成功后清理是安全的；但推送失败时不能 rmi —— 那张本地镜像可能是排障线索，
// 且原实现在 push 失败时直接 return，cleanup 本就不应被执行。
func TestSC_C04_SyncCleanupOnlyAfterSuccessfulPush(t *testing.T) {
	log := filepath.Join(t.TempDir(), "docker.log")
	bin := fakeRegistryDocker(t, log, "broken")
	list := writeImageList(t, "library/good:1.0", "library/broken:1.0")

	code, _ := runEnvIn(t, t.TempDir(), []string{"PATH=" + bin},
		"registry", "sync",
		"-s", "docker.io", "-t", "https://harbor.example.com",
		"-f", list, "-p", "pw", "-c", "1")
	if code == 0 {
		t.Fatal("有镜像失败应当非 0 退出")
	}

	got := readFile(t, log)
	// 好镜像：push 之后有 rmi。
	pushGood := strings.Index(got, "push harbor.example.com/library/good:1.0")
	rmiGood := strings.Index(got, "rmi library/good:1.0")
	if pushGood < 0 || rmiGood < 0 || rmiGood < pushGood {
		t.Errorf("成功镜像应当在 push 之后 rmi，调用记录：\n%s", got)
	}
	// 坏镜像：push 失败，不得有 rmi 清理它。
	if strings.Contains(got, "rmi library/broken:1.0") {
		t.Errorf("推送失败的镜像不应被 rmi 清理，调用记录：\n%s", got)
	}
}
