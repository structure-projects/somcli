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
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/structure-projects/somcli/pkg/docker"
	"github.com/structure-projects/somcli/pkg/utils"
)

func NewDockerCmd() *cobra.Command {
	var (
		silent      bool
		offline     bool
		version     string
		nodesFile   string
		nodeIPs     []string
		nodeUser    string
		nodeSSHKey  string
		installPath string
		dataPath    string
	)

	rootCmd := &cobra.Command{
		Use:   "docker",
		Short: "Manage Docker installation and operations",
		Long: `The docker command provides installation, uninstallation and passthrough operations for Docker.
By default (without subcommands), it will install Docker on local machine.

For Docker commands, just use 'somcli docker [command]' to pass through to Docker CLI.`,
		Run: func(cmd *cobra.Command, args []string) {
			// 默认行为改为执行本地安装
			if len(args) == 0 {
				if !silent {
					fmt.Println("🔧 No subcommand provided, performing default local Docker installation")
				}

				installer := docker.NewInstaller(silent, offline)
				if err := installer.Install(version); err != nil {
					fmt.Printf("Error installing Docker: %v\n", err)
					os.Exit(1)
				}
				return
			}

			// 如果有参数则透传给Docker CLI
			installer := docker.NewInstaller(silent, offline)
			if err := installer.Passthrough(args); err != nil {
				fmt.Printf("Error executing docker command: %v\n", err)
				os.Exit(1)
			}
		},
	}

	// install 子命令
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install Docker engine",
		Long: `Install Docker engine on local or remote nodes.
Examples:
  # Install latest Docker on local machine (default behavior)
  somcli docker
  
  # Install specific version on local machine
  somcli docker install --version 20.10.12
  
  # Install on remote nodes specified in file
  somcli docker install -f nodes.yaml
  
  # Install on specific remote nodes
  somcli docker install --node 192.168.1.101,192.168.1.102 --user root --ssh-key ~/.ssh/id_rsa`,
		RunE: func(cmd *cobra.Command, args []string) error {
			installer := docker.NewInstaller(silent, offline)

			// 获取目标节点列表
			nodes, err := getTargetNodes(nodesFile, nodeIPs, nodeUser, nodeSSHKey)
			if err != nil {
				return err
			}

			// 设置安装路径
			if installPath == "" {
				installPath = filepath.Join(utils.GetDownloadDir(), "docker", version)
			}
			if dataPath == "" {
				dataPath = filepath.Join(utils.GetDataDir(), "docker")
			}

			return installer.Install(version, nodes...)
		},
	}

	// uninstall 子命令
	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Docker engine",
		Long: `Uninstall Docker engine from local or remote nodes.
Examples:
  # Uninstall Docker from local machine
  somcli docker uninstall
  
  # Uninstall from remote nodes specified in file
  somcli docker uninstall -f nodes.yaml
  
  # Uninstall from specific remote nodes
  somcli docker uninstall --node 192.168.1.101,192.168.1.102 --user root --ssh-key ~/.ssh/id_rsa`,
		RunE: func(cmd *cobra.Command, args []string) error {
			installer := docker.NewInstaller(silent, offline)

			// 获取目标节点列表
			nodes, err := getTargetNodes(nodesFile, nodeIPs, nodeUser, nodeSSHKey)
			if err != nil {
				return err
			}

			return installer.Uninstall(nodes...)
		},
	}

	// status 子命令
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Check Docker installation status",
		Long: `Check Docker installation status on local or remote nodes.
Examples:
  # Check Docker status on local machine
  somcli docker status
  
  # Check status on remote nodes specified in file
  somcli docker status -f nodes.yaml
  
  # Check status on specific remote nodes
  somcli docker status --node 192.168.1.101,192.168.1.102 --user root --ssh-key ~/.ssh/id_rsa`,
		RunE: func(cmd *cobra.Command, args []string) error {
			installer := docker.NewInstaller(silent, offline)

			// 获取目标节点列表
			nodes, err := getTargetNodes(nodesFile, nodeIPs, nodeUser, nodeSSHKey)
			if err != nil {
				return err
			}

			return installer.Status(nodes...)
		},
	}

	// 添加公共flags
	addCommonNodeFlags := func(cmd *cobra.Command) {
		cmd.Flags().StringVarP(&nodesFile, "file", "f", "", "Path to YAML file containing node configurations")
		cmd.Flags().StringSliceVar(&nodeIPs, "node", nil, "Comma-separated list of node IP addresses")
		cmd.Flags().StringVar(&nodeUser, "user", "root", "SSH user for remote nodes")
		cmd.Flags().StringVar(&nodeSSHKey, "ssh-key", "", "Path to SSH private key for remote nodes")
	}

	// 安装命令特有flags
	installCmd.Flags().StringVar(&version, "version", "latest", "Docker version to install")
	installCmd.Flags().StringVar(&installPath, "install-path", "", "Custom installation path")
	installCmd.Flags().StringVar(&dataPath, "data-path", "", "Custom data directory path")

	// 添加公共flags到子命令
	addCommonNodeFlags(installCmd)
	addCommonNodeFlags(uninstallCmd)
	addCommonNodeFlags(statusCmd)

	// 全局flags
	rootCmd.PersistentFlags().BoolVarP(&silent, "yes", "y", false, "Automatic yes to prompts")
	rootCmd.PersistentFlags().BoolVar(&offline, "offline", false, "Offline mode (no downloads)")

	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(statusCmd)

	return rootCmd
}

// getTargetNodes 获取目标节点列表
func getTargetNodes(nodesFile string, nodeIPs []string, user string, sshKey string) ([]docker.RemoteNode, error) {
	var nodes []docker.RemoteNode

	// 如果既没有指定节点文件也没有指定节点IP，则默认为本地节点
	if nodesFile == "" && len(nodeIPs) == 0 {
		return []docker.RemoteNode{{IsLocal: true}}, nil
	}

	// 从文件加载节点配置
	if nodesFile != "" {
		fileNodes, err := loadNodesFromFile(nodesFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load nodes from file: %v", err)
		}
		nodes = append(nodes, fileNodes...)
	}

	// 从命令行参数添加节点
	if len(nodeIPs) > 0 {
		for _, ip := range nodeIPs {
			nodes = append(nodes, docker.RemoteNode{
				IP:     ip,
				User:   user,
				SSHKey: sshKey,
			})
		}
	}

	return nodes, nil
}

// loadNodesFromFile 从YAML文件加载节点配置
func loadNodesFromFile(filePath string) ([]docker.RemoteNode, error) {
	// TODO: 实现YAML文件解析逻辑
	return nil, fmt.Errorf("YAML node config loading not implemented yet")
}
