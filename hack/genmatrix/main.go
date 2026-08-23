// genmatrix 由命令树 + test/matrix.yaml 渲染 doc/00-功能清单与矩阵.md。
//
// 命令部分与 hack/gendoc 同源（都从 cmd.NewRootCommand 反射），场景状态从 matrix.yaml 取，
// 这样"测试守住了什么"与"文档声称支持什么"不可能悄悄分叉。
// 重新生成：go run ./hack/genmatrix；CI 会 git diff --exit-code。
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"

	"github.com/structure-projects/somcli/cmd"
)

type matrixFile struct {
	Scenarios []scenario `yaml:"scenarios"`
}

type scenario struct {
	ID      string   `yaml:"id"`
	Desc    string   `yaml:"desc"`
	Env     []string `yaml:"env"`
	Files   []string `yaml:"files"`
	Defects []string `yaml:"defects"`
	Phase   int      `yaml:"phase"`
	Status  string   `yaml:"status"`
}

type group struct {
	prefix string
	title  string
	items  []scenario
}

var groupHeader = regexp.MustCompile(`^#\s*-+\s*(SC-[A-Z])\s+(.+?)\s*-+\s*$`)

func main() {
	raw, err := os.ReadFile(filepath.Join("test", "matrix.yaml"))
	if err != nil {
		fatal(err)
	}
	var mf matrixFile
	if err := yaml.UnmarshalStrict(raw, &mf); err != nil {
		fatal(fmt.Errorf("解析 test/matrix.yaml: %w", err))
	}

	groups := scanGroups(string(raw))
	byPrefix := map[string]*group{}
	for i := range groups {
		byPrefix[groups[i].prefix] = &groups[i]
	}
	var orphans []scenario
	for _, s := range mf.Scenarios {
		p := prefix(s.ID)
		if g, ok := byPrefix[p]; ok {
			g.items = append(g.items, s)
		} else {
			orphans = append(orphans, s)
		}
	}

	out, err := os.Create(filepath.Join("doc", "00-功能清单与矩阵.md"))
	if err != nil {
		fatal(err)
	}
	defer out.Close()

	fmt.Fprintln(out, "<!-- 本文件由 hack/genmatrix 自动生成，请勿手改。改命令/场景状态后重新 go run ./hack/genmatrix -->")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "# 功能清单与矩阵")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "命令清单来自二进制的命令树，场景状态来自 `test/matrix.yaml`。两者都由代码/测试驱动，不手写。")
	fmt.Fprintln(out)

	writeCommandTree(out, cmd.NewRootCommand())

	fmt.Fprintln(out, "## 场景矩阵")
	fmt.Fprintln(out)
	done, pending := countStatus(mf.Scenarios)
	fmt.Fprintf(out, "共 %d 个场景：**%d done** / %d pending。pending 项由 CI 矩阵或真集群/多节点环境承载，转绿前不臆造为已验证。\n", len(mf.Scenarios), done, pending)
	fmt.Fprintln(out)

	for _, g := range groups {
		if len(g.items) == 0 {
			continue
		}
		fmt.Fprintf(out, "### %s %s\n\n", g.prefix, g.title)
		fmt.Fprintln(out, "| 编号 | 场景 | 环境 | 阶段 | 状态 |")
		fmt.Fprintln(out, "|---|---|---|---|---|")
		for _, s := range g.items {
			fmt.Fprintf(out, "| %s | %s | %s | %s | %s |\n",
				s.ID, escapeBar(s.Desc), strings.Join(s.Env, ", "), phaseStr(s.Phase), statusEmoji(s.Status))
		}
		fmt.Fprintln(out)
	}

	if len(orphans) > 0 {
		fmt.Fprintln(out, "### 未分组")
		fmt.Fprintln(out)
		for _, s := range orphans {
			fmt.Fprintf(out, "- %s %s (%s)\n", s.ID, s.Desc, s.Status)
		}
		fmt.Fprintln(out)
	}
}

func writeCommandTree(out *os.File, root *cobra.Command) {
	fmt.Fprintln(out, "## 命令清单")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "| 命令 | 简述 | 自有标志 |")
	fmt.Fprintln(out, "|---|---|---|")

	var walk func(c *cobra.Command, depth int)
	walk = func(c *cobra.Command, depth int) {
		name := c.CommandPath()
		short := strings.TrimSpace(c.Short)
		flags := c.LocalFlags()
		var flagNames []string
		flags.VisitAll(func(f *pflag.Flag) {
			flagNames = append(flagNames, "`--"+f.Name+"`")
		})
		flagCell := "—"
		if len(flagNames) > 0 {
			flagCell = strings.Join(flagNames, ", ")
		}
		fmt.Fprintf(out, "| %s | %s | %s |\n", name, escapeBar(short), flagCell)

		children := c.Commands()
		sort.Sort(byName(children))
		for _, child := range children {
			if child.IsAvailableCommand() && !child.IsAdditionalHelpTopicCommand() {
				walk(child, depth+1)
			}
		}
	}
	walk(root, 0)
	fmt.Fprintln(out)
}

// scanGroups 从 matrix.yaml 的 `# ---------- SC-X 标题 ----------` 注释里提取分组与顺序。
// 这样新增分组只要在 matrix.yaml 写一行注释，这里不必改。
func scanGroups(raw string) []group {
	var groups []group
	sc := bufio.NewScanner(bytes.NewReader([]byte(raw)))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		m := groupHeader.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		groups = append(groups, group{prefix: m[1], title: m[2]})
	}
	return groups
}

// prefix 取场景编号的分组前缀，形如 "SC-E01" -> "SC-E"。
func prefix(id string) string {
	if len(id) >= 4 && id[:3] == "SC-" {
		return "SC-" + id[3:4]
	}
	return ""
}

func countStatus(ss []scenario) (done, pending int) {
	for _, s := range ss {
		switch s.Status {
		case "done":
			done++
		default:
			pending++
		}
	}
	return
}

func phaseStr(p int) string {
	return fmt.Sprintf("M%d", p)
}

func statusEmoji(s string) string {
	switch s {
	case "done":
		return "done"
	default:
		return "pending"
	}
}

func escapeBar(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

type byName []*cobra.Command

func (a byName) Len() int           { return len(a) }
func (a byName) Less(i, j int) bool { return a[i].Name() < a[j].Name() }
func (a byName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "genmatrix:", err)
	os.Exit(1)
}
