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
			// 查看帮助不得产生任何副作用。透传路径会在 compose 缺失时自动下载安装，
			// 所以帮助必须在触碰安装器之前拦掉。
			if isHelpRequest(cmd.Root().PersistentFlags(), args) {
				_ = cmd.Help()
				return
			}

			if envFile != "" {
				if _, err := os.Stat(envFile); err == nil {
					os.Setenv("COMPOSE_FILE", envFile)
				}
			}

			coomposeInstall := compose.NewComposeInstaller(silent, viper.GetViper())
			if installPath != "" {
				coomposeInstall.SetInstallPath(installPath)
			}

			filteredArgs := filterArgs(args, envFile != "")
			if err := coomposeInstall.Passthrough(filteredArgs); err != nil {
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

// isHelpRequest 判断这次调用是不是在要帮助。
//
// DisableFlagParsing 之下 cobra 连 somcli 自己的全局 flag 都不解析，而是一并塞进 args：
// `somcli --workdir /tmp/x docker-compose --help` 的 args[0] 是 --workdir 而不是 --help。
// 所以要先跳过头部属于 somcli 的全局 flag（全局 flag 只可能出现在子命令名之前），
// 再看第一个真正给 compose 的参数。只认这一个，避免误吞 `docker-compose logs -h`
// 这类本该透传下去的写法。
func isHelpRequest(globals *pflag.FlagSet, args []string) bool {
	for len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "help":
			return true
		}
		if !strings.HasPrefix(args[0], "-") {
			return false
		}

		name, _, hasValue := strings.Cut(strings.TrimLeft(args[0], "-"), "=")
		flag := globals.Lookup(name)
		if flag == nil {
			// 不是 somcli 的全局 flag，那就是 compose 自己的，透传。
			return false
		}
		args = args[1:]
		if !hasValue && flag.Value.Type() != "bool" && len(args) > 0 {
			args = args[1:] // 连它的取值一起跳过
		}
	}
	// 只给了全局 flag，等同于没给 compose 任何参数。
	return true
}

// todo env file 提取到root上
func filterArgs(args []string, hasEnvFile bool) []string {
	var filtered []string
	skipNext := false

	for i, arg := range args {
		if skipNext {
			skipNext = false
			continue
		}

		if strings.HasPrefix(arg, "-") {
			switch arg {
			case "-y", "--yes", "-p", "--proxy", "--path", "-e", "--env-file":
				skipNext = true
				continue
			}
		}

		filtered = append(filtered, args[i])
	}

	return filtered
}
