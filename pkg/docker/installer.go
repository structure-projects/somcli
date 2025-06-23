package docker

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
)

const (
	scriptURL  = "https://structured.oss-cn-beijing.aliyuncs.com/docker/2.4/docker-manager.sh"
	scriptName = "docker-manager.sh"
)

// Installer Docker安装器
type Installer struct {
	silent     bool
	offline    bool
	scriptPath string
}

func NewInstaller(silent, offline bool) *Installer {
	scriptPath := filepath.Join(utils.GetScriptDir(), scriptName)
	return &Installer{
		silent:     silent,
		offline:    offline,
		scriptPath: scriptPath,
	}
}

// ensureScript 确保脚本存在
func (i *Installer) ensureScript() error {
	if err := os.MkdirAll(filepath.Dir(i.scriptPath), 0755); err != nil {
		return fmt.Errorf("failed to create scripts directory: %v", err)
	}

	if _, err := os.Stat(i.scriptPath); os.IsNotExist(err) {
		if i.offline {
			return fmt.Errorf("script %s not found in offline mode", scriptName)
		}

		if !i.silent {
			fmt.Println("🔍 Downloading docker-manager script...")
		}

		resp, err := http.Get(scriptURL)
		if err != nil {
			return fmt.Errorf("failed to download script: %v", err)
		}
		defer resp.Body.Close()

		out, err := os.Create(i.scriptPath)
		if err != nil {
			return fmt.Errorf("failed to create script file: %v", err)
		}
		defer out.Close()

		if _, err := io.Copy(out, resp.Body); err != nil {
			return fmt.Errorf("failed to write script: %v", err)
		}

		if err := os.Chmod(i.scriptPath, 0755); err != nil {
			return fmt.Errorf("failed to set script permissions: %v", err)
		}
	}
	return nil
}

// runScriptCommand 执行脚本命令
func (i *Installer) runScriptCommand(args ...string) error {
	cmd := exec.Command(i.scriptPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runRemoteCommand 在远程节点执行命令
func (i *Installer) runRemoteCommand(node types.RemoteNode, command string) error {
	sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no %s@%s \"%s\"",
		node.SSHKey, node.User, node.IP, command)

	cmd := exec.Command("bash", "-c", sshCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if !i.silent {
		fmt.Printf("🔧 Executing on %s: %s\n", node.IP, command)
	}
	return cmd.Run()
}

// remoteFileExists 检查远程文件是否存在
func (i *Installer) remoteFileExists(node types.RemoteNode, remotePath string) (bool, error) {
	checkCmd := fmt.Sprintf("test -f %s && echo exists || echo not_exists", remotePath)
	output, err := exec.Command("bash", "-c",
		fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no %s@%s \"%s\"",
			node.SSHKey, node.User, node.IP, checkCmd)).Output()

	if err != nil {
		return false, fmt.Errorf("failed to check remote file: %v", err)
	}

	return strings.TrimSpace(string(output)) == "exists", nil
}

// copyToRemote 复制文件到远程节点（如果不存在）
func (i *Installer) copyToRemote(node types.RemoteNode, localPath, remotePath string) error {
	// 检查远程文件是否已存在
	exists, err := i.remoteFileExists(node, remotePath)
	if err != nil {
		return err
	}
	if exists {
		if !i.silent {
			fmt.Printf("ℹ️ File already exists on %s:%s, skipping copy\n", node.IP, remotePath)
		}
		return nil
	}

	scpCmd := fmt.Sprintf("scp -i %s -o StrictHostKeyChecking=no %s %s@%s:%s",
		node.SSHKey, localPath, node.User, node.IP, remotePath)

	cmd := exec.Command("bash", "-c", scpCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if !i.silent {
		fmt.Printf("📤 Copying %s to %s:%s\n", localPath, node.IP, remotePath)
	}
	return cmd.Run()
}

// checkRsyncVersion 检查远程rsync版本
func (i *Installer) checkRsyncVersion(node types.RemoteNode) (bool, error) {
	cmd := "rsync --version | head -1 | grep -oE '[0-9]+\\.[0-9]+\\.[0-9]+' || echo '1.0.0'"
	output, err := exec.Command("bash", "-c",
		fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no %s@%s \"%s\"",
			node.SSHKey, node.User, node.IP, cmd)).CombinedOutput()

	if err != nil {
		return false, fmt.Errorf("failed to check rsync version: %v", err)
	}

	version := strings.TrimSpace(string(output))
	parts := strings.Split(version, ".")
	if len(parts) < 3 {
		return false, nil
	}

	// 简单判断版本是否>=2.6.0
	if parts[0] > "2" || (parts[0] == "2" && parts[1] >= "6") {
		return true, nil
	}
	return false, nil
}

// copyDirectoryToRemote 兼容低版本的目录复制方法
func (i *Installer) copyDirectoryToRemote(node types.RemoteNode, localDir, remoteDir string) error {
	if !i.silent {
		fmt.Printf("📦 Copying directory %s to %s:%s\n", localDir, node.IP, remoteDir)
	}

	// 1. 确保远程目录存在
	if err := i.runRemoteCommand(node, fmt.Sprintf("mkdir -p %s", remoteDir)); err != nil {
		return fmt.Errorf("failed to create remote directory: %v", err)
	}

	// 2. 尝试使用rsync（如果可用）
	hasModernRsync, err := i.checkRsyncVersion(node)
	if err != nil && !i.silent {
		fmt.Printf("⚠️ Rsync version check failed: %v\n", err)
	}

	if hasModernRsync {
		// 使用rsync的兼容模式（避免使用可能不支持的参数）
		rsyncCmd := fmt.Sprintf("rsync -rlptD -e \"ssh -i %s -o StrictHostKeyChecking=no\" %s/ %s@%s:%s/",
			node.SSHKey,
			localDir,
			node.User,
			node.IP,
			remoteDir)

		cmd := exec.Command("bash", "-c", rsyncCmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err == nil {
			return nil
		} else if !i.silent {
			fmt.Printf("⚠️ Rsync failed, falling back to tar: %v\n", err)
		}
	}

	// 3. 回退到tar方法
	if !i.silent {
		fmt.Println("ℹ️ Using compatible tar-based directory copy")
	}

	tarCmd := fmt.Sprintf("tar -czf - -C %s . | ssh -i %s -o StrictHostKeyChecking=no %s@%s \"tar -xzf - -C %s\"",
		localDir,
		node.SSHKey,
		node.User,
		node.IP,
		remoteDir)

	cmd := exec.Command("bash", "-c", tarCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// prepareLocalInstallArgs 准备本地安装参数
func (i *Installer) prepareLocalInstallArgs(version string) []string {
	args := []string{}
	if i.silent {
		args = append(args, "-y")
	}
	if i.offline {
		args = append(args, "-o")
	}
	if version != "latest" {
		args = append(args, "-v", version)
	}
	args = append(args, "-p", filepath.Join(utils.GetDownloadDir(), "docker", version))
	args = append(args, "--data", filepath.Join(utils.GetDataDir(), "docker"))
	return args
}

// Install 安装Docker
func (i *Installer) Install(version string, nodes ...types.RemoteNode) error {
	if err := i.ensureScript(); err != nil {
		return err
	}

	// 默认本地安装
	if len(nodes) == 0 {
		nodes = []types.RemoteNode{{IsLocal: true}}
	}

	for _, node := range nodes {
		if !i.silent {
			fmt.Printf("🚀 Installing Docker (version: %s) on %s\n",
				version, node.IP)
		}

		var err error
		if node.IsLocal {
			err = i.runScriptCommand(i.prepareLocalInstallArgs(version)...)
		} else {
			err = i.installOnRemote(node, version)
		}

		if err != nil {
			return fmt.Errorf("failed to install Docker on %s: %v",
				node.IP, err)
		}

		if !i.silent {
			fmt.Printf("✅ Successfully installed Docker on %s\n",
				node.IP)
		}
	}
	return nil
}

// installOnRemote 在远程节点安装Docker
func (i *Installer) installOnRemote(node types.RemoteNode, version string) error {
	// 1. 准备本地路径
	localWorkDir := utils.GetWorkDir()
	localScriptPath := i.scriptPath
	localDockerDir := filepath.Join(utils.GetDownloadDir(), "docker", version)

	// 2. 计算远程路径 (保持与本地相同的相对路径)
	remoteWorkDir := utils.GetWorkDir()

	// 计算脚本相对路径
	relScriptPath, err := filepath.Rel(localWorkDir, localScriptPath)
	if err != nil {
		return fmt.Errorf("failed to calculate script relative path: %v", err)
	}
	remoteScriptPath := filepath.Join(remoteWorkDir, relScriptPath)
	remoteScriptDir := filepath.Dir(remoteScriptPath)

	// 准备Docker相关路径
	remoteDockerDir := filepath.Join(remoteWorkDir, "download", "docker", version)
	remoteDataDir := filepath.Join(remoteWorkDir, "data", "docker")

	// 3. 创建远程目录结构
	createDirsCmd := fmt.Sprintf("mkdir -p %s %s %s",
		remoteScriptDir, remoteDockerDir, remoteDataDir)
	if err := i.runRemoteCommand(node, createDirsCmd); err != nil {
		return fmt.Errorf("failed to create remote directories: %v", err)
	}

	// 4. 复制脚本到远程 (保持路径一致性)
	if err := i.copyToRemote(node, localScriptPath, remoteScriptPath); err != nil {
		return fmt.Errorf("failed to copy script to remote: %v", err)
	}

	// 5. 复制本地Docker文件（如果存在且需要）
	if _, err := os.Stat(localDockerDir); err == nil {
		if err := i.copyDirectoryToRemote(node, localDockerDir, remoteDockerDir); err != nil {
			return fmt.Errorf("failed to copy Docker files: %v", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check local Docker directory: %v", err)
	}

	// 6. 执行远程安装 (传递离线参数)
	args := []string{}
	if i.silent {
		args = append(args, "-y")
	}
	if i.offline {
		args = append(args, "-o")
	}
	if version != "latest" {
		args = append(args, "-v", version)
	}
	args = append(args, "-p", remoteDockerDir)
	args = append(args, "--data", remoteDataDir)

	installCmd := fmt.Sprintf("chmod +x %s && %s %s",
		remoteScriptPath, remoteScriptPath, strings.Join(args, " "))

	return i.runRemoteCommand(node, installCmd)
}

// Uninstall 卸载Docker
func (i *Installer) Uninstall(nodes ...types.RemoteNode) error {
	if err := i.ensureScript(); err != nil {
		return err
	}

	// 默认本地卸载
	if len(nodes) == 0 {
		nodes = []types.RemoteNode{{IsLocal: true}}
	}

	for _, node := range nodes {
		if !i.silent {
			fmt.Printf("🚨 Uninstalling Docker from %s\n", node.IP)
		}

		var err error
		if node.IsLocal {
			err = i.runScriptCommand(i.prepareUninstallArgs()...)
		} else {
			err = i.uninstallOnRemote(node)
		}

		if err != nil {
			return fmt.Errorf("failed to uninstall Docker from %s: %v",
				node.IP, err)
		}

		if !i.silent {
			fmt.Printf("✅ Successfully uninstalled Docker from %s\n",
				node.IP)
		}
	}
	return nil
}

// prepareUninstallArgs 准备卸载参数
func (i *Installer) prepareUninstallArgs() []string {
	args := []string{}
	if i.silent {
		args = append(args, "-y")
	}
	args = append(args, "-u")
	return args
}

// uninstallOnRemote 在远程节点卸载Docker
func (i *Installer) uninstallOnRemote(node types.RemoteNode) error {
	// 保持与本地相同的脚本路径结构
	localWorkDir := utils.GetWorkDir()
	relScriptPath, err := filepath.Rel(localWorkDir, i.scriptPath)
	if err != nil {
		return fmt.Errorf("failed to calculate script relative path: %v", err)
	}
	remoteScriptPath := filepath.Join(utils.GetWorkDir(), relScriptPath)

	// 检查远程脚本是否存在
	exists, err := i.remoteFileExists(node, remoteScriptPath)
	if err != nil {
		return fmt.Errorf("failed to check remote script: %v", err)
	}
	if !exists {
		// 如果脚本不存在，直接复制
		if err := i.copyToRemote(node, i.scriptPath, remoteScriptPath); err != nil {
			return fmt.Errorf("failed to copy uninstall script: %v", err)
		}
	}

	uninstallCmd := fmt.Sprintf("chmod +x %s && %s -y -u",
		remoteScriptPath, remoteScriptPath)
	return i.runRemoteCommand(node, uninstallCmd)
}

// Status 检查Docker状态
func (i *Installer) Status(nodes ...types.RemoteNode) error {
	if err := i.ensureScript(); err != nil {
		return err
	}

	// 默认检查本地状态
	if len(nodes) == 0 {
		nodes = []types.RemoteNode{{IsLocal: true}}
	}

	for _, node := range nodes {
		output, err := i.getDockerStatus(node)
		if err != nil {
			return fmt.Errorf("docker status check failed on %s: %v",
				node.IP, err)
		}

		if !i.silent {
			fmt.Printf("🐳 Docker Status on %s:\n", node.IP)
		}
		fmt.Println(string(output))
	}
	return nil
}

// getDockerStatus 获取Docker状态
func (i *Installer) getDockerStatus(node types.RemoteNode) ([]byte, error) {
	if node.IsLocal {
		return exec.Command(i.scriptPath, "-c").CombinedOutput()
	}

	// 保持与本地相同的脚本路径结构
	localWorkDir := utils.GetWorkDir()
	relScriptPath, err := filepath.Rel(localWorkDir, i.scriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate script relative path: %v", err)
	}
	remoteScriptPath := filepath.Join(utils.GetWorkDir(), relScriptPath)

	// 检查远程脚本是否存在
	exists, err := i.remoteFileExists(node, remoteScriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check remote script: %v", err)
	}
	if !exists {
		// 如果脚本不存在，直接复制
		if err := i.copyToRemote(node, i.scriptPath, remoteScriptPath); err != nil {
			return nil, fmt.Errorf("failed to copy status script: %v", err)
		}
	}

	statusCmd := fmt.Sprintf("chmod +x %s && %s -c", remoteScriptPath, remoteScriptPath)
	return exec.Command("bash", "-c",
		fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no %s@%s \"%s\"",
			node.SSHKey, node.User, node.IP, statusCmd)).CombinedOutput()
}

// Passthrough 透传命令给Docker
func (i *Installer) Passthrough(args []string, nodes ...types.RemoteNode) error {
	if err := i.ensureScript(); err != nil {
		return err
	}

	// 默认本地执行
	if len(nodes) == 0 {
		nodes = []types.RemoteNode{{IsLocal: true}}
	}

	for _, node := range nodes {
		if !i.silent {
			fmt.Printf("🔧 Running Docker command on %s: docker %s\n",
				node.IP, strings.Join(args, " "))
		}

		err := i.runDockerCommand(node, args)
		if err != nil {
			return fmt.Errorf("failed to execute Docker command on %s: %v",
				node.IP, err)
		}
	}
	return nil
}

// runDockerCommand 执行Docker命令
func (i *Installer) runDockerCommand(node types.RemoteNode, args []string) error {
	if node.IsLocal {
		return i.runScriptCommand(args...)
	}

	// 保持与本地相同的脚本路径结构
	localWorkDir := utils.GetWorkDir()
	relScriptPath, err := filepath.Rel(localWorkDir, i.scriptPath)
	if err != nil {
		return fmt.Errorf("failed to calculate script relative path: %v", err)
	}
	remoteScriptPath := filepath.Join(utils.GetWorkDir(), relScriptPath)

	// 检查远程脚本是否存在
	exists, err := i.remoteFileExists(node, remoteScriptPath)
	if err != nil {
		return fmt.Errorf("failed to check remote script: %v", err)
	}
	if !exists {
		// 如果脚本不存在，直接复制
		if err := i.copyToRemote(node, i.scriptPath, remoteScriptPath); err != nil {
			return fmt.Errorf("failed to copy Docker passthrough script: %v", err)
		}
	}

	cmd := fmt.Sprintf("chmod +x %s && %s %s",
		remoteScriptPath, remoteScriptPath, strings.Join(args, " "))
	return i.runRemoteCommand(node, cmd)
}
