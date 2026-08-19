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
package installer

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"github.com/structure-projects/somcli/pkg/state"
	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
)

type Installer struct {
	DownloadDir string

	// Force 忽略 check 探针与状态记录，强制重新实施。
	// 幂等一旦参与决策就必须留一个出口：用户在目标机上手工改坏了东西、
	// 或者想在不改 version 的前提下重装一遍，否则只能去删状态文件。
	Force bool
}

func NewInstaller() *Installer {
	return &Installer{}
}

// 加载配置
func (i *Installer) InstallFromFile(configPath string, quiet bool) error {
	config, err := utils.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config failed: %w", err)
	}

	store := state.Load()
	var failures []string

	for _, tool := range config.Resources {
		err := i.install(tool, quiet, store)
		if err == nil {
			continue
		}
		// on_error: continue 只影响"要不要接着装下一个"，不影响退出码。
		// 失败却退 0 正是 M0 清掉的那类事：用户看不出环境是半装状态。
		if !continueOnError(tool) {
			return fmt.Errorf("%s install failed: %w", tool.Name, err)
		}
		utils.PrintError("%s 安装失败（on_error: continue，继续后续资源）: %v", tool.Name, err)
		failures = append(failures, fmt.Sprintf("  - %s: %v", tool.Name, err))
	}

	if len(failures) > 0 {
		return fmt.Errorf("%d 个资源失败（on_error: continue）:\n%s",
			len(failures), strings.Join(failures, "\n"))
	}
	return nil
}

// 根据名称安装
func (i *Installer) InstallTool(configPath string, name string, quiet bool) error {
	//  从资源中读取需要安装的资源
	config, err := utils.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config failed: %w", err)
	}
	declared := make([]string, 0, len(config.Resources))
	for _, tool := range config.Resources {
		//判断名称一致则启用安装流程
		if tool.Name == name {
			if err := i.install(tool, quiet, state.Load()); err != nil {
				return fmt.Errorf("%s install failed: %w", tool.Name, err)
			}
			return nil
		}
		declared = append(declared, tool.Name)
	}

	if len(declared) == 0 {
		return fmt.Errorf("配置 %s 中没有声明任何资源，无法安装 %q", configPath, name)
	}
	return fmt.Errorf("配置 %s 中不存在资源 %q（已声明：%s）", configPath, name, strings.Join(declared, ", "))
}

// Install 安装单个资源。保留导出签名给 pkg/cluster 等既有调用方，状态自带自读自写。
func (i *Installer) Install(tool types.Resource, quiet bool) error {
	return i.install(tool, quiet, state.Load())
}

// continueOnError 判断资源是否声明了容错。取值合法性由 utils 那边报错，这里只看是不是 continue。
func continueOnError(tool types.Resource) bool {
	return strings.EqualFold(strings.TrimSpace(tool.OnError), "continue")
}

// install 是单个资源的完整实施流程。
//
// 顺序：确定目标（幂等判据） → 下载分发 → 附加文件 → pre_install → method → post_install → 记状态。
// 幂等判据放在最前面：跳过一个资源就该连下载都不做，否则"已安装、跳过"还得先等几十兆下完。
func (i *Installer) install(tool types.Resource, quiet bool, store *state.Store) error {
	utils.PrintStage("开始安装 -> %s", tool.Name)

	targets, err := i.pendingTargets(tool, store)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		// 跳过原因已在 pendingTargets 里逐目标打印
		utils.PrintInfo("%s 无需实施，跳过", tool.Name)
		return nil
	}

	scoped := tool
	if len(scoped.Hosts) > 0 {
		scoped.Hosts = targets
	}

	nodes, err := resolveHosts(scoped)
	if err != nil {
		return err
	}

	artifacts, err := prepareFiles(scoped, nodes, quiet)
	if err != nil {
		return err
	}

	// 附加文件在 pre_install 之前就位：pre_install 里检查配置文件是否存在是常见写法
	if err := writeExtraFiles(scoped, nodes); err != nil {
		return err
	}

	walk := &stageWalk{res: scoped, alive: targets, continueOnError: continueOnError(tool)}

	ok, err := walk.step("pre-install failed", func(res types.Resource) (utils.RunOutcome, error) {
		utils.PrintStage("执行安装前置处理脚本")
		return utils.RunScriptsDetailed(res.PreInstall, res)
	})
	if !ok {
		return err
	}

	// method 分发在 pre 与 post 之间：pre 负责前置检查，post 负责收尾配置，
	// 中间这一步才是"把东西装上"本身
	ok, err = walk.step(fmt.Sprintf("method: %s 实施失败", tool.Method), func(res types.Resource) (utils.RunOutcome, error) {
		return applyMethod(res, artifacts)
	})
	if !ok {
		return err
	}

	ok, err = walk.step("post-install failed", func(res types.Resource) (utils.RunOutcome, error) {
		utils.PrintStage("执行安装后置处理脚本")
		return utils.RunScriptsDetailed(res.PostInstall, res)
	})
	if !ok {
		return err
	}

	// 每个资源装完就落一次状态，不等整轮结束：中途失败时前面装好的资源必须留下记录，
	// 否则重跑会把它们全部再装一遍（SC-F08）
	recordInstalled(store, tool, walk.alive)

	if len(walk.failed) > 0 {
		// 部分目标失败：装成的那些已经记了状态，但整体仍然是失败
		return fmt.Errorf("%s 部分目标失败: %w", tool.Name, utils.RunOutcome{Failed: walk.failed}.Err())
	}

	utils.PrintSuccess("%s %s 成功安装!", tool.Name, tool.Version)
	return nil
}

// pendingTargets 决定这一轮要在哪些目标上实施，并把跳过的原因逐个说出来。
//
// 判据顺序：--force > check 探针 > 状态记录。
// check 排在状态之前是因为它问的是机器的实况，状态问的是 somcli 自己的账本 ——
// 账本可能与实况不符（东西被别人删了、机器重装了），实况优先。
func (i *Installer) pendingTargets(tool types.Resource, store *state.Store) ([]string, error) {
	targets := utils.ScriptTargets(tool)
	if i.Force {
		return targets, nil
	}

	pending := make([]string, 0, len(targets))
	for _, target := range targets {
		label := utils.TargetLabel(target)

		if tool.Check != "" {
			hit, err := utils.RunCheck(tool, target)
			if err != nil {
				return nil, err
			}
			if hit {
				utils.PrintInfo("%s 的 check 已命中（%s），跳过 -> %s", tool.Name, tool.Check, label)
				continue
			}
		}

		if store.Installed(tool.Name, tool.Version, target) {
			utils.PrintInfo("%s %s 已记录为已安装，跳过 -> %s（要重装请加 --force）",
				tool.Name, tool.Version, label)
			continue
		}
		if old := store.InstalledVersion(tool.Name, target); old != "" {
			utils.PrintInfo("%s 已记录版本 %s，配置声明 %s，重新实施 -> %s",
				tool.Name, old, tool.Version, label)
		}
		pending = append(pending, target)
	}
	return pending, nil
}

// recordInstalled 记账并落盘。写不进去只警告：状态是辅助判据，
// 为了一个记账文件让已经装成的安装以失败收场是本末倒置 —— 顶多下次多装一遍。
func recordInstalled(store *state.Store, tool types.Resource, targets []string) {
	for _, target := range targets {
		store.Put(state.Record{
			Name:    tool.Name,
			Version: tool.Version,
			Host:    target,
			Method:  tool.Method,
		})
	}
	if err := store.Save(); err != nil {
		utils.PrintWarning("状态未能写入 %s（下次会重新实施 %s）: %v", state.Path(), tool.Name, err)
	}
}

// stageWalk 串起 pre_install / method / post_install 三个阶段，
// 并在 on_error: continue 下把已经失败的目标从后续阶段里剔掉。
//
// 剔除是必需的：带着一台前置检查就没过的机器继续跑 method 和 post_install，
// 只会把一个能定位的错误摊成一串派生错误，还可能在半截状态上做破坏性动作。
type stageWalk struct {
	res             types.Resource
	alive           []string
	failed          []utils.TargetError
	continueOnError bool
}

// step 执行一个阶段。返回 false 表示不再继续后续阶段，此时 error 即最终错误。
func (w *stageWalk) step(label string, run func(types.Resource) (utils.RunOutcome, error)) (bool, error) {
	scoped := w.res
	if len(scoped.Hosts) > 0 {
		scoped.Hosts = w.alive
	}

	outcome, fatal := run(scoped)
	if fatal != nil {
		return false, fmt.Errorf("%s: %w", label, fatal)
	}
	if len(outcome.Failed) == 0 {
		return true, nil
	}

	w.failed = append(w.failed, outcome.Failed...)
	if !w.continueOnError {
		return false, fmt.Errorf("%s: %w", label, utils.RunOutcome{Failed: w.failed}.Err())
	}

	w.alive = outcome.AliveTargets(w.alive)
	if len(w.alive) == 0 {
		return false, fmt.Errorf("%s: %w", label, utils.RunOutcome{Failed: w.failed}.Err())
	}
	utils.PrintWarning("%s 有 %d 个目标失败，on_error: continue，继续在剩余目标上实施：%s",
		w.res.Name, len(outcome.Failed), strings.Join(labels(w.alive), ", "))
	return true, nil
}

func labels(targets []string) []string {
	out := make([]string, 0, len(targets))
	for _, target := range targets {
		out = append(out, utils.TargetLabel(target))
	}
	return out
}

// resolveHosts 把 hosts 里的名字解析成节点，未声明 hosts 则返回空表示"在操作机本地实施"。
// 解析一次给下载分发与附加文件分发共用，避免两处各查一遍还可能查出不同结果。
func resolveHosts(tool types.Resource) ([]types.RemoteNode, error) {
	nodes := make([]types.RemoteNode, 0, len(tool.Hosts))
	for _, hostname := range tool.Hosts {
		node, err := utils.GetNode(hostname)
		if err != nil {
			return nil, fmt.Errorf("资源 %s 的 hosts 项 %q 无法解析为节点: %w", tool.Name, hostname, err)
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// prepareFiles 下载资源并分发到各节点，返回本地产物清单供 method 使用。
func prepareFiles(tool types.Resource, nodes []types.RemoteNode, quiet bool) ([]types.DownloadResult, error) {
	downloader := utils.NewDownloader(viper.GetString("github_proxy"))
	downloader.SetQuiet(quiet)
	utils.PrintStage("安装前文件准备工作")
	utils.PrintDebug("输出资源信息 -> %v , ", tool)

	artifacts := make([]types.DownloadResult, 0, len(tool.URLs))
	for _, url := range tool.URLs {
		res := DownloadSingleFile(downloader, tool, fmt.Sprint(url))
		if res.Error != nil {
			return nil, fmt.Errorf("准备文件失败 (%s): %w", res.URL, res.Error)
		}
		artifacts = append(artifacts, res)

		for _, node := range nodes {
			if node.IP == utils.LocalNodeIP {
				utils.PrintWarning("本机目标 %s，跳过文件分发", node.Host)
				continue
			}
			utils.PrintInfo("拷贝文件 %s 到远程主机-> %s", res.LocalPath, node.IP)
			if err := utils.CopyToRemote(node.User, node.IP, node.SSHKey, res.LocalPath, res.LocalPath); err != nil {
				return nil, fmt.Errorf("拷贝 %s 到节点 %s (%s) 失败: %w", res.LocalPath, node.Host, node.IP, err)
			}
		}
	}
	return artifacts, nil
}
