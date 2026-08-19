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
	"sync"

	"github.com/spf13/viper"
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
	outcome, err := RunScriptsDetailed(scripts, res)
	if err != nil {
		return err
	}
	return outcome.Err()
}

// TargetError 记录某个目标上的失败。Host 为空串表示操作机本地。
type TargetError struct {
	Host string
	Err  error
}

// RunOutcome 一次脚本执行的逐目标结果。
//
// 之所以不只返回一个 error：多节点下"3 个节点里 1 个失败"与"全都失败"是两种局面，
// on_error: continue 要求前者能带着剩下 2 个节点接着装，而调用方只有拿到
// 失败目标的名单才知道后续阶段该在谁身上继续（SC-F05）。
type RunOutcome struct {
	Failed []TargetError
}

// AliveTargets 从给定目标里剔除本次失败的，返回还能继续的目标。
func (o RunOutcome) AliveTargets(targets []string) []string {
	alive := make([]string, 0, len(targets))
	for _, target := range targets {
		failed := false
		for _, f := range o.Failed {
			if f.Host == target {
				failed = true
				break
			}
		}
		if !failed {
			alive = append(alive, target)
		}
	}
	return alive
}

// Err 把逐目标失败汇总成一个错误，全成功则返回 nil。
// 单目标时直接返回原错误，避免给本机执行的场景平白加一层"节点 xx:"。
func (o RunOutcome) Err() error {
	switch len(o.Failed) {
	case 0:
		return nil
	case 1:
		return o.Failed[0].Err
	}
	parts := make([]string, 0, len(o.Failed))
	for _, f := range o.Failed {
		parts = append(parts, fmt.Sprintf("  - %s: %v", TargetLabel(f.Host), f.Err))
	}
	return fmt.Errorf("%d 个目标失败:\n%s", len(o.Failed), strings.Join(parts, "\n"))
}

// ScriptTargets 返回资源的实施目标列表。
// 未声明 hosts 时返回单个空串，表示"在操作机本地实施" —— 与远程目标共用一套
// 逐目标逻辑，幂等判据与失败隔离就不必为本机再写一遍。
func ScriptTargets(res types.Resource) []string {
	if len(res.Hosts) == 0 {
		return []string{""}
	}
	return res.Hosts
}

// TargetLabel 返回目标的可读名，空串（操作机本地）显示为"本机"。
func TargetLabel(host string) string {
	if host == "" {
		return "本机"
	}
	return host
}

// resolvedTarget 是解析好节点信息的实施目标。node 为 nil 表示操作机本地。
type resolvedTarget struct {
	name string
	node *types.RemoteNode
}

// resolveTargets 一次性把目标名解析成节点。
//
// 解析失败一律是致命错误，不受 on_error: continue 影响：hosts 里写了个不存在的
// 主机名是配置写错，对所有目标、所有脚本都成立，降级成"跳过这个节点"会让一次
// 拼写错误表现为"装好了，只是少了一台"。
func resolveTargets(targets []string) ([]resolvedTarget, error) {
	resolved := make([]resolvedTarget, 0, len(targets))
	for _, target := range targets {
		if target == "" {
			resolved = append(resolved, resolvedTarget{})
			continue
		}
		node, err := GetNode(target)
		if err != nil {
			return nil, fmt.Errorf("无法确定目标节点: %w", err)
		}
		resolved = append(resolved, resolvedTarget{name: target, node: &node})
	}
	return resolved, nil
}

// failurePolicy 解析 on_error。
// 未知取值报错而不是当成默认值：静默当 abort 处理的话，写了 on_error: coninue 的用户
// 会以为容错开着，直到某次真的失败才发现整条流水线停了。
func failurePolicy(res types.Resource) (continueOnError bool, err error) {
	switch res.OnError {
	case "", "abort":
		return false, nil
	case "continue":
		return true, nil
	default:
		return false, fmt.Errorf("资源 %s 的 on_error 取值 %q 无法识别，可用：abort（默认）/ continue",
			res.Name, res.OnError)
	}
}

// parallelLimit 读 --parallel。<=1 一律走串行代码路径，
// 于是单节点（绝大多数场景）的行为完全不受并发实现影响。
func parallelLimit() int {
	if n := viper.GetInt("parallel"); n > 1 {
		return n
	}
	return 1
}

// RunScriptsDetailed 逐目标执行脚本，返回哪些目标失败了。
//
// 返回值里的 error 是**致命错误**（模板解析失败、节点解析失败）—— 它们不归属于
// 某个具体目标，也不该被 on_error: continue 放过。逐目标的失败在 RunOutcome 里。
func RunScriptsDetailed(scripts []string, res types.Resource) (RunOutcome, error) {
	var outcome RunOutcome
	if len(scripts) == 0 {
		return outcome, nil
	}

	continueOnError, err := failurePolicy(res)
	if err != nil {
		return outcome, err
	}
	alive, err := resolveTargets(ScriptTargets(res))
	if err != nil {
		return outcome, err
	}
	limit := parallelLimit()

	for idx, script := range scripts {
		runScript, err := ParseStr(script, res)
		if err != nil {
			return outcome, fmt.Errorf("第 %d 个脚本模板解析失败 (%s): %w", idx+1, script, err)
		}
		PrintDebug("exec scripts -> %s", runScript)

		results := fanOut(alive, limit, runScript)

		// 输出按目标声明序打印：并发下各节点是乱序完成的，照完成顺序打日志
		// 会让两次相同的运行产出不同的输出，"结果与串行一致"就无从断言（SC-E14）
		survivors := make([]resolvedTarget, 0, len(alive))
		failedThisStep := 0
		for i, r := range results {
			printScriptOutput(alive[i], runScript, r.out)
			if r.err == nil {
				survivors = append(survivors, alive[i])
				continue
			}
			failedThisStep++
			outcome.Failed = append(outcome.Failed, TargetError{
				Host: alive[i].name,
				Err:  scriptError(alive[i], idx, runScript, r.err),
			})
		}

		if failedThisStep == 0 {
			continue
		}
		if !continueOnError {
			return outcome, nil
		}
		// 失败的目标不再参与后续脚本：带着一台已经出错的机器往下装，
		// 只会把一个能定位的错误摊成一串派生错误
		alive = survivors
		if len(alive) == 0 {
			return outcome, nil
		}
	}
	return outcome, nil
}

func scriptError(target resolvedTarget, idx int, runScript string, err error) error {
	if target.node == nil {
		return fmt.Errorf("本机执行第 %d 个脚本失败 (%s): %w", idx+1, runScript, err)
	}
	return fmt.Errorf("节点 %s (%s) 执行第 %d 个脚本失败 (%s): %w",
		target.name, target.node.IP, idx+1, runScript, err)
}

func printScriptOutput(target resolvedTarget, runScript, out string) {
	if target.node == nil {
		PrintInfo("exec local scripts -> %s ,scripts out ->\n%s", runScript, out)
		return
	}
	PrintInfo("exec remote -> node: %s ,scripts: %s ,scripts out ->\n%s", target.name, runScript, out)
}

type targetResult struct {
	out string
	err error
}

// fanOut 在各目标上执行同一条命令，结果下标与 targets 对齐。
//
// 每条脚本之后有一道栅栏（等全部目标跑完才进下一条）：并发只发生在"同一条脚本、
// 不同节点"之间。少了这道栅栏，节点 A 的第 2 条可能早于节点 B 的第 1 条执行，
// 对有先后依赖的编排来说就不再等价于串行了。
func fanOut(targets []resolvedTarget, limit int, runScript string) []targetResult {
	results := make([]targetResult, len(targets))

	if limit <= 1 || len(targets) <= 1 {
		for i, target := range targets {
			out, err := runOnTarget(target, runScript)
			results[i] = targetResult{out: out, err: err}
		}
		return results
	}

	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Add(1)
		go func(i int, target resolvedTarget) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			out, err := runOnTarget(target, runScript)
			results[i] = targetResult{out: out, err: err}
		}(i, target)
	}
	wg.Wait()
	return results
}

func runOnTarget(target resolvedTarget, runScript string) (string, error) {
	if target.node == nil {
		return RunCommandWithOutput("sh", "-c", runScript)
	}
	PrintDebug("remote node %s", target.node.IP)
	return RunCommandOnNode(target.node, runScript)
}

// RunCheck 在某个目标上执行幂等探针（Resource.check）。
//
// 退出 0 即视为"已经装好了"，跳过整个资源 —— 与 command -v / test -f 这类惯用写法一致。
// 探针非 0 退出不是错误，就是"还没装"；只有模板或节点解析这类说明配置本身有问题的
// 情况才返回 error。探针跑不起来（比如 SSH 不通）也按"还没装"处理：
// 让真正的安装去报那个真实的错，而不是在探针这一步报一个绕了一层的错。
func RunCheck(res types.Resource, target string) (bool, error) {
	probe, err := ParseStr(res.Check, res)
	if err != nil {
		return false, fmt.Errorf("资源 %s 的 check 模板解析失败 (%s): %w", res.Name, res.Check, err)
	}
	resolved, err := resolveTargets([]string{target})
	if err != nil {
		return false, err
	}

	out, runErr := runOnTarget(resolved[0], probe)
	PrintDebug("check %s on %s -> err=%v ,out ->\n%s", probe, TargetLabel(target), runErr, out)
	return runErr == nil, nil
}
