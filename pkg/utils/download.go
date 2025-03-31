// wget_downloader.go
package utils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WgetDownloader 基于 wget 的下载器
type WgetDownloader struct {
	Proxy       string // 代理地址
	UserAgent   string // 用户代理
	Timeout     int    // 超时时间(秒)
	MaxAttempts int    // 最大尝试次数
	Quiet       bool   // 是否静默模式
}

// NewWgetDownloader 创建新的 wget 下载器
func NewDownloader(proxy string) *WgetDownloader {
	return &WgetDownloader{
		Proxy:       proxy,
		UserAgent:   "somcli-downloader/1.0",
		Timeout:     900, // 15分钟
		MaxAttempts: 3,
		Quiet:       false,
	}
}

// Download 使用 wget 下载文件
func (d *WgetDownloader) Download(urlStr, destFile string, cacheDir string) error {
	// 如果是离线模式，检查文件是否存在
	if os.Getenv("SOMCLI_OFFLINE") == "true" {
		if _, err := os.Stat(destFile); os.IsNotExist(err) {
			return fmt.Errorf("离线模式下文件不存在: %s (请确保已下载离线安装包)", destFile)
		}
		return nil
	}
	PrintInfo("urlStr -> %s , destFile -> %s , cacheDir-> %s", urlStr, destFile, cacheDir)
	// 确保缓存目录存在
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// 如果 destFile 不是绝对路径，则放在缓存目录下
	if !filepath.IsAbs(destFile) {
		destFile = filepath.Join(cacheDir, destFile)
	}

	// 准备 wget 命令参数 - 使用老版本兼容的参数
	args := []string{
		"-c",                                   // 断点续传 (老版本使用 -c 而不是 --continue)
		"-t", fmt.Sprintf("%d", d.MaxAttempts), // 老版本使用 -t 而不是 --tries
		"-T", fmt.Sprintf("%d", d.Timeout), // 老版本使用 -T 而不是 --timeout
		"-O", destFile, // 老版本使用 -O 而不是 --output-document
	}

	// 设置用户代理
	if d.UserAgent != "" {
		args = append(args, "-U", d.UserAgent) // 老版本使用 -U 而不是 --user-agent
	}
	// 设置代理（仅对 GitHub URL）
	if d.Proxy != "" && strings.Contains(urlStr, "github.com") {
		// 应用GitHub代理
		downloadURL, err := ApplyGitHubProxy(urlStr, d.Proxy)
		if err != nil {
			return fmt.Errorf("failed to apply GitHub proxy: %v", err)
		}
		urlStr = downloadURL
	}

	// 静默模式
	if d.Quiet {
		args = append(args, "-q") // 老版本使用 -q 而不是 --quiet
	} else {
		// 老版本可能不支持 --show-progress，改为使用 -v (verbose)
		args = append(args, "-v")
	}

	// 添加 URL
	args = append(args, urlStr)

	PrintInfo("downloadURL : %s", urlStr)

	// 创建命令
	cmd := exec.Command("wget", args...)

	// 设置输出缓冲区
	var stdout, stderr bytes.Buffer
	if !d.Quiet {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	// 执行命令
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wget failed: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}

// SetQuiet 设置静默模式
func (d *WgetDownloader) SetQuiet(quiet bool) {
	d.Quiet = quiet
}
