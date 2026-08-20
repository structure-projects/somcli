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
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/structure-projects/somcli/pkg/compose"
)

func NewComposeCmd() *cobra.Command {
	var (
		silent      bool
		installPath string
		proxy       string
		envFile     string
	)

	rootCmd := &cobra.Command{
		Use:     "docker-compose",
		Aliases: []string{"compose", "dc"},
		Short:   "Enhanced Docker Compose management",
		Long: `Enhanced Docker Compose wrapper with additional features:

* Auto-install if not present
* Smart command defaults (e.g. 'up' defaults to -d)
* Proxy support for installation
* Environment file support
* Native signal handling

Examples:
  somcli docker-compose up          # Auto -d
  somcli docker-compose ps          # Auto -a
  somcli docker-compose logs -f     # Passthrough with signals
  somcli docker-compose --env-file .env.prod up`,
		// 透传模式需要 DisableFlagParsing，否则 -d/-f 等会被 cobra 抢走。
		// 代价是 --help 也变成透传参数，因此必须在 Run 里显式拦截，见下。
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			// DisableFlagParsing 之下 cobra 既不解析 somcli 自己的 flag，也不把它们从 args 里
			// 摘掉，于是 `somcli --workdir /tmp/x docker-compose up` 既丢了 --workdir，
			// 又把它连值一起透传给 docker compose（D10）。这里自己切一刀。
			own := []*pflag.FlagSet{cmd.Root().PersistentFlags(), cmd.Flags()}
			mine, rest := splitOwnFlags(own, args)

			// 查看帮助不得产生任何副作用。透传路径会在 compose 缺失时自动下载安装，
			// 所以帮助必须在触碰安装器之前拦掉。
			if isHelpRequest(rest) {
				_ = cmd.Help()
				return
			}

			if err := parseOwnFlags(own, mine); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			// 全局标志是刚才才解析出来的，PersistentPreRun 那一遍用的是默认值
			applyGlobalFlags()

			if envFile != "" {
				if _, err := os.Stat(envFile); err != nil {
					fmt.Fprintf(os.Stderr, "Error: --env-file %s 不可读: %v\n", envFile, err)
					os.Exit(1)
				}
				// 原实现把它写进 COMPOSE_FILE，那是"编排文件"而不是"环境文件"，
				// compose 会拿 .env 当 yaml 解析。正确做法是原样转交给 compose
				rest = append([]string{"--env-file", envFile}, rest...)
			}

			coomposeInstall := compose.NewComposeInstaller(silent, viper.GetViper())
			if installPath != "" {
				coomposeInstall.SetInstallPath(installPath)
			}

			if err := coomposeInstall.Passthrough(rest); err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	rootCmd.Flags().StringVarP(&envFile, "env-file", "e", "", "Specify an alternate environment file")
	rootCmd.PersistentFlags().BoolVarP(&silent, "yes", "y", false, "Automatic yes to prompts")
	rootCmd.PersistentFlags().StringVar(&installPath, "path", "", "Custom installation path")
	// rootCmd.PersistentFlags().StringVarP(&proxy, "proxy", "p", "", "Proxy server for installation")
	addDockerComposeSubcommands(rootCmd, &silent, &installPath, &proxy)

	return rootCmd
}

func addDockerComposeSubcommands(rootCmd *cobra.Command, silent *bool, installPath *string, proxy *string) {
	installCmd := &cobra.Command{
		Use:   "install [version]",
		Short: "Install Docker Compose",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			version := "2.24.0"
			if len(args) > 0 {
				version = args[0]
			}

			installer := compose.NewComposeInstaller(*silent, viper.GetViper())
			if *installPath != "" {
				installer.SetInstallPath(*installPath)
			}
			return installer.Install(version)
		},
	}

	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Docker Compose",
		RunE: func(cmd *cobra.Command, args []string) error {
			composeInstaller := compose.NewComposeInstaller(*silent, viper.GetViper())
			if *installPath != "" {
				composeInstaller.SetInstallPath(*installPath)
			}
			return composeInstaller.Uninstall()
		},
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Show Docker Compose version",
		RunE: func(cmd *cobra.Command, args []string) error {
			composeInstaller := compose.NewComposeInstaller(*silent, viper.GetViper())
			if *installPath != "" {
				composeInstaller.SetInstallPath(*installPath)
			}

			version, err := composeInstaller.Version()
			if err != nil {
				return err
			}

			fmt.Printf("Docker Compose version: %s\n", version)
			return nil
		},
	}

	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(versionCmd)
}

// splitOwnFlags 把头部属于 somcli 的 flag 与要透传给 docker compose 的参数分开。
//
// 判据：全局 flag 只可能出现在子命令名之前，所以从头扫，遇到第一个"不是 somcli 认识的
// flag"的 token 就停，其后一律原样交给下游。这条边界不能省 ——
// `docker-compose -p myproj up` 里的 -p 是 compose 的项目名，原实现按名字硬摘一批
// flag（含 -p / -e），项目名与 `exec -e K=V` 都会被吞掉。
func splitOwnFlags(sets []*pflag.FlagSet, args []string) (mine, rest []string) {
	for len(args) > 0 {
		tok := args[0]
		if !strings.HasPrefix(tok, "-") || tok == "-" || tok == "--" {
			break
		}

		name, _, hasValue := strings.Cut(strings.TrimLeft(tok, "-"), "=")
		flag := lookupOwnFlag(sets, name, strings.HasPrefix(tok, "--"))
		if flag == nil {
			break
		}

		mine = append(mine, tok)
		args = args[1:]
		if !hasValue && flag.Value.Type() != "bool" && len(args) > 0 {
			mine = append(mine, args[0]) // 连它的取值一起带走
			args = args[1:]
		}
	}
	return mine, args
}

// lookupOwnFlag 在 somcli 自己的几个 flagset 里查这个名字：长写法查全名，短写法查简写。
// 查不到就当成下游的参数 —— 认不出时倾向透传，而不是吞掉。
func lookupOwnFlag(sets []*pflag.FlagSet, name string, long bool) *pflag.Flag {
	for _, set := range sets {
		if long {
			if f := set.Lookup(name); f != nil {
				return f
			}
			continue
		}
		if len(name) == 1 {
			if f := set.ShorthandLookup(name); f != nil {
				return f
			}
		}
	}
	return nil
}

// parseOwnFlags 让切出来的那批 flag 真正生效。
func parseOwnFlags(sets []*pflag.FlagSet, args []string) error {
	if len(args) == 0 {
		return nil
	}

	// AddFlagSet 加进来的是同一批 *pflag.Flag，解析写的就是原来那些变量
	merged := pflag.NewFlagSet("somcli-own", pflag.ContinueOnError)
	merged.SetOutput(io.Discard)
	for _, set := range sets {
		merged.AddFlagSet(set)
	}
	return merged.Parse(args)
}

// isHelpRequest 判断这次调用是不是在要帮助。args 是 splitOwnFlags 切完之后
// 真正给 compose 的那部分，所以只看第一个 token：`docker-compose logs -h` 里的 -h
// 本该透传下去，不算帮助请求。
func isHelpRequest(args []string) bool {
	if len(args) == 0 {
		// 只给了 somcli 自己的 flag，等同于没给 compose 任何参数
		return true
	}
	switch args[0] {
	case "-h", "--help", "help":
		return true
	}
	return false
}
