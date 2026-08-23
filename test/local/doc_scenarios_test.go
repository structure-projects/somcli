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
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestSC_X12_ScenarioIdsDocumented 校验 test/matrix.yaml 里的每个场景 ID 都在人工文档中出现。
//
// 场景 ID 是验收清单与文档之间的锚点：一个场景被测试守住，就该在用户能读到的文档里
// 有对应说明，否则"可测"和"可用"之间永远隔着一段口述。doc/00 与 doc/11 是生成物
// （00 由 matrix 渲染，天然包含全部 ID），把它们算进来这条用例就永远绿、失去意义，
// 故只扫人工写的 doc/*.md 与 README.md，排除 提案-* 与 设计.md（历史记录）。
//
// 纯文本比对，不 import pkg/。
func TestSC_X12_ScenarioIdsDocumented(t *testing.T) {
	matrixIDs := scanMatrixIDs(t)
	if len(matrixIDs) == 0 {
		t.Fatal("没从 test/matrix.yaml 抽到任何场景 ID，抽取逻辑失效")
	}

	docText := readHumanDocs(t)
	found := map[string]bool{}
	for _, id := range scenarioIDRe.FindAllString(docText, -1) {
		found[id] = true
	}

	var missing []string
	for _, id := range matrixIDs {
		if !found[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("以下 %d 个场景 ID 在人工文档中找不到（去 doc/ 对应章节加锚点）：\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// TestSC_X13_NoInlineStandaloneConfigs 场景文档不得内联可独立运行的 somcli 配置。
//
// 顶层含 resources:/cluster:/images: 的 YAML 块本身就是一份能 `somcli install -f -`
// 的配置；贴在文档里会随实现漂移，用户照敲得到过期示例。可运行的完整配置一律落在
// configs/examples/ 下、文档按路径引用；讲解单个字段的节选要缩进，让人一眼看出它不是
// 独立文件。cluster.md 带历史滞后横幅，与 设计.md 同属历史记录，排除。
//
// 纯文本比对，不 import pkg/。
func TestSC_X13_NoInlineStandaloneConfigs(t *testing.T) {
	standaloneKey := regexp.MustCompile(`(?m)^(resources|cluster|images):\s*(#.*)?$`)

	for _, path := range humanDocFiles(t, "cluster.md") {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			lines := strings.Split(readFile(t, path), "\n")
			inBlock, isYAML, blockStart := false, false, 0
			for i, raw := range lines {
				line := strings.TrimSpace(raw)
				if !strings.HasPrefix(line, "```") {
					continue
				}
				if inBlock {
					if isYAML {
						block := strings.Join(lines[blockStart:i], "\n")
						if standaloneKey.MatchString(block) {
							t.Errorf("%s:%d 的 YAML 块出现顶层 resources:/cluster:/images: —— "+
								"这是一份可独立运行的配置，请放进 configs/examples/ 并按路径引用，"+
								"或把整段缩进成节选（行首加空格）。",
								path, blockStart+1)
						}
					}
					inBlock, isYAML = false, false
					continue
				}
				inBlock = true
				isYAML = strings.TrimPrefix(line, "```") == "yaml"
				blockStart = i + 1
			}
		})
	}
}

var (
	matrixIDRow  = regexp.MustCompile(`(?m)^\s*-\s*id:\s*(SC-[A-Z]\d+)\s*$`)
	scenarioIDRe = regexp.MustCompile(`SC-[A-Z]\d+`)
)

func scanMatrixIDs(t *testing.T) []string {
	t.Helper()
	raw := readFile(t, filepath.Join(repoRoot, "test", "matrix.yaml"))
	matches := matrixIDRow.FindAllStringSubmatch(raw, -1)
	seen := map[string]bool{}
	var ids []string
	for _, m := range matches {
		if !seen[m[1]] {
			seen[m[1]] = true
			ids = append(ids, m[1])
		}
	}
	return ids
}

// readHumanDocs 拼接所有人工文档文本。排除生成物与历史记录。
func readHumanDocs(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, path := range humanDocFiles(t) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", path, err)
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	return b.String()
}

// humanDocFiles 列出要参与文档校验的人工文档：README + doc/*.md，
// 排除生成物（00-/11-）、提案附录（提案-*）与历史记录（设计.md）。
// extraExclude 可再按文件名追加排除（如带滞后横幅的 cluster.md）。
func humanDocFiles(t *testing.T, extraExclude ...string) []string {
	t.Helper()
	skip := map[string]bool{"设计.md": true}
	for _, name := range extraExclude {
		skip[name] = true
	}

	paths := []string{filepath.Join(repoRoot, "README.md")}
	entries, err := os.ReadDir(filepath.Join(repoRoot, "doc"))
	if err != nil {
		t.Fatalf("读取 doc/ 失败: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		if strings.HasPrefix(name, "提案-") || skip[name] {
			continue
		}
		// 生成物：00 由 matrix 渲染（天然含全部 ID），11 由 cobra 生成。
		if strings.HasPrefix(name, "00-") || strings.HasPrefix(name, "11-") {
			continue
		}
		paths = append(paths, filepath.Join(repoRoot, "doc", name))
	}
	return paths
}
