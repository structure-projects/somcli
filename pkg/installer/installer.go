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
	for _, tool := range config.Resources {
		//判断名称一致则启用安装流程
		if tool.Name == name {
			if err := i.Install(tool, quiet); err != nil {
				return fmt.Errorf("%s install failed: %w", tool.Name, err)
			}
		}
	}
	return nil
}

// 安装
func (i *Installer) Install(tool types.Resource, quiet bool) error {
	utils.PrintStage("开始安装 -> %s", tool.Name)
	//判断是否需要下载
	proxy := viper.GetString("github_proxy")
	downloader := utils.NewDownloader(proxy)
	downloader.SetQuiet(quiet)
	utils.PrintStage("安装前文件准备工作")
	utils.PrintDebug("输出资源信息 -> %v , ", tool)
	for _, url := range tool.URLs {
		res := DownloadSingleFile(downloader, tool, fmt.Sprint(url))
		// 拷贝文件
		for _, hostname := range tool.Hosts {
			node := utils.GetNode(hostname)
			if node.IP == "127.0.0.1" {
				utils.PrintWarning("loacl install not copy file .")
			} else {
				utils.PrintInfo("拷贝文件 %s 到远程主机-> %s", res.LocalPath, node.IP)
				utils.CopyToRemote(node.User, node.IP, node.SSHKey, res.LocalPath, res.LocalPath)
			}

		}
	}
	utils.PrintStage("执行安装前置处理脚本")
	// 前置脚本
	if err := utils.RunScripts(tool.PreInstall, tool); err != nil {
		return fmt.Errorf("pre-install failed: %w", err)
	}

	//运行后置脚本
	utils.PrintStage("执行安装后置处理脚本")
	if err := utils.RunScripts(tool.PostInstall, tool); err != nil {
		return fmt.Errorf("post-install failed: %w", err)
	}

	utils.PrintSuccess("%s %s 成功安装!", tool.Name, tool.Version)
	return nil

}
