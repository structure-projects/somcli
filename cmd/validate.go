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

package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
	"gopkg.in/yaml.v2"
)

var validateConfigFile string

// validateCmd 只解析不执行：走 install 的同一条加载与渲染路径，但一步动作都不做。
// 有了它，「配置写对了吗」才能在动手装之前回答，而不是装到一半才报错。
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a config file without installing anything",
	Long: `Parse the config file and render every template it contains, then report.

Reads only: no download, no script execution, no file is written.
Exit code is non-zero if the config would fail at install time.`,
	Example: `  # Check a config before installing
  somcli validate -f configs/tools.yaml`,
	Run: runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
	validateCmd.Flags().StringVarP(&validateConfigFile, "file", "f", "", "Config file to validate (required)")
	_ = validateCmd.MarkFlagRequired("file")
}

func runValidate(cmd *cobra.Command, args []string) {
	problems, summary, err := validateFile(validateConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	for _, p := range problems {
		fmt.Fprintf(os.Stderr, "  ✗ %s\n", p)
	}
	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "\n%s: %d 处问题\n", validateConfigFile, len(problems))
		os.Exit(1)
	}

	fmt.Printf("%s: %s\n", validateConfigFile, summary)
}

// validateFile 一份配置一次验完：resources / cluster / images 各段都查，
// 不按"这是集群配置还是安装配置"分流 —— 统一 schema 下同一份文件本来就可能全都有。
func validateFile(path string) (problems []string, summary string, err error) {
	docs, err := countDocuments(path)
	if err != nil {
		return nil, "", err
	}

	// yaml.Unmarshal 只认第一个文档，`---` 之后的内容会被静默丢掉。
	// 与其让用户以为配了、其实从未生效，不如在这里直接说不支持。
	if docs > 1 {
		return nil, "", fmt.Errorf("%s 含 %d 个 YAML 文档，当前的加载器只读第一个，`---` 之后的内容不会生效", path, docs)
	}

	config, err := utils.LoadConfig(path)
	if err != nil {
		return nil, "", err
	}

	problems = append(problems, validateResources(config.Resources)...)
	problems = append(problems, validateClusters(config.Clusters)...)

	return problems, fmt.Sprintf("%d 个资源，%d 个节点，%d 套集群，%d 个镜像，全部可解析",
		len(config.Resources), len(config.Nodes), len(config.Clusters), len(config.Images)), nil
}

// validateClusters 集群段的必填项。名字重复了 --cluster-name 就选不出来，所以也算问题。
func validateClusters(clusters []types.ClusterSpec) []string {
	var problems []string
	seen := map[string]bool{}

	for i, c := range clusters {
		where := fmt.Sprintf("cluster[%d]", i)
		if c.Name != "" {
			where = fmt.Sprintf("cluster[%d] (%s)", i, c.Name)
		}

		switch c.Type {
		case "k8s", "swarm":
		case "":
			problems = append(problems, where+": type 为空（k8s / swarm）")
		default:
			problems = append(problems, fmt.Sprintf("%s: type %q 不支持（k8s / swarm）", where, c.Type))
		}
		if len(c.Nodes) == 0 {
			problems = append(problems, where+": nodes 为空，没有可操作的节点")
		}
		if c.Name == "" {
			problems = append(problems, where+": name 为空，多套集群时无法用 --cluster-name 选中")
		} else if seen[c.Name] {
			problems = append(problems, fmt.Sprintf("%s: 集群名重复", where))
		}
		seen[c.Name] = true
	}

	return problems
}

// countDocuments 数一下文件里有几个 YAML 文档。
func countDocuments(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	docs := 0
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var doc map[string]interface{}
		if err := dec.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return 0, fmt.Errorf("解析配置 %s 失败: %w", path, err)
		}
		docs++
	}
	return docs, nil
}

// validateResources 把每个资源的模板字段都真渲染一遍。
// 只在这里报"渲染不出来"，不判断 URL 可达或脚本对不对 —— 那要真跑才知道。
func validateResources(resources []types.Resource) []string {
	var problems []string

	for i, res := range resources {
		where := fmt.Sprintf("resources[%d]", i)
		if res.Name != "" {
			where = fmt.Sprintf("resources[%d] (%s)", i, res.Name)
		}
		if res.Name == "" {
			problems = append(problems, where+": name 为空")
		}

		for j, url := range res.URLs {
			rendered, err := utils.ParseStr(url, res)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s: urls[%d] 渲染失败: %v", where, j, err))
				continue
			}
			// target 的可用变量比别处多（Filename / Ext 由 URL 推出），所以得逐个 URL 验。
			if res.Target != "" {
				if _, err := utils.ParseTargetPath(res.Target, rendered, res); err != nil {
					problems = append(problems, fmt.Sprintf("%s: target 按 urls[%d] 渲染失败: %v", where, j, err))
				}
			}
		}
		// 没有 URL 时 target 仍要能单独渲染出来。
		if res.Target != "" && len(res.URLs) == 0 {
			if _, err := utils.ParseTargetPath(res.Target, "", res); err != nil {
				problems = append(problems, fmt.Sprintf("%s: target 渲染失败: %v", where, err))
			}
		}

		for _, group := range []struct {
			field   string
			scripts []string
		}{
			{"pre_install", res.PreInstall},
			{"post_install", res.PostInstall},
			{"remove_scripts", res.RemoveScripts},
		} {
			for j, script := range group.scripts {
				if _, err := utils.ParseStr(script, res); err != nil {
					problems = append(problems,
						fmt.Sprintf("%s: %s[%d] 渲染失败: %v", where, group.field, j, err))
				}
			}
		}
	}

	return problems
}
