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
	cfgFile     string
	githubProxy string
	workDir     string
)

var rootCmd = &cobra.Command{
	Use:   "somcli",
	Short: "Structure-Projects Container Management CLI",
	Long: `somcli is a unified management tool for container technologies including 
Docker, Docker Compose, Docker Swarm and Kubernetes.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// 初始化配置必须在所有命令执行前完成
		initConfig()
		// 设置工作目录
	},
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

	// 绑定viper
	viper.BindPFlag("github_proxy", rootCmd.PersistentFlags().Lookup("github-proxy"))
	viper.BindPFlag("workdir", rootCmd.PersistentFlags().Lookup("workdir"))
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
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
	verifyProxyConfig()
}

func verifyProxyConfig() {
	if proxy := viper.GetString("github_proxy"); proxy != "" {
		if !strings.HasPrefix(proxy, "http://") && !strings.HasPrefix(proxy, "https://") {
			utils.PrintError("GitHub代理地址格式错误，必须以http://或https://开头")
		}
	}
}
