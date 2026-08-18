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

// RunScripts 按顺序执行脚本，任一步失败即中止并返回错误。
//
// 失败必须向上传播：调用方（installer.Install）据此决定是否继续 post_install，
// 以及最终的退出码。曾经这里只打调试日志然后 return nil，导致失败的安装也打印
// "✓ 成功"，任何"测试通过"都不可信。
func RunScripts(scripts []string, res types.Resource) error {
	for idx, script := range scripts {
		runScript, err := ParseStr(script, res)
		if err != nil {
			return fmt.Errorf("第 %d 个脚本模板解析失败 (%s): %w", idx+1, script, err)
		}
		PrintDebug("exec scripts -> %s", runScript)

		// 未声明 hosts 时在操作机本地执行
		if len(res.Hosts) == 0 {
			out, err := RunCommandWithOutput("sh", "-c", runScript)
			PrintInfo("exec local scripts -> %s ,scripts out ->\n%s", runScript, out)
			if err != nil {
				return fmt.Errorf("本机执行第 %d 个脚本失败 (%s): %w", idx+1, runScript, err)
			}
			continue
		}

		for _, hostname := range res.Hosts {
			node, err := GetNode(hostname)
			if err != nil {
				return fmt.Errorf("第 %d 个脚本无法确定目标节点: %w", idx+1, err)
			}
			PrintDebug("remote node %s", node.IP)
			out, err := RunCommandOnNode(&node, runScript)
			PrintInfo("exec remote -> node: %s ,scripts: %s ,scripts out ->\n%s", hostname, runScript, out)
			if err != nil {
				return fmt.Errorf("节点 %s (%s) 执行第 %d 个脚本失败 (%s): %w",
					hostname, node.IP, idx+1, runScript, err)
			}
		}
	}
	return nil
}
