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

	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
	"gopkg.in/yaml.v2"
)

// LoadDownloadConfig 加载下载配置文件
func LoadDownloadConfig(configPath string) (*types.ResourceConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config types.ResourceConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
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
	fullPath := filepath.Join(cacheDir, targetPath)
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
	utils.PrintSuccess("%s %s 成功下载 %s", result.Name, result.Version, result.LocalPath)

	return result
}

// DownloadResources 执行批量下载
func DownloadResources(config *types.ResourceConfig, quiet bool) ([]types.DownloadResult, error) {
	// 初始化下载器
	downloader := utils.NewDownloader(config.Proxy)
	downloader.SetQuiet(quiet)

	var results []types.DownloadResult
	for _, res := range config.Resources {
		for _, url := range res.URLs {
			result := DownloadSingleFile(downloader, res, url)
			results = append(results, result)
		}
	}

	return results, nil
}
