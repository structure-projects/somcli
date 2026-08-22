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

// installPackage 实现 method: package —— 交给目标机器上的发行版包管理器。
//
// 装哪个包、用什么命令模板、怎么探测包管理器，全部在 pkg/utils 的发行版抽象里；
// 这里只负责从配置取出包名并决定要不要提权。默认不提权：非 root 就自动加 sudo
// 等于用户没要求提权却被提权了，远程节点上更难预料 —— 要不要提权永远由 --sudo 说。
func installPackage(res types.Resource) ([]string, error) {
	pkg := res.Package
	if pkg == "" {
		pkg = res.Name
	}
	if pkg == "" {
		return nil, fmt.Errorf("method: package 需要 package: 或 name: 指明包名")
	}

	sudo := ""
	if viper.GetBool("sudo") {
		sudo = "sudo "
	}

	cmds := []string{}
	// 装包前先按顶层 source: 配置在目标节点上把软件源配好（换源/挂 ISO）。
	// official 或留空返回空串，等价于用系统自带默认源直接装。
	setup, err := utils.RenderSourceSetup(utils.Config.Source, sudo)
	if err != nil {
		return nil, err
	}
	if setup != "" {
		cmds = append(cmds, setup)
	}
	cmds = append(cmds, utils.RenderPkgCommand(utils.PkgInstall, pkg, sudo))
	return cmds, nil
}
