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
package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func CopyToRemote(user, ip, keyPath, localPath, remotePath string) error {
	// 示例配置一律写 ~/.ssh/id_rsa，而 scp / ssh 是被 exec 直接拉起的，没有 shell 帮忙展开 ~，
	// 不在这里展开就等于对所有人分发都失败。
	keyPath = ExpandPath(keyPath)

	// 检查文件是否存在
	exists, err := RemoteFileExists(user, ip, keyPath, remotePath)
	if err != nil {
		return fmt.Errorf("检查远程文件失败: %w", err)
	}
	if exists {
		// 验证远程文件和本地文件一致性
		PrintInfo("检查本地文件和远程文件hash值是否一致------------------------>")
		remoteChecksum, err := GetRemoteFileChecksum(user, ip, keyPath, remotePath)
		if err == nil {
			if err := VerifyChecksum(localPath, remoteChecksum); err == nil {
				PrintWarning("ℹ️ 文件已存在于 %s:%s，跳过复制\n", ip, remotePath)
				return nil
			}
		}
	} else {
		// 创建远程目录（路径用单引号包裹）
		SSHMkdir(user, ip, keyPath, filepath.Dir(remotePath))
	}

	// 执行 SCP（添加超时和详细日志）
	scpCmd := exec.Command("scp",
		"-i", keyPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=30",
		localPath,
		fmt.Sprintf("%s@%s:'%s'", user, ip, remotePath)) // 远程路径加单引号

	PrintDebug("执行 SCP 命令: %s", scpCmd)
	scpCmd.Stdout = os.Stdout
	scpCmd.Stderr = os.Stderr

	if err := scpCmd.Run(); err != nil {
		return fmt.Errorf("SCP 传输失败: %w (命令: %s)", err, scpCmd)
	}

	PrintInfo("📤 已复制 %s 到 %s:%s\n", localPath, ip, remotePath)
	return nil
}

func SSHMkdir(user, ip, keyPath, remotePath string, mode ...string) error {
	// 安全处理路径中的特殊字符（如空格、$等）
	safePath := fmt.Sprintf("'%s'", strings.ReplaceAll(remotePath, "'", "'\\''"))

	// 构建 mkdir 命令
	cmd := fmt.Sprintf("mkdir -p %s", safePath)

	// 如果指定了目录权限
	if len(mode) > 0 && mode[0] != "" {
		cmd += fmt.Sprintf(" && chmod %s %s", mode[0], safePath)
	}

	_, err := SSHMCmd(user, ip, keyPath, cmd)

	if err != nil {
		return fmt.Errorf("SSH目录创建失败: %w", err)
	}

	return nil
}

// 执行远程命令
func SSHMCmd(user, ip, keyPath, cmd string) (string, error) {

	// 构建 SSH 命令
	sshArgs := []string{
		"-i", ExpandPath(keyPath),
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=30",
		fmt.Sprintf("%s@%s", user, ip),
		cmd,
	}

	sshCmd := exec.Command("ssh", sshArgs...)
	PrintDebug("sshCmd -> %s", sshCmd)
	output, err := sshCmd.CombinedOutput()

	if err != nil {
		return string(output), fmt.Errorf("SSH执行失败远程主机:%s 远程命令: %w\n命令: %s\n输出: %s", ip,
			err,
			sshCmd.String(),
			string(output))
	}

	return string(output), nil
}

// remoteFileExists 检查远程文件是否存在
func RemoteFileExists(user, ip, keyPath, remotePath string) (bool, error) {
	checkCmd := fmt.Sprintf("test -f %s && echo exists || echo not_exists", remotePath)
	output, err := SSHMCmd(user, ip, keyPath, checkCmd)

	if err != nil {
		return false, fmt.Errorf("failed to check remote file: %v", err)
	}

	return strings.TrimSpace(output) == "exists", nil
}
