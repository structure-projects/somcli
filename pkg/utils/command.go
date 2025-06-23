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
	"os"
	"os/exec"
	"strings"

	"github.com/structure-projects/somcli/pkg/types"
)

func RunCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %v", err)
	}
	return nil
}

func RunCommandWithOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %v, output: %s", err, string(output))
	}
	return string(output), nil
}

// RunCommandInDir 在指定目录执行命令
func RunCommandInDir(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// RunCommandWithEnv 带环境变量执行本地命令
func RunCommandWithEnv(env map[string]string, name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 设置环境变量
	for k, v := range env {
		cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", k, v))
	}

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%s: %w\n%s", strings.Join(cmd.Args, " "), err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// CommandExists 检查命令是否存在
func CommandExists(name string) bool {
	_, err := exec.LookPath(name)

	return err == nil
}

// RunCommandOnNode 在节点上执行命令（改为接收指针）
func RunCommandOnNode(node *types.RemoteNode, command string) (string, error) {
	if node.Host == "localhost" || node.Host == "127.0.0.1" || node.IP == "127.0.0.1" {
		return RunCommandWithOutput("sh", "-c", command)
	}

	sshKey := ExpandPath(node.SSHKey)
	output, err := SSHMCmd(node.User, node.IP, sshKey, command)
	if err != nil {
		return "", fmt.Errorf("failed to execute command '%s' on node %s (%s@%s): %w",
			command, node.Host, node.User, node.IP, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// 运行脚本
func RunScripts(scripts []string, res types.Resource) error {

	for _, script := range scripts {
		runScript, err := ParseStr(script, res)
		if err != nil {
			PrintWarning("scripts parse err -> %v", err)
		}
		PrintDebug("exec scripts -> %s", runScript)
		//判断是否在本地执行
		if len(res.Hosts) > 0 {
			//远程执行
			for _, hostname := range res.Hosts {

				node := GetNode(hostname)
				PrintDebug("remote node %s", node.IP)
				out, err := RunCommandOnNode(&node, runScript)
				if err != nil {
					PrintDebug("err -> %v", err)
				}
				PrintInfo("exec remote -> node: %s ,scripts: %s ,scripts out ->\n%s", hostname, runScript, out)
			}
		} else {
			//本地执行
			out, err := RunCommandWithOutput("sh", "-c", runScript)
			if err != nil {
				PrintDebug("err -> %v", err)
			}
			PrintInfo("exec local scripts -> %s ,scripts out ->    \n%s", runScript, out)
		}

	}
	return nil
}
