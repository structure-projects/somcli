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
	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
)

type Installer struct {
	DownloadDir string
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
	for _, tool := range config.Resources {
		if err := i.Install(tool, quiet); err != nil {
			return fmt.Errorf("%s install failed: %w", tool.Name, err)
		}
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
			if err := i.Install(tool, quiet); err != nil {
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

// 安装
func (i *Installer) Install(tool types.Resource, quiet bool) error {
	utils.PrintStage("开始安装 -> %s", tool.Name)

	nodes, err := resolveHosts(tool)
	if err != nil {
		return err
	}

	artifacts, err := prepareFiles(tool, nodes, quiet)
	if err != nil {
		return err
	}

	// 附加文件在 pre_install 之前就位：pre_install 里检查配置文件是否存在是常见写法
	if err := writeExtraFiles(tool, nodes); err != nil {
		return err
	}

	utils.PrintStage("执行安装前置处理脚本")
	if err := utils.RunScripts(tool.PreInstall, tool); err != nil {
		return fmt.Errorf("pre-install failed: %w", err)
	}

	// method 分发在 pre 与 post 之间：pre 负责前置检查，post 负责收尾配置，
	// 中间这一步才是"把东西装上"本身
	if err := applyMethod(tool, artifacts); err != nil {
		return fmt.Errorf("method: %s 实施失败: %w", tool.Method, err)
	}

	utils.PrintStage("执行安装后置处理脚本")
	if err := utils.RunScripts(tool.PostInstall, tool); err != nil {
		return fmt.Errorf("post-install failed: %w", err)
	}

	utils.PrintSuccess("%s %s 成功安装!", tool.Name, tool.Version)
	return nil
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
