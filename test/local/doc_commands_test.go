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
	"strings"
	"testing"
)

// TestSC_X04_DocumentedCommandsExist 文档里写出来的每条命令与标志都必须真实存在。
//
// 用户照文档敲一条不存在的命令，得到的是 unknown command —— 这类缺陷没有任何运行时
// 征兆，只能靠把文档当输入来验。手法：从 README 与 doc/*.md 的代码块里抽出所有 somcli
// 调用，对命令路径跑 --help（退出码非 0 即不存在），并要求用到的标志出现在帮助输出里。
//
// 只跑 --help、不执行真实动作，前提是 SC-X07（帮助无副作用）成立。
func TestSC_X04_DocumentedCommandsExist(t *testing.T) {
	total := 0
	for _, doc := range docFiles(t) {
		doc := doc
		calls := docCalls(t, doc)
		total += len(calls)
		t.Run("SC-X04/"+filepath.Base(doc), func(t *testing.T) {
			for _, call := range calls {
				code, out := run(t, append(call.path, "--help")...)
				if code != 0 {
					t.Errorf("%s:%d 文档里的 `somcli %s` 不存在（--help 退出码 %d）\n文档原文：%s",
						doc, call.line, strings.Join(call.path, " "), code, call.raw)
					continue
				}
				registered := helpFlags(out)
				for _, flag := range call.flags {
					if !registered[flag] {
						t.Errorf("%s:%d `somcli %s` 未注册文档里用到的 %s\n文档原文：%s",
							doc, call.line, strings.Join(call.path, " "), flag, call.raw)
					}
				}
			}
		})
	}

	// 一条也没抽到，说明抽取逻辑或文档格式变了 —— 那这条用例就是永远绿的空壳。
	if total == 0 {
		t.Fatal("所有文档里都没抽到 somcli 调用，抽取逻辑失效")
	}
}

// docFiles 收集要验的文档。
//
// 排除 doc/提案-*.md 与 doc/设计.md：前者是技术附录，成篇引用的正是"当前不可用"的命令；
// 后者描述目标设计而非当前实现。把它们算进来，验的就不是"文档与工具是否一致"了。
func docFiles(t *testing.T) []string {
	t.Helper()

	docs := []string{filepath.Join(repoRoot, "README.md")}

	entries, err := os.ReadDir(filepath.Join(repoRoot, "doc"))
	if err != nil {
		t.Fatalf("读取 doc/ 失败: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, "提案-") || name == "设计.md" {
			continue
		}
		docs = append(docs, filepath.Join(repoRoot, "doc", name))
	}
	return docs
}

type docCall struct {
	line  int      // 文档行号，报错时直接可定位
	raw   string   // 文档原文
	path  []string // 命令路径（somcli 之后、第一个标志之前的词）
	flags []string // 该行用到的标志
}

// docCalls 从一个文档里抽出代码块中的 somcli 调用。
func docCalls(t *testing.T, path string) []docCall {
	t.Helper()

	var calls []docCall
	inBlock := false

	for i, raw := range strings.Split(readFile(t, path), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "```") {
			inBlock = !inBlock
			continue
		}
		if !inBlock {
			continue
		}

		line = strings.TrimPrefix(line, "$ ")
		line = strings.TrimPrefix(line, "sudo ")
		if !strings.HasPrefix(line, "somcli ") {
			continue
		}
		// 含占位符、重定向或管道的示例不是一条可执行命令，跳过。
		if strings.ContainsAny(line, "<>|$*`[]{}") {
			continue
		}

		call := docCall{line: i + 1, raw: line}
		for _, tok := range strings.Fields(line)[1:] {
			switch {
			case tok == "\\":
				// 续行符，后面的参数在下一行，本行只验到这里。
			case tok == "--":
				// 之后的都是透传给下游的参数，与 somcli 无关。
			case strings.HasPrefix(tok, "-"):
				name, _, _ := strings.Cut(tok, "=")
				call.flags = append(call.flags, name)
			case len(call.flags) == 0:
				call.path = append(call.path, tok)
			}
		}
		if len(call.path) == 0 && len(call.flags) == 0 {
			continue
		}

		// 透传命令下游的参数不属于 somcli：`somcli docker ps -a` 的 -a 是 docker 的标志，
		// 而 `docker-compose up --help` 会把 --help 交给 compose、触发"缺失就自动安装"。
		// 所以只验这条透传命令本身存在，参数一概不深究。
		if len(call.path) > 0 && isPassthroughCall(t, call.path) {
			call.path = call.path[:1]
			call.flags = nil
		}

		calls = append(calls, call)
	}

	if len(calls) == 0 {
		// 有的文档（如 docker-manager.md 讲的是独立 bash 脚本）本来就不含 somcli 调用。
		// 整体抽不到才是抽取逻辑坏了，那个断言放在调用方。
		return nil
	}
	return calls
}

// passthroughRoots 把参数原样交给下游 CLI 的命令。
// 它们各自真实注册的子命令（docker install / docker-compose version …）仍按 somcli 的标志验，
// 其余一律视为下游参数。
var passthroughRoots = map[string]bool{
	"docker":         true,
	"docker-compose": true,
	"compose":        true,
	"dc":             true,
}

func isPassthroughCall(t *testing.T, path []string) bool {
	t.Helper()

	if !passthroughRoots[path[0]] {
		return false
	}
	if len(path) == 1 {
		return true
	}
	return !registeredSubcommands(t, path[0])[path[1]]
}

// helpFlags 从 --help 输出里读出真实注册的标志名，短名长名都收。
// 必须精确成集合而不是子串匹配：`strings.Contains(out, "-v")` 会被 `--version` 命中，
// 于是文档里写错的 `-v` 反而验过了。
func helpFlags(out string) map[string]bool {
	flags := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "-") {
			continue
		}
		// 形如 `-f, --file string   说明` 或 `      --debug   说明`
		head, _, _ := strings.Cut(line, "   ")
		for _, part := range strings.Split(head, ",") {
			fields := strings.Fields(part)
			if len(fields) == 0 || !strings.HasPrefix(fields[0], "-") {
				continue
			}
			flags[fields[0]] = true
		}
	}
	return flags
}

// registeredSubcommands 从 `somcli <cmd> --help` 的 Available Commands 段里读子命令名。
// 不写死清单：清单会过期，而帮助输出就是命令树的事实来源。
func registeredSubcommands(t *testing.T, cmd string) map[string]bool {
	t.Helper()

	code, out := run(t, cmd, "--help")
	if code != 0 {
		t.Fatalf("somcli %s --help 退出码 = %d，输出：\n%s", cmd, code, out)
	}

	names := map[string]bool{}
	inList := false
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Available Commands:") {
			inList = true
			continue
		}
		if inList {
			fields := strings.Fields(line)
			if len(fields) == 0 {
				break // 段落结束
			}
			names[fields[0]] = true
		}
	}
	return names
}
