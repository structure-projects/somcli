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
	"path/filepath"
	"runtime"
)

func GetPlatform() string {
	return runtime.GOOS
}

// GetUnameArch 返回 uname -m 风格的架构名（x86_64 / aarch64 / i386）。
//
// 与 {{.Arch}}（Go 的 GOARCH：amd64 / arm64）并存不是冗余：release 资产命名两派都有 ——
// kubectl 用 amd64，docker compose 用 x86_64。少了这个变量，URL 里就只能硬编码架构，
// 而硬编码的 x86_64 正是 compose 安装器在 arm64 机器上装不上的原因（F13）。
func GetUnameArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	case "386":
		return "i386"
	default:
		return runtime.GOARCH
	}
}

func InitSource(sources []string) {
	PrintInfo("加载源 -> %s", sources)

	for _, source := range sources {
		ext := filepath.Ext(source)
		PrintDebug("Init source -> %s , ext -> %s", source, ext)
		if ext == ".iso" {

		}
		if ext == ".sh" {

		}
	}
}
