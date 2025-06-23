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
	"path/filepath"
	"runtime"
	"strings"
)

// SystemInfo 包含完整的系统信息
type SystemInfo struct {
	Platform      string            `json:"platform"`
	Architecture  string            `json:"architecture"`
	DistroInfo    map[string]string `json:"distro_info"`
	KernelVersion string            `json:"kernel_version"`
}

// GetSystemInfo 获取完整的系统信息
func GetSystemInfo() *SystemInfo {
	return &SystemInfo{
		Platform:      GetPlatform(),
		Architecture:  GetArchitecture(),
		DistroInfo:    GetDistroInfo(),
		KernelVersion: GetKernelVersion(),
	}
}

func GetPlatform() string {
	return runtime.GOOS
}

func GetArchitecture() string {
	arch := runtime.GOARCH
	// 将Go的架构名称转换为常见的下载URL中的架构名称
	switch arch {
	case "amd64":
		return "amd64" // 对应x86_64
	case "arm64":
		return "arm64" // 对应aarch64
	case "386":
		return "386" // 32位x86
	default:
		return arch
	}
}

func GetDistroInfo() map[string]string {
	distroInfo := make(map[string]string)

	if runtime.GOOS != "linux" {
		distroInfo["error"] = "Not a Linux system"
		return distroInfo
	}

	// 读取/etc/os-release文件
	if content, err := os.ReadFile("/etc/os-release"); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				key := parts[0]
				value := strings.Trim(parts[1], `"`)
				distroInfo[key] = value
			}
		}
	} else {
		distroInfo["error"] = fmt.Sprintf("Failed to read /etc/os-release: %v", err)
	}

	return distroInfo
}

func GetKernelVersion() string {
	switch runtime.GOOS {
	case "linux", "darwin", "freebsd", "openbsd":
		cmd := exec.Command("uname", "-r")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			return strings.TrimSpace(out.String())
		}
	case "windows":
		cmd := exec.Command("cmd", "/c", "ver")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			return strings.TrimSpace(out.String())
		}
	}
	return "unknown"
}

// GetDownloadURL 构建下载URL
func GetDownloadURL(baseURL string) (string, error) {
	info := GetSystemInfo()

	// 替换常见的占位符
	url := strings.ReplaceAll(baseURL, "{platform}", info.Platform)
	url = strings.ReplaceAll(url, "{arch}", info.Architecture)

	// 对于Kubernetes相关的URL，可能需要特定格式
	if strings.Contains(baseURL, "kubernetes") {
		if info.Platform != "linux" {
			return "", fmt.Errorf("kubernetes only supports Linux platform")
		}
		// Kubernetes URL通常使用amd64而不是x86_64
		url = strings.ReplaceAll(url, "x86_64", "amd64")
	}

	return url, nil
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
