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
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
)

// LoadDownloadConfig 加载下载配置文件。
//
// 直接委托 utils.LoadConfig：否则 download 与 install 读同一份文件会得到不同结果 ——
// 配置里的 offline / workdir 只在 install 那条路径上生效。
// 统一配置的前提是"哪条命令读都一样"。
func LoadDownloadConfig(configPath string) (*types.ResourceConfig, error) {
	return utils.LoadConfig(configPath)
}

// DownloadSingleFile 下载单个文件
func DownloadSingleFile(downloader *utils.Downloader, res types.Resource, url string) types.DownloadResult {

	// 解析URL
	parsedURL, err := utils.ParseStr(url, res)
	if err != nil {
		utils.PrintWarning("this url parse error -> %v", err)
		return types.DownloadResult{
			Name:    res.Name,
			Version: res.Version,
			URL:     url,
			Error:   fmt.Errorf("url parse failed: %w", err),
		}
	}

	// 解析目标路径模板
	targetPath, err := utils.ParseTargetPath(res.Target, parsedURL, res)
	if err != nil {
		return types.DownloadResult{
			Name:    res.Name,
			Version: res.Version,
			URL:     parsedURL,
			Error:   fmt.Errorf("parse target path failed: %w", err),
		}
	}

	cacheDir := filepath.Join(utils.GetDownloadDir(), res.Name, res.Version)
	// target 写绝对路径时产物就落在那个绝对路径上，不能再往 cacheDir 里拼。
	// 拼了的话 LocalPath 指向一个不存在的路径 —— 于是校验和读不到文件、分发拷不到东西、
	// 校验失败时要删的残留也删不掉，三处一起错（F3）。
	fullPath := targetPath
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(cacheDir, targetPath)
	}
	utils.PrintInfo("输出文件信息 -> 缓存目录： %s, 目标文件: %s , 下载地址: %s ", cacheDir, targetPath, parsedURL)

	err = downloader.Download(parsedURL, targetPath, cacheDir)
	result := types.DownloadResult{
		Name:      res.Name,
		Version:   res.Version,
		URL:       parsedURL,
		LocalPath: fullPath,
		Error:     err,
	}

	// 校验和验证
	if err == nil && res.Checksum != "" {
		if err := utils.VerifyChecksum(fullPath, res.Checksum); err != nil {
			result.Error = fmt.Errorf("checksum verification failed: %w", err)
			_ = os.Remove(fullPath)
		}
	}

	if result.Error != nil {
		utils.PrintError("%s %s 下载失败 %s -> %v", result.Name, result.Version, result.URL, result.Error)
		return result
	}
	utils.PrintSuccess("%s %s 成功下载 %s", result.Name, result.Version, result.LocalPath)

	return result
}

// DownloadResources 执行批量下载。任一资源失败即返回错误，使调用方的退出码非 0。
func DownloadResources(config *types.ResourceConfig, quiet bool) ([]types.DownloadResult, error) {
	// 代理取 github_proxy，与 installer.Install 同源。这里曾经读一个独立的顶层 proxy: 键，
	// 于是同一份配置 install 认代理、download 不认 —— 正是统一配置要消灭的差异。
	downloader := utils.NewDownloader(viper.GetString("github_proxy"))
	downloader.SetQuiet(quiet)

	var results []types.DownloadResult
	var failed int
	for _, res := range config.Resources {
		for _, url := range res.URLs {
			result := DownloadSingleFile(downloader, res, url)
			if result.Error != nil {
				failed++
			}
			results = append(results, result)
		}
	}

	if failed > 0 {
		return results, fmt.Errorf("%d/%d 个资源下载失败", failed, len(results))
	}
	return results, nil
}
