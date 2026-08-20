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
	"path/filepath"
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
	defaultInstallPath = "/usr/local/bin/docker-compose" // 默认安装路径
	applicationName    = "docker-compose"                // 应用名称（缓存目录用它，拼错就换一个目录）
	defaultVersion     = "2.24.0"                        // 默认版本

	// downloadTemplateUrl 用 {{.UnameArch}} 而不是硬编码 x86_64：
	// docker compose 的 release 资产按 uname -m 命名（x86_64 / aarch64），
	// 硬编码等于在 arm64 机器上必然 404（F13）
	downloadTemplateUrl = "https://github.com/docker/compose/releases/download/v{{.Version}}/docker-compose-{{.Platform}}-{{.UnameArch}}"
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
//
// 走 method: binary —— 下载、落盘、chmod、失败传播都用 installer 那一套，
// compose 这边不再有自己的一条平行实现（原实现丢掉了 installer.Install 的返回值，
// 于是"下载失败"也照样打印安装成功，且从没往 installPath 放过任何东西：D8/F13）
func (i *Installer) Install(version string) error {
	if version == "" {
		version = defaultVersion
	}
	// URL 模板自带 v，入参写 v2.24.0 或 2.24.0 都得能用
	version = strings.TrimPrefix(version, "v")

	utils.PrintStage("开始安装 Docker Compose, 版本: %s", version)

	if i.isInstalled() {
		if !i.silent {
			utils.PrintInfo("Docker Compose 已安装在 %s", i.installPath)
		}
		return nil
	}

	// Target 取安装路径的文件名：缓存里的名字就是最终落地的名字，
	// 否则装出来的会是 docker-compose-linux-x86_64
	res := types.Resource{
		Name:       applicationName,
		Version:    version,
		URLs:       []string{downloadTemplateUrl},
		Target:     filepath.Base(i.installPath),
		Method:     "binary",
		InstallDir: filepath.Dir(i.installPath),
	}

	if err := installer.NewInstaller().Install(res, i.silent); err != nil {
		utils.PrintError("安装 Docker Compose 失败: %v", err)
		return fmt.Errorf("failed to install docker-compose: %w", err)
	}

	if err := i.verify(version); err != nil {
		utils.PrintError("校验 Docker Compose 安装结果失败: %v", err)
		return err
	}

	if !i.silent {
		utils.PrintSuccess("Docker Compose %s 成功安装到 %s", version, i.installPath)
	}
	return nil
}

// verify 确认装出来的东西真的在那儿、真的能跑、真的是要的那个版本。
//
// 缺了这一步，"报告成功但产物不存在"这类错就只能等用户下次执行 compose 命令时才暴露。
func (i *Installer) verify(want string) error {
	if !utils.IsExecutable(i.installPath) {
		return fmt.Errorf("安装后 %s 不存在或没有可执行权限", i.installPath)
	}

	got, err := i.Version()
	if err != nil {
		return fmt.Errorf("安装后无法执行 %s: %w", i.installPath, err)
	}

	if strings.TrimPrefix(got, "v") != want {
		return fmt.Errorf("安装后版本不符：期望 %s，实际 %s", want, got)
	}
	return nil
}

// Uninstall 卸载 Docker Compose
func (i *Installer) Uninstall() error {
	utils.PrintStage("开始卸载 Docker Compose")
	// 判据用"文件存在"而不是 isInstalled：装坏了（存在但没有可执行位）也得能清掉
	if !utils.FileExists(i.installPath) {
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
//
// `-d` 只补给 up：down 与 restart 都没有这个标志，补上去等于让透传必然失败
// （`somcli docker-compose down` → "unknown shorthand flag: 'd'"）
func (i *Installer) processArgs(args []string) []string {
	utils.PrintDebug("处理命令参数，原始参数: %v", args)
	// 子命令不一定在 args[0]：`--env-file .env up` 这种前面还有 compose 的全局标志
	switch subcommandOf(args) {
	case "up":
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
	utils.PrintDebug("处理后的参数: %v", args)
	return args
}

// subcommandOf 找出第一个不是标志的 token。
// 只认 compose 自己那批带值的全局标志，认不出就当它是子命令 —— 猜错的方向是"不补默认标志"，
// 比"把 --env-file 的取值当成子命令"安全。
func subcommandOf(args []string) string {
	takesValue := map[string]bool{
		"--env-file": true, "--file": true, "-f": true,
		"--project-name": true, "-p": true, "--project-directory": true,
		"--profile": true, "--ansi": true, "--parallel": true, "--progress": true,
	}
	for idx := 0; idx < len(args); idx++ {
		tok := args[idx]
		if !strings.HasPrefix(tok, "-") {
			return tok
		}
		if takesValue[tok] {
			idx++ // 跳过它的取值
		}
	}
	return ""
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
//
// 判据是"存在且可执行"。原实现写的是 !os.IsNotExist(err)：权限不足之类的非 ENOENT 错误
// 会被读成"已安装"，然后直接去 exec 一个读不到的路径
func (i *Installer) isInstalled() bool {
	return utils.IsExecutable(i.installPath)
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
