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
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/structure-projects/somcli/pkg/utils"
)

var (
	cfgFile     string   // 配置信息
	githubProxy string   // github代理
	workDir     string   //工作目录
	debugMode   bool     // 新增debug模式标志
	offline     bool     // 是否离线模式
	source      []string // 镜像源，可逗号分隔或重复传
	setVars     []string // --set k=v，覆盖配置里的 vars
)
var rootCmd = &cobra.Command{
	Use:   "somcli",
	Short: "Structure-Projects Container Management CLI",
	Long: `somcli is a unified management tool for container technologies including 
Docker, Docker Compose, Docker Swarm and Kubernetes.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		applyGlobalFlags()
	},
}

// applyGlobalFlags 把全局标志的值真正落到运行期状态上。
//
// 单独抽出来是因为透传命令（DisableFlagParsing）要在 PersistentPreRun 之后才解析出
// 自己那份全局标志，解析完必须再调一次，否则 --workdir 解析了也不生效（D10）。
func applyGlobalFlags() {
	// 设置调试模式
	utils.SetDebugMode(debugMode)

	utils.SetOffline(offline)

	// --set 先于配置生效：applyGlobalSettings 读配置时只填 configVars 那一层，
	// 覆盖层在这里一次装好，两层的优先级就与读取顺序无关了。
	vars, err := parseSetVars(setVars)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	utils.SetOverrideVars(vars)

	// 初始化配置必须在所有命令执行前完成
	initConfig()
}

// parseSetVars 解析 --set k=v。
// 拒绝没有 = 的写法而不是忽略：静默忽略等于用户以为传进去了，模板里却渲染成报错或空值。
func parseSetVars(pairs []string) (map[string]string, error) {
	vars := make(map[string]string, len(pairs))
	for _, pair := range pairs {
		key, value, found := strings.Cut(pair, "=")
		if !found || key == "" {
			return nil, fmt.Errorf("--set 需要 k=v 形式，收到 %q", pair)
		}
		vars[key] = value
	}
	return vars, nil
}

func Execute() {
	// 添加所有子命令
	addSubcommands()

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func addSubcommands() {
	rootCmd.AddCommand(NewVersionCmd())
	rootCmd.AddCommand(NewDockerCmd())
	rootCmd.AddCommand(NewComposeCmd())
}

func init() {
	cobra.OnInitialize(initConfig)

	// 全局标志
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.somcli.yaml)")
	rootCmd.PersistentFlags().StringVar(&githubProxy, "github-proxy", "", "GitHub proxy URL (e.g. https://gh-proxy.com/)")
	rootCmd.PersistentFlags().StringVar(&workDir, "workdir", "", "working directory (default is ./somwork if exists, otherwise current directory)")
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "enable debug mode") // 新增debug标志
	// 帮助里一直写着"comma-separated or multiple flags"，类型却是 bool：绑到
	// mirrors_source 后 GetStringSlice 读回 ["false"]，于是每条命令都白跑一次 InitSource
	// 并打印一行"加载源 -> [false]"，而配置文件里真正的 mirrors_source 被它盖掉。
	rootCmd.PersistentFlags().StringSliceVar(&source, "source", nil, "Mirror sources (comma-separated or multiple flags)")
	rootCmd.PersistentFlags().BoolVar(&offline, "offline", false, "enable 离线模式") // 新增debug标志 Mirror source
	rootCmd.PersistentFlags().StringArrayVar(&setVars, "set", nil,
		"Set a template variable, repeatable (e.g. --set port=8080). Overrides vars: in the config file")

	// 绑定viper
	viper.BindPFlag("github_proxy", rootCmd.PersistentFlags().Lookup("github-proxy"))
	viper.BindPFlag("workdir", rootCmd.PersistentFlags().Lookup("workdir"))
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))           // 绑定debug到viper
	viper.BindPFlag("mirrors_source", rootCmd.PersistentFlags().Lookup("source")) // 绑定debug到viper
	viper.BindPFlag("offline", rootCmd.PersistentFlags().Lookup("offline"))       // 绑定debug到viper
}

func initConfig() {
	if cfgFile != "" {
		// 使用指定的配置文件
		viper.SetConfigFile(cfgFile)
	} else {
		// 搜索默认配置文件
		viper.AddConfigPath("$HOME")
		viper.SetConfigName(".somcli")
	}

	// 读取环境变量
	viper.AutomaticEnv()

	// 读取配置文件
	if err := viper.ReadInConfig(); err == nil {
		if utils.IsDebugMode() {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}

		// 新增：将配置解析到 utils.Config
		if err := viper.Unmarshal(&utils.Config); err != nil {
			utils.PrintError("Failed to parse config: %v", err)
		}

		if utils.IsDebugMode() {
			fmt.Printf("Loaded config: %+v\n", utils.Config)
		}
	}
	verifyProxyConfig()

	//初始化源
	if sourceList := viper.GetStringSlice("mirrors_source"); len(sourceList) > 0 {
		utils.InitSource(sourceList)
	}

}

func verifyProxyConfig() {
	if proxy := viper.GetString("github_proxy"); proxy != "" {
		if !strings.HasPrefix(proxy, "http://") && !strings.HasPrefix(proxy, "https://") {
			utils.PrintError("GitHub代理地址格式错误，必须以http://或https://开头")
		}
	}
}
