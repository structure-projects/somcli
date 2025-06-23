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
package compose

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"

	"github.com/spf13/viper"
	"github.com/structure-projects/somcli/pkg/installer"
	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
)

// 常量定义
const (
	defaultInstallPath  = "/usr/local/bin/docker-compose"                                                                         // 默认安装路径
	downloadTemplateUrl = "https://github.com/docker/compose/releases/download/v{{.Version}}/docker-compose-{{.Platform}}-x86_64" // 下载模板URL
	applicationName     = "docker-comopose"                                                                                       // 应用名称
	defaultVersion      = "2.24.0"                                                                                                // 默认版本
)

// Installer 结构体用于管理 Docker Compose 的安装和卸载
type Installer struct {
	silent      bool         // 是否静默模式
	installPath string       // 安装路径
	Config      *viper.Viper // 配置
}

// NewComposeInstaller 创建一个新的 Installer 实例
func NewComposeInstaller(silent bool, config *viper.Viper) *Installer {
	utils.PrintInfo("创建新的 Docker Compose 安装器实例")
	return &Installer{
		silent:      silent,
		installPath: defaultInstallPath,
		Config:      config,
	}
}

// SetInstallPath 设置安装路径
func (i *Installer) SetInstallPath(path string) {
	utils.PrintDebug("设置安装路径为: %s", path)
	i.installPath = path
}

// Install 安装 Docker Compose
func (i *Installer) Install(version string) error {
	utils.PrintStage("开始安装 Docker Compose, 版本: %s", version)

	// 1. 检查是否已安装
	if i.isInstalled() {
		if !i.silent {
			utils.PrintInfo("Docker Compose 已安装在 %s", i.installPath)
		}
		return nil
	}

	// 使用统一的下载管理器
	utils.PrintInfo("开始下载 Docker Compose")
	// downloader := utils.NewDownloader(i.Config.GetString("github_proxy"))
	dockerComposeResource := types.Resource{
		Name:        applicationName,
		Version:     defaultVersion,
		URLs:        []string{downloadTemplateUrl},
		Target:      "{{.Name}}",
		PreInstall:  []string{"chmod +x {{}}"},
		PostInstall: []string{"chmod +x {{}}"},
	}

	// result := installer.DownloadSingleFile(downloader, dockerComposeResource, downloadTemplateUrl)

	installer := installer.NewInstaller()
	installer.Install(dockerComposeResource, i.silent)

	// if err := utils.CopyFile(result.LocalPath, i.installPath); err != nil {
	// 	utils.PrintError("安装 Docker Compose 失败: %v", err)
	// 	return fmt.Errorf("failed to install docker-compose: %v", err)
	// }

	// if err := os.Chmod(i.installPath, 0755); err != nil {
	// 	utils.PrintError("设置可执行权限失败: %v", err)
	// 	return fmt.Errorf("failed to set executable permissions: %v", err)
	// }

	// if !i.silent {
	// 	utils.PrintSuccess("Docker Compose %s 成功安装到 %s", version, i.installPath)
	// }
	return nil
}

// Uninstall 卸载 Docker Compose
func (i *Installer) Uninstall() error {
	utils.PrintStage("开始卸载 Docker Compose")
	if !i.isInstalled() {
		if !i.silent {
			utils.PrintInfo("Docker Compose 未安装")
		}
		return nil
	}

	if err := os.Remove(i.installPath); err != nil {
		utils.PrintError("卸载 Docker Compose 失败: %v", err)
		return fmt.Errorf("failed to uninstall docker-compose: %v", err)
	}

	if !i.silent {
		utils.PrintSuccess("Docker Compose 卸载成功")
	}
	return nil
}

// Version 获取已安装的 Docker Compose 版本
func (i *Installer) Version() (string, error) {
	utils.PrintDebug("检查 Docker Compose 版本")
	if !i.isInstalled() {
		utils.PrintWarning("Docker Compose 未安装")
		return "", fmt.Errorf("docker compose is not installed")
	}

	cmd := exec.Command(i.installPath, "version", "--short")
	output, err := cmd.CombinedOutput()
	if err != nil {
		utils.PrintError("获取版本失败: %v", err)
		return "", fmt.Errorf("failed to get version: %v", err)
	}

	version := strings.TrimSpace(string(output))
	utils.PrintDebug("获取到版本号: %s", version)
	return version, nil
}

// Passthrough 透传命令到 Docker Compose
func (i *Installer) Passthrough(args []string) error {
	utils.PrintDebug("透传命令到 Docker Compose，参数: %v", args)
	if !i.isInstalled() {
		if !i.silent {
			utils.PrintWarning("Docker Compose 未找到，尝试自动安装...")
		}
		if err := i.Install(defaultVersion); err != nil {
			utils.PrintError("自动安装失败: %v", err)
			return fmt.Errorf("auto-install failed: %v\nPlease install manually first", err)
		}
	}

	args = i.processArgs(args)
	execPath := i.installPath
	if runtime.GOOS == "windows" && !strings.HasSuffix(execPath, ".exe") {
		execPath += ".exe"
	}

	utils.PrintDebug("执行命令: %s %v", execPath, args)
	cmd := exec.Command(execPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()

	i.handleSignals(cmd)

	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			utils.PrintDebug("命令退出状态: %d", status.ExitStatus())
			os.Exit(status.ExitStatus())
		}
		return exitErr
	}
	return err
}

// processArgs 处理参数
func (i *Installer) processArgs(args []string) []string {
	utils.PrintDebug("处理命令参数，原始参数: %v", args)
	if len(args) > 0 {
		switch args[0] {
		case "up", "down", "restart":
			if !contains(args, "-d") && !contains(args, "--detach") {
				args = append(args, "-d")
				utils.PrintDebug("添加 -d 参数")
			}
		case "ps":
			if !contains(args, "-a") && !contains(args, "--all") {
				args = append(args, "-a")
				utils.PrintDebug("添加 -a 参数")
			}
		}
	}
	utils.PrintDebug("处理后的参数: %v", args)
	return args
}

// handleSignals 处理信号
func (i *Installer) handleSignals(cmd *exec.Cmd) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		utils.PrintDebug("接收到信号: %v", sig)
		if cmd.Process != nil {
			cmd.Process.Signal(sig)
		}
	}()
}

// isInstalled 检查是否已安装
func (i *Installer) isInstalled() bool {
	_, err := os.Stat(i.installPath)
	return !os.IsNotExist(err)
}

// contains 检查切片是否包含某字符串
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
