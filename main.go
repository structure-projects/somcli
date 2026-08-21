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
package main

import (
	"embed"

	"github.com/structure-projects/somcli/cmd"
	"github.com/structure-projects/somcli/pkg/cluster"
)

// k8sCatalog 是 cluster create 装什么的全部内容，编译进二进制。
//
// 为什么嵌入而不是装到 /etc 下：install.sh 只拷一个二进制，从来不分发 configs/。
// 改成"必须有配置文件"的话，装好的 somcli 第一次 cluster create 就会因为找不到
// configs/k8s 而失败，而用户完全看不出自己少拿了什么东西。
//
// 为什么嵌入代码写在这里而不是 pkg/cluster 里：go:embed 的路径不能穿 ..，
// 而这批 YAML 必须留在仓库的 configs/ 下（用户要能看见、能照着改），
// 于是只有与 configs/ 同级的这个包能嵌入它们。
//
//go:embed configs/k8s/*.yaml
var k8sCatalog embed.FS

func main() {
	cluster.SetBuiltinK8sCatalog(k8sCatalog, "configs/k8s")
	cmd.Execute()
}
