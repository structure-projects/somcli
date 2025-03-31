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
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
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

	return session.Run(command)
}

// SSHCopy 复制文件到远程主机
func SSHCopy(user, host, keyPath string, content io.Reader, remotePath string) error {
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

	// 确保远程目录存在
	remoteDir := filepath.Dir(remotePath)
	mkdirCmd := fmt.Sprintf("mkdir -p %s", remoteDir)
	if _, err := session.CombinedOutput(mkdirCmd); err != nil {
		return fmt.Errorf("failed to create remote directory: %v", err)
	}

	// 使用scp协议复制文件
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()

		// 获取文件大小
		var buf bytes.Buffer
		size, _ := io.Copy(&buf, content)
		content = &buf

		// SCP协议头
		fmt.Fprintf(w, "C0644 %d %s\n", size, filepath.Base(remotePath))
		io.Copy(w, content)
		fmt.Fprint(w, "\x00")
	}()

	return session.Run(fmt.Sprintf("/usr/bin/scp -qt %s", remoteDir))
}

// getSSHConfig 创建SSH客户端配置
func getSSHConfig(user, keyPath string) (*ssh.ClientConfig, error) {
	// 读取私钥文件
	key, err := os.ReadFile(keyPath)
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

// IsCommandAvailable 检查命令是否可用
func IsCommandAvailable(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// SSHClient 创建SSH客户端连接
func SSHClient(user, host, keyPath string) (*ssh.Client, error) {
	key, err := os.ReadFile(keyPath)
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
