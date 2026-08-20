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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeCompose 放一个假的 docker-compose：把收到的参数逐行写进 argsFile 再成功退出。
//
// 有了它，"到底有哪些参数被交给了下游"就是可观察的，不需要装 docker 也不需要联网。
func fakeCompose(t *testing.T) (bin, argsFile string) {
	t.Helper()

	dir := t.TempDir()
	bin = filepath.Join(dir, "docker-compose")
	argsFile = filepath.Join(dir, "args.txt")

	// version --short 也要能应付：透传前 somcli 会检查它装没装
	script := fmt.Sprintf(`#!/bin/sh
for a in "$@"; do echo "$a" >> %q; done
exit 0
`, argsFile)

	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("写假 compose 失败: %v", err)
	}
	return bin, argsFile
}

// passedArgs 返回假 compose 收到的参数。文件不存在说明它压根没被调用。
func passedArgs(t *testing.T, argsFile string) []string {
	t.Helper()

	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("假 compose 没有被调用（%s 不存在）: %v", argsFile, err)
	}
	var args []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			args = append(args, line)
		}
	}
	return args
}

// SC-X08：somcli 自己的全局 flag 要生效，且不能泄漏给下游。
//
// 透传命令开了 DisableFlagParsing，cobra 既不解析 somcli 的全局 flag 也不摘掉它们，
// 于是 --workdir 既没生效（产物落回 ./somwork）又被当成 compose 的参数传下去（D10）。
// 两半都得断言：只断言"没泄漏"的话，把全局 flag 直接丢掉也能过。
func TestSC_X08_GlobalFlagsApplyAndDoNotLeak(t *testing.T) {
	bin, argsFile := fakeCompose(t)
	workdir := t.TempDir()

	code, out := runIn(t, workdir, "--debug", "docker-compose", "--path", bin, "logs", "web")
	if code != 0 {
		t.Fatalf("透传退出码 = %d，输出：\n%s", code, out)
	}

	got := passedArgs(t, argsFile)
	want := []string{"logs", "web"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("交给下游的参数 = %v，期望 %v（somcli 自己的 flag 不得泄漏）", got, want)
	}

	// --debug 生效的可观察证据：调试日志里能看到 somcli 打的那条透传记录
	if !strings.Contains(out, "透传命令到 Docker Compose") {
		t.Fatalf("--debug 没有生效（看不到调试输出），输出：\n%s", out)
	}
}

// SC-X08：--workdir 在透传路径下同样要落到实处。
//
// 判据用自动安装：compose 缺失时透传会先装一遍，产物必须落在 --workdir 指定的目录下。
// 这是"全局 flag 真的生效"最直接的证据 —— 原实现下它连解析都没有，
// 产物会落回 ./somwork（即仓库目录）。
func TestSC_X08_WorkdirFlagReachesPassthrough(t *testing.T) {
	const autoVersion = "2.24.0" // 透传自动安装用的缺省版本

	argsFile := filepath.Join(t.TempDir(), "args.txt")
	srv, _ := composeAssetServer(t, autoVersion, http.StatusOK)
	srv.Config.Handler = recordingComposeHandler(argsFile, autoVersion)

	workdir := filepath.Join(t.TempDir(), "custom-work")
	installPath := filepath.Join(t.TempDir(), "docker-compose")

	code, out := runIn(t, workdir, "--github-proxy", srv.URL+"/",
		"docker-compose", "--path", installPath, "ps")
	if code != 0 {
		t.Fatalf("透传退出码 = %d，输出：\n%s", code, out)
	}

	cached := filepath.Join(workdir, "download", "docker-compose", autoVersion, "docker-compose")
	if _, err := os.Stat(cached); err != nil {
		t.Fatalf("--workdir 没有生效：产物不在 %s（%v）\n输出：\n%s", cached, err, out)
	}

	got := strings.Join(passedArgs(t, argsFile), " ")
	if strings.Contains(got, workdir) || strings.Contains(got, "--workdir") {
		t.Fatalf("--workdir 泄漏给了下游：%s", got)
	}
	if !strings.Contains(got, "ps") {
		t.Fatalf("下游没收到 ps：%s", got)
	}
}

// SC-X08：下游自己的同形 flag 不能被吞掉。
//
// -p 是 docker compose 的项目名，-e 是 exec 的环境变量。原实现按名字硬摘一批 flag
// （含 -p / -e），于是 `compose -p myproj up` 丢了项目名。
func TestSC_X08_DownstreamFlagsSurvive(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"项目名 -p", []string{"-p", "myproj", "logs"}, []string{"-p", "myproj", "logs"}},
		{"exec 的 -e", []string{"exec", "-e", "K=V", "web", "true"}, []string{"exec", "-e", "K=V", "web", "true"}},
		{"编排文件 -f", []string{"-f", "stack.yml", "logs"}, []string{"-f", "stack.yml", "logs"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bin, argsFile := fakeCompose(t)

			args := append([]string{"docker-compose", "--path", bin}, tc.args...)
			code, out := run(t, args...)
			if code != 0 {
				t.Fatalf("透传退出码 = %d，输出：\n%s", code, out)
			}

			got := passedArgs(t, argsFile)
			if strings.Join(got, " ") != strings.Join(tc.want, " ") {
				t.Fatalf("交给下游的参数 = %v，期望 %v", got, tc.want)
			}
		})
	}
}

// recordingComposeHandler 提供一个"既能报版本又会记下自己收到什么参数"的假 compose。
// 自动安装场景下假二进制是下载来的，所以记录逻辑必须写在被下载的内容里。
func recordingComposeHandler(argsFile, version string) http.Handler {
	body := fmt.Sprintf(`#!/bin/sh
for a in "$@"; do echo "$a" >> %q; done
echo %s
`, argsFile, version)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, body)
	})
}

// SC-X08：--env-file 是 somcli 声明的 flag，得原样转交给 compose。
//
// 原实现把它写进 COMPOSE_FILE —— 那是"编排文件"，compose 会拿 .env 当 yaml 解析。
func TestSC_X08_EnvFileForwardedAsEnvFile(t *testing.T) {
	bin, argsFile := fakeCompose(t)

	envFile := filepath.Join(t.TempDir(), ".env.prod")
	if err := os.WriteFile(envFile, []byte("FOO=bar\n"), 0o644); err != nil {
		t.Fatalf("写 env 文件失败: %v", err)
	}

	code, out := run(t, "docker-compose", "--path", bin, "--env-file", envFile, "logs")
	if code != 0 {
		t.Fatalf("透传退出码 = %d，输出：\n%s", code, out)
	}

	got := strings.Join(passedArgs(t, argsFile), " ")
	if !strings.Contains(got, "--env-file "+envFile) {
		t.Fatalf("--env-file 没有转交给下游，实际参数：%s", got)
	}
}

// SC-X08：env 文件不存在要明确报错，不是静默忽略。
func TestSC_X08_MissingEnvFileFailsLoudly(t *testing.T) {
	bin, _ := fakeCompose(t)

	code, out := run(t, "docker-compose", "--path", bin, "--env-file", "/nope/.env", "logs")
	if code == 0 {
		t.Fatalf("env 文件不存在却退出 0，输出：\n%s", out)
	}
	if !strings.Contains(out, "/nope/.env") {
		t.Fatalf("错误信息没有点出缺的是哪个文件，输出：\n%s", out)
	}
}

// SC-X08：全局 flag 在前也不影响 --help 拦截，且帮助不得触发安装。
//
// D9 在 M0 修的是 args[0] 不是 --help 的情形；这里连同全局 flag 一起再守一遍。
func TestSC_X08_HelpStillInterceptedAfterGlobalFlags(t *testing.T) {
	workdir := filepath.Join(t.TempDir(), "work")
	missing := filepath.Join(t.TempDir(), "docker-compose")

	code, out := runIn(t, workdir, "--debug", "docker-compose", "--path", missing, "--help")
	if code != 0 {
		t.Fatalf("查看帮助退出码 = %d，输出：\n%s", code, out)
	}
	if _, err := os.Stat(missing); err == nil {
		t.Fatalf("查看帮助触发了安装：%s 被创建了", missing)
	}
	if _, err := os.Stat(workdir); err == nil {
		t.Fatalf("查看帮助创建了工作目录 %s", workdir)
	}
}
