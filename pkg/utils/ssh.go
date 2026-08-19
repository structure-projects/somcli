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
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHExec 在远程主机上执行命令并返回输出
func SSHExec(user, host, keyPath, command string) ([]byte, error) {
	config, err := getSSHConfig(user, keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH config: %v", err)
	}

	// 建立SSH连接
	client, err := ssh.Dial("tcp", net.JoinHostPort(host, "22"), config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH server: %v", err)
	}
	defer client.Close()

	// 创建会话
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH session: %v", err)
	}
	defer session.Close()

	// 执行命令并捕获输出
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	err = session.Run(command)
	if err != nil {
		return nil, fmt.Errorf("command failed: %v\nStderr: %s", err, stderrBuf.String())
	}

	return stdoutBuf.Bytes(), nil
}

// SSHExecWithOutput 执行命令并实时输出结果
func SSHExecWithOutput(user, host, keyPath, command string) error {
	config, err := getSSHConfig(user, keyPath)
	if err != nil {
		return fmt.Errorf("failed to create SSH config: %v", err)
	}

	client, err := ssh.Dial("tcp", net.JoinHostPort(host, "22"), config)
	if err != nil {
		return fmt.Errorf("failed to dial SSH server: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %v", err)
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr
	session.Stdin = os.Stdin
	PrintDebug("执行远程命令")
	return session.Run(command)
}

// getSSHConfig 创建SSH客户端配置
func getSSHConfig(user, keyPath string) (*ssh.ClientConfig, error) {
	// 读取私钥文件
	key, err := os.ReadFile(ExpandPath(keyPath))
	if err != nil {
		return nil, fmt.Errorf("unable to read private key: %v", err)
	}

	// 解析私钥
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("unable to parse private key: %v", err)
	}

	return &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}, nil
}

// SSHClient 创建SSH客户端连接
func SSHClient(user, host, keyPath string) (*ssh.Client, error) {
	key, err := os.ReadFile(ExpandPath(keyPath))
	if err != nil {
		return nil, fmt.Errorf("unable to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("unable to parse private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	return client, nil
}

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

// RsyncCopy 兼容低版本的rsync实现
func RsyncCopy(keyPath, localPath, user, ip, remotePath string) error {
	// 基础参数（兼容大多数rsync版本）
	args := []string{
		"-rlpt",      // 等效于 -a 但不保留设备和特殊文件
		"-z",         // 压缩传输
		"-v",         // 详细输出
		"--partial",  // 断点续传（比 -P 更兼容）
		"--progress", // 显示进度（比 -P 更兼容）
	}

	// 仅当版本支持时添加checksum（3.0.0+）
	if rsyncVersionAtLeast("3.0.0") {
		args = append(args, "--checksum")
	} else {
		args = append(args, "-c") // 旧版本的校验和选项
	}

	// SSH配置（保持与SCP相同的安全设置）
	sshOpts := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no", ExpandPath(keyPath))
	args = append(args, "--rsh="+sshOpts)

	// 添加路径参数
	args = append(args, localPath)
	args = append(args, fmt.Sprintf("%s@%s:%s", user, ip, remotePath))

	rsyncCmd := exec.Command("rsync", args...)
	rsyncCmd.Stdout = os.Stdout
	rsyncCmd.Stderr = os.Stderr

	return rsyncCmd.Run()
}

// 检查rsync版本是否>=指定版本
func rsyncVersionAtLeast(minVersion string) bool {
	out, err := exec.Command("rsync", "--version").Output()
	if err != nil {
		return false // 保守策略，假设版本较低
	}

	// 解析版本号（示例输出："rsync  version 2.6.9"）
	var version string
	if _, err := fmt.Sscanf(string(out), "rsync version %s", &version); err != nil {
		return false
	}

	return compareVersions(version, minVersion) >= 0
}

// 简单的版本号比较
func compareVersions(v1, v2 string) int {
	// 简化的比较逻辑，实际使用时建议完善
	if v1 == v2 {
		return 0
	}
	if v1 > v2 {
		return 1
	}
	return -1
}
