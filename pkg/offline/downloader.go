package offline

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/structure-projects/somcli/pkg/utils"
	"gopkg.in/yaml.v2"
)

// LoadDownloadConfig 加载下载配置文件
func LoadDownloadConfig(configPath string) (*DownloadConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config DownloadConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// DownloadResources 执行批量下载
func DownloadResources(config *DownloadConfig, quiet bool) ([]DownloadResult, error) {

	// 初始化下载器
	downloader := utils.NewDownloader(config.Proxy)
	downloader.SetQuiet(quiet)

	var results []DownloadResult
	for _, res := range config.Resources {
		for _, url := range res.URLs {
			// 解析目标路径模板
			targetPath, err := parseTargetPath(res.Target, url, res)
			if err != nil {
				results = append(results, DownloadResult{
					Name:    res.Name,
					Version: res.Version,
					URL:     url,
					Error:   fmt.Errorf("parse target path failed: %w", err),
				})
				continue
			}
			cacheDir := filepath.Join(utils.GetDownloadDir(), res.Name, res.Version)

			// 执行下载
			err = downloader.Download(url, targetPath, cacheDir)
			result := DownloadResult{
				Name:      res.Name,
				Version:   res.Version,
				URL:       url,
				LocalPath: targetPath,
				Error:     err,
			}

			// 校验和验证
			if err == nil && res.Checksum != "" {
				if err := verifyChecksum(filepath.Join(cacheDir, targetPath), res.Checksum); err != nil {
					result.Error = fmt.Errorf("checksum verification failed: %w", err)
					_ = os.Remove(filepath.Join(cacheDir, targetPath))
				}
			}

			results = append(results, result)
		}
	}

	return results, nil
}

// GetCachedFilePath 获取已缓存文件的绝对路径
func GetCachedFilePath(relativePath string, cacheDir string) (string, error) {
	if cacheDir == "" {
		cacheDir = utils.GetDownloadDir()
	}
	absPath := filepath.Join(cacheDir, relativePath)
	if _, err := os.Stat(absPath); err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}
	return absPath, nil
}

// --- 内部辅助函数 ---

func parseTargetPath(tmpl, url string, res DownloadResource) (string, error) {
	tpl, err := template.New("").Parse(tmpl)
	if err != nil {
		return "", err
	}

	data := struct {
		Name     string
		Version  string
		Filename string
	}{
		Name:     res.Name,
		Version:  res.Version,
		Filename: filepath.Base(url),
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func verifyChecksum(filePath, checksum string) error {
	// 实现校验和验证逻辑
	// 示例: 实际应根据checksum前缀（如sha256:）使用相应算法验证
	return nil
}
