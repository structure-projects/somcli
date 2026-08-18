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
	"reflect"
	"strings"
	"testing"
)

// TestSC_F07_UnknownConfigKeyRejected 未知配置键必须被拒绝，而不是静默忽略。
//
// 这是本里程碑唯一的破坏性变更（yaml.Unmarshal → UnmarshalStrict），也是最值得的一个：
// 静默忽略意味着用户配了某个能力、以为生效了，实际那一行等于注释。审计里有 9 类键
// 属于这种情况，包括 k8sConfig.resources 这种关键字段。
//
// 每个子用例都要求错误信息里出现出错的键名 —— 报一句"解析失败"等于让用户逐行猜。
func TestSC_F07_UnknownConfigKeyRejected(t *testing.T) {
	cases := []struct {
		name    string
		wantKey string
		cfg     string
	}{
		{
			name:    "SC-F07/整段未知键",
			wantKey: "unknown_section",
			cfg: `
unknown_section:
  foo: bar
resources:
  - name: tool
    version: "1.0"
    post_install:
      - "true"
`,
		},
		{
			name:    "SC-F07/资源里拼错的键",
			wantKey: "postinstall",
			cfg: `
resources:
  - name: tool
    version: "1.0"
    postinstall:
      - "echo 这一行原本被静默忽略"
`,
		},
		{
			name:    "SC-F07/下划线写法拼错",
			wantKey: "pre-install",
			cfg: `
resources:
  - name: tool
    version: "1.0"
    pre-install:
      - "true"
`,
		},
		{
			name:    "SC-F07/重复键",
			wantKey: "version",
			cfg: `
resources:
  - name: tool
    version: "1.0"
    version: "2.0"
    post_install:
      - "true"
`,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			workdir := t.TempDir()
			cfg := writeConfig(t, tc.cfg)

			code, out := runIn(t, workdir, "install", "-f", cfg)
			if code == 0 {
				t.Fatalf("配置含 %s 却退出码 0（该键会被静默忽略），输出：\n%s", tc.wantKey, out)
			}
			if !strings.Contains(out, tc.wantKey) {
				t.Errorf("错误信息未指出出错的键 %q，用户无从定位，输出：\n%s", tc.wantKey, out)
			}
			if strings.Contains(out, "[SUCCESS]") {
				t.Errorf("失败路径上打印了 [SUCCESS]，输出：\n%s", out)
			}
			// 配置没通过校验，脚本一步都不该跑。
			if got := tree(t, workdir); len(got) != 0 {
				t.Errorf("配置校验失败却已经产生了落盘产物：%v", got)
			}
		})
	}
}

// TestSC_F07_GlobalSettingsKeysAccepted 全局设置键必须被接受并生效。
//
// 严格解析的另一面：示例配置里写了 offline / debug / workdir / github_proxy /
// mirrors_source，它们此前既解析不到又无人消费。改严格解析之后，如果不同时补上字段，
// 所有现有示例会全部报错 —— 那就从"静默失效"变成"直接不可用"。
// 这里用 debug 做判据：配置里打开它，输出就该出现 [DEBUG]。
func TestSC_F07_GlobalSettingsKeysAccepted(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, `
debug: true
offline: false
github_proxy: ""
resources:
  - name: tool
    version: "1.0"
    post_install:
      - "echo ok > {{.WorkDir}}/ok.txt"
`)

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code != 0 {
		t.Fatalf("配置里的全局设置键被拒绝了，退出码 = %d，输出：\n%s", code, out)
	}
	if !strings.Contains(out, "[DEBUG]") {
		t.Errorf("配置里 debug: true 没有生效（解析到了但没人消费），输出：\n%s", out)
	}
	if got := strings.TrimSpace(readFile(t, filepath.Join(workdir, "ok.txt"))); got != "ok" {
		t.Errorf("资源没有真正安装，产物 = %q", got)
	}
}

// TestSC_F07_MalformedYamlRejected 语法都不对的 YAML 更要报错，且不留副作用。
func TestSC_F07_MalformedYamlRejected(t *testing.T) {
	workdir := t.TempDir()
	cfg := writeConfig(t, "resources:\n  - name: tool\n   version: \"1.0\"\n")

	code, out := runIn(t, workdir, "install", "-f", cfg)
	if code == 0 {
		t.Fatalf("YAML 语法错误却退出码 0，输出：\n%s", out)
	}
	if got := tree(t, workdir); len(got) != 0 {
		t.Errorf("解析失败却产生了落盘产物：%v", got)
	}
}

// TestD9_HelpHasNoSideEffects 看帮助不得产生任何副作用。
//
// docker-compose 是透传命令，DisableFlagParsing 让 --help 也成了透传参数，
// 于是 somcli docker-compose --help 会走进"compose 缺失就自动下载安装"的分支：
// 用户只想看用法，工具却去动网络和文件系统。这里对每个叶子命令跑 --help，
// 并对 workdir 与 HOME 做前后快照 —— 帮助是纯只读的，快照必须一模一样。
func TestD9_HelpHasNoSideEffects(t *testing.T) {
	commands := [][]string{
		{"--help"},
		{"install", "--help"},
		{"download", "--help"},
		{"version", "--help"},
		{"docker", "--help"},
		{"docker-compose", "--help"},
		{"docker-compose", "-h"},
		{"compose", "--help"},
		{"cluster", "--help"},
		{"images", "--help"},
		{"registry", "--help"},
		{"resources", "--help"},
	}

	for _, args := range commands {
		args := args
		t.Run("D9/"+strings.Join(args, " "), func(t *testing.T) {
			workdir := t.TempDir()
			home := t.TempDir()

			beforeWork := tree(t, workdir)
			beforeHome := tree(t, home)

			code, out := runHelp(t, workdir, home, args...)
			if code != 0 {
				t.Fatalf("%v 退出码 = %d，帮助应当总是成功，输出：\n%s", args, code, out)
			}

			if afterWork := tree(t, workdir); !reflect.DeepEqual(beforeWork, afterWork) {
				t.Errorf("%v 在 workdir 下产生了副作用：\n之前 %v\n之后 %v\n输出：\n%s",
					args, beforeWork, afterWork, out)
			}
			if afterHome := tree(t, home); !reflect.DeepEqual(beforeHome, afterHome) {
				t.Errorf("%v 在 HOME 下产生了副作用：\n之前 %v\n之后 %v\n输出：\n%s",
					args, beforeHome, afterHome, out)
			}
			if strings.Contains(out, "[SUCCESS]") || strings.Contains(out, "开始安装") {
				t.Errorf("%v 的输出里出现了安装动作的痕迹：\n%s", args, out)
			}
		})
	}
}

// runHelp 与 runIn 的区别只在于 HOME 由调用方指定，
// 这样才能对同一个 HOME 做运行前后的对比快照。
func runHelp(t *testing.T, workdir, home string, args ...string) (int, string) {
	t.Helper()

	bin := somcliBinary(t)
	full := append([]string{"--workdir", workdir}, args...)

	cmd := execCommand(bin, full...)
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()

	code := 0
	if exitErr, ok := err.(*exitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("执行 %v 失败: %v\n%s", args, err, out)
	}
	return code, string(out)
}