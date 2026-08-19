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

	"github.com/structure-projects/somcli/pkg/state"
	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
)

// UninstallFromFile 卸载配置里的全部资源（E2）。
//
// 资源按声明的**逆序**处理：配置是按依赖顺序写的（先容器运行时、再 k8s 组件），
// 拆的时候顺着来会先删掉被依赖的东西，后面的卸载脚本就没法执行了。
// 单个资源内部的 remove_scripts 按声明顺序执行 —— 那本来就是一段"卸载步骤"。
func (i *Installer) UninstallFromFile(configPath string, quiet bool) error {
	config, err := utils.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config failed: %w", err)
	}
	if len(config.Resources) == 0 {
		return fmt.Errorf("配置 %s 中没有声明任何资源", configPath)
	}

	for idx := len(config.Resources) - 1; idx >= 0; idx-- {
		if err := i.Uninstall(config.Resources[idx]); err != nil {
			return err
		}
	}
	return nil
}

// UninstallTool 按名字卸载单个资源。
func (i *Installer) UninstallTool(configPath, name string, quiet bool) error {
	config, err := utils.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config failed: %w", err)
	}
	declared := make([]string, 0, len(config.Resources))
	for _, res := range config.Resources {
		if res.Name == name {
			return i.Uninstall(res)
		}
		declared = append(declared, res.Name)
	}
	if len(declared) == 0 {
		return fmt.Errorf("配置 %s 中没有声明任何资源，无法卸载 %q", configPath, name)
	}
	return fmt.Errorf("配置 %s 中不存在资源 %q（已声明：%s）", configPath, name, strings.Join(declared, ", "))
}

// Uninstall 执行单个资源的 remove_scripts。
//
// 没写 remove_scripts 的资源只警告不报错：一份配置里往往只有一部分资源需要卸载动作，
// 为此让整个 uninstall 失败会逼用户给每个资源都写一句 true。
// 但这件事必须说出来 —— 否则"卸载成功"与"什么都没卸"在输出上没有区别。
func (i *Installer) Uninstall(res types.Resource) error {
	if len(res.RemoveScripts) == 0 {
		utils.PrintWarning("%s 未声明 remove_scripts，跳过（不会有任何卸载动作）", res.Name)
		return nil
	}

	utils.PrintStage("开始卸载 -> %s", res.Name)
	if err := utils.RunScripts(res.RemoveScripts, res); err != nil {
		return fmt.Errorf("%s 卸载失败: %w", res.Name, err)
	}

	forgetInstalled(res)
	utils.PrintSuccess("%s %s 已卸载", res.Name, res.Version)
	return nil
}

// forgetInstalled 卸载成功后销掉状态记录。
//
// 不销的话下次 install 会被幂等判据拦住，打印"已记录为已安装，跳过"，
// 而目标机上其实什么都没有 —— 状态参与决策就必须跟着卸载一起维护。
func forgetInstalled(res types.Resource) {
	store := state.Load()
	for _, target := range utils.ScriptTargets(res) {
		store.Forget(res.Name, target)
	}
	if err := store.Save(); err != nil {
		utils.PrintWarning("状态未能写入 %s（%s 的安装记录仍在，下次 install 可能被跳过）: %v",
			state.Path(), res.Name, err)
	}
}
