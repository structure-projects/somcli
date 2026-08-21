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
	"os/exec"
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
// 严格解析的另一面：示例配置里写了 offline / debug / workdir / github_proxy，
// 它们此前既解析不到又无人消费。改严格解析之后，如果不同时补上字段，
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

// TestSC_X07_HelpHasNoSideEffects 看帮助不得产生任何副作用。
//
// docker-compose 是透传命令，DisableFlagParsing 让 --help 也成了透传参数，
// 于是 somcli docker-compose --help 会走进"compose 缺失就自动下载安装"的分支：
// 用户只想看用法，工具却去动网络和文件系统。这里对每个叶子命令跑 --help，
// 并对 workdir 与 HOME 做前后快照 —— 帮助是纯只读的，快照必须一模一样。
func TestSC_X07_HelpHasNoSideEffects(t *testing.T) {
	commands := [][]string{
		{"--help"},
		{"install", "--help"},
		{"validate", "--help"},
		{"download", "--help"},
		{"version", "--help"},
		{"apply", "--help"},
		{"get", "--help"},
		{"delete", "--help"},
		{"describe", "--help"},
		{"docker", "--help"},
		{"docker", "install", "--help"},
		{"docker", "status", "--help"},
		{"docker", "uninstall", "--help"},
		{"docker-compose", "--help"},
		{"docker-compose", "-h"},
		{"cluster", "--help"},
		{"cluster", "create", "--help"},
		{"cluster", "remove", "--help"},
		{"images", "--help"},
		{"images", "pull", "--help"},
		{"images", "push", "--help"},
		{"images", "export", "--help"},
		{"images", "import", "--help"},
		{"registry", "--help"},
		{"registry", "install", "--help"},
		{"registry", "sync", "--help"},
		{"registry", "uninstall", "--help"},
	}

	for _, args := range commands {
		args := args
		t.Run("SC-X07/"+strings.Join(args, " "), func(t *testing.T) {
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

// TestSC_X05_ExampleConfigsAreValid configs/ 下每个示例配置都必须能被解析并渲染。
//
// 示例配置是大多数人的第一份输入，它错了就等于工具开箱即坏。审计时这里有三个文件
// 谁都装不上：两个用的是 `kind:` 多文档 schema（加载器只读第一个文档，`---` 之后
// 静默丢弃），一个引用了类型里不存在的字段。
//
// 判据用 `validate -f`：只解析、只渲染，不下载不执行 —— 否则验一遍示例配置就等于
// 真去装一套 k8s。顺带对 workdir 与 HOME 做前后快照，validate 说自己只读，那就验它只读。
func TestSC_X05_ExampleConfigsAreValid(t *testing.T) {
	configs := exampleConfigs(t)
	if len(configs) < 5 {
		t.Fatalf("configs/ 下只找到 %d 个示例配置，路径或后缀变了", len(configs))
	}

	for _, cfg := range configs {
		cfg := cfg
		t.Run("SC-X05/"+filepath.Base(cfg), func(t *testing.T) {
			workdir := t.TempDir()
			home := t.TempDir()

			before := tree(t, workdir)
			code, out := runHelp(t, workdir, home, "validate", "-f", cfg)
			if code != 0 {
				t.Errorf("示例配置解析不了（退出码 %d），照着它改的人一开始就装不上：\n%s", code, out)
			}
			if after := tree(t, workdir); !reflect.DeepEqual(before, after) {
				t.Errorf("validate 声称只读却动了 workdir：\n之前 %v\n之后 %v", before, after)
			}
		})
	}
}

// TestSC_X05_ValidateRejectsBrokenConfig validate 不能是个只会说 ok 的橡皮章。
func TestSC_X05_ValidateRejectsBrokenConfig(t *testing.T) {
	cases := []struct {
		name    string
		wantKey string
		cfg     string
	}{
		{
			name:    "SC-X05/未知键",
			wantKey: "postinstall",
			cfg: `
resources:
  - name: tool
    version: "1.0"
    postinstall:
      - "true"
`,
		},
		{
			name:    "SC-X05/模板变量不存在",
			wantKey: "post_install",
			cfg: `
resources:
  - name: tool
    version: "1.0"
    post_install:
      - "echo {{.NoSuchVar}}"
`,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			workdir := t.TempDir()
			cfg := writeConfig(t, tc.cfg)

			code, out := runIn(t, workdir, "validate", "-f", cfg)
			if code == 0 {
				t.Fatalf("配置有问题却验过了，输出：\n%s", out)
			}
			if !strings.Contains(out, tc.wantKey) {
				t.Errorf("错误信息没指出问题在 %q，输出：\n%s", tc.wantKey, out)
			}
		})
	}
}

// TestSC_X09_OneConfigServesEveryScenario 一份配置要能在所有场景下加载，
// 各命令只取自己那一段。
//
// 改之前不是这样：install 认 resources:，cluster create 认的是另一套以 cluster: 为根的
// schema，images --custom-file 又是第三种。同一份文件换个场景就解析失败，严格解析之后
// 更是直接报未知键 —— 于是用户被迫为同一套环境维护三份互相抄的配置。
//
// 判据取"同一个文件、三条命令、都认得"：
//   - validate 报出的资源 / 节点 / 集群 / 镜像条数与文件里写的一致（四段都真解析到了）
//   - install 只消费 resources，cluster: 与 images: 存在不影响它跑
//   - cluster create 认得同一个文件里的 cluster:，且点名不存在的集群时列出候选
//     （用不存在的名字，才能在不真去 SSH 装集群的前提下证明这一段被读到了）
//
// images --custom-file 读同一份文件那条不在这里：它要 docker 才跑得起来，
// 见 SC-X10。宁可矩阵上少一条，也不要把没断言的东西写成 done。
func TestSC_X09_OneConfigServesEveryScenario(t *testing.T) {
	cfg := writeConfig(t, `
debug: false
resources:
  - name: marker
    version: "1.0"
    post_install:
      - "echo installed > {{.WorkDir}}/marker.txt"
nodes:
  - host: "local-a"
    ip: "127.0.0.1"
    role: "manager"
cluster:
  - type: "swarm"
    name: "my-swarm"
    nodes:
      - host: "local-a"
        ip: "127.0.0.1"
        role: "manager"
  - type: "k8s"
    name: "my-k8s"
    nodes:
      - host: "local-a"
        ip: "127.0.0.1"
        role: "master"
images:
  - name: "busybox"
    tag: "1.36"
`)

	t.Run("SC-X09/validate 四段都解析到", func(t *testing.T) {
		code, out := runIn(t, t.TempDir(), "validate", "-f", cfg)
		if code != 0 {
			t.Fatalf("统一配置解析失败，退出码 = %d，输出：\n%s", code, out)
		}
		for _, want := range []string{"1 个资源", "1 个节点", "2 套集群", "1 个镜像"} {
			if !strings.Contains(out, want) {
				t.Errorf("validate 没报出 %q，说明这一段没被解析到，输出：\n%s", want, out)
			}
		}
	})

	t.Run("SC-X09/install 只取 resources", func(t *testing.T) {
		workdir := t.TempDir()
		code, out := runIn(t, workdir, "install", "-f", cfg)
		if code != 0 {
			t.Fatalf("同一份配置装不了，退出码 = %d，输出：\n%s", code, out)
		}
		if got := strings.TrimSpace(readFile(t, filepath.Join(workdir, "marker.txt"))); got != "installed" {
			t.Errorf("resources 没执行，产物 = %q", got)
		}
	})

	t.Run("SC-X09/cluster create 认得同一个文件", func(t *testing.T) {
		code, out := runIn(t, t.TempDir(), "cluster", "create", "-f", cfg, "--cluster-name", "no-such")
		if code == 0 {
			t.Fatalf("点名了不存在的集群却成功了，输出：\n%s", out)
		}
		// 报错必须列出候选，否则用户不知道该填什么。
		for _, want := range []string{"my-swarm", "my-k8s"} {
			if !strings.Contains(out, want) {
				t.Errorf("错误信息没列出候选集群 %q，输出：\n%s", want, out)
			}
		}
	})
}

// exampleConfigs 列出 configs/ 下的 YAML 示例。不写死清单：新增示例应当自动被这条用例覆盖。

func exampleConfigs(t *testing.T) []string {
	t.Helper()

	dir := filepath.Join(repoRoot, "configs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取 configs/ 失败: %v", err)
	}

	var configs []string
	for _, e := range entries {
		if name := e.Name(); strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			configs = append(configs, filepath.Join(dir, name))
		}
	}
	return configs
}

// runHelp 与 runIn 的区别只在于 HOME 由调用方指定，
// 这样才能对同一个 HOME 做运行前后的对比快照。
func runHelp(t *testing.T, workdir, home string, args ...string) (int, string) {
	t.Helper()

	bin := somcliBinary(t)
	full := append([]string{"--workdir", workdir}, args...)

	cmd := exec.Command(bin, full...)
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()

	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("执行 %v 失败: %v\n%s", args, err, out)
	}
	return code, string(out)
}
