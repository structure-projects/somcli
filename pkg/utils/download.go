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
// wget_downloader.go
package utils

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// WgetDownloader 基于 wget 的下载器
type Downloader struct {
	Proxy       string // 代理地址
	UserAgent   string // 用户代理
	Timeout     int    // 超时时间(秒)
	MaxAttempts int    // 最大尝试次数
	Quiet       bool   // 是否静默模式
	RetryDelay  int    // 重试间隔时间(秒)
}

// NewWgetDownloader 创建新的 wget 下载器
func NewDownloader(proxy string) *Downloader {
	return &Downloader{
		Proxy:       proxy,
		UserAgent:   "somcli-downloader/1.0",
		Timeout:     900, // 15分钟
		MaxAttempts: 5,
		Quiet:       false,
		RetryDelay:  5, // 默认5秒重试间隔
	}
}

// Download 使用 wget 下载文件，带超时和重试机制
func (d *Downloader) Download(urlStr, destFile string, cacheDir string) error {
	PrintDebug("urlStr -> %s , destFile -> %s , cacheDir-> %s", urlStr, destFile, cacheDir)

	// 处理文件路径
	if filepath.IsAbs(destFile) {
		PrintDebug("绝对 -> %s", filepath.Dir(destFile))
		if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
			return fmt.Errorf("failed to create cache directory: %w", err)
		}
	} else {
		PrintDebug("相对 -> %s", cacheDir)
		destFile = filepath.Join(cacheDir, destFile)
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			return fmt.Errorf("failed to create cache directory: %w", err)
		}
	}

	// 离线模式检查
	if IsOffline() {
		if _, err := os.Stat(destFile); os.IsNotExist(err) {
			return fmt.Errorf("离线模式下文件不存在: %s (请确保已下载离线安装包)", destFile)
		}
		return nil
	}

	var lastErr error
	for attempt := 1; attempt <= d.MaxAttempts; attempt++ {
		if attempt > 1 {
			PrintInfo("尝试第 %d 次下载 (共 %d 次), %d 秒后重试...",
				attempt, d.MaxAttempts, d.RetryDelay)
			time.Sleep(time.Duration(d.RetryDelay) * time.Second)
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(d.Timeout)*time.Second)
		defer cancel()

		err := d.downloadWithContext(ctx, urlStr, destFile)
		if err == nil {
			// 下载成功，计算哈希
			var hashFn func() hash.Hash = sha256.New
			hash, err := CalculateLocalHash(destFile, hashFn)
			if err != nil {
				return err
			}
			PrintInfo("文件下载成功，校验和 -> sha256:%s", hash)
			return nil
		}

		lastErr = err
		if ctx.Err() == context.DeadlineExceeded {
			PrintInfo("下载超时 (尝试 %d/%d)", attempt, d.MaxAttempts)
		} else {
			PrintInfo("下载失败: %v (尝试 %d/%d)", err, attempt, d.MaxAttempts)
		}
	}

	return fmt.Errorf("下载失败，超过最大尝试次数 %d: %w", d.MaxAttempts, lastErr)
}

// downloadWithContext 使用context实现带超时的下载
func (d *Downloader) downloadWithContext(ctx context.Context, urlStr, destFile string) error {
	// 准备 wget 命令参数
	args := []string{
		"-c",      // 断点续传
		"-t", "1", // 禁用wget内部重试，由我们控制重试逻辑
		"-T", fmt.Sprintf("%d", d.Timeout), // 超时时间
		"-O", destFile, // 输出文件
	}

	// 设置用户代理
	if d.UserAgent != "" {
		args = append(args, "-U", d.UserAgent)
	}

	// 设置代理（仅对 GitHub URL）
	if d.Proxy != "" && strings.Contains(urlStr, "github.com") {
		downloadURL, err := ApplyGitHubProxy(urlStr, d.Proxy)
		if err != nil {
			return fmt.Errorf("failed to apply GitHub proxy: %v", err)
		}
		urlStr = downloadURL
	}

	// 静默模式
	if d.Quiet {
		args = append(args, "-q")
	} else {
		args = append(args, "-v")
	}

	// 添加 URL
	args = append(args, urlStr)

	PrintDebug("exec download wget args -> %s", args)

	// 创建命令
	cmd := exec.CommandContext(ctx, "wget", args...)

	// 设置输出缓冲区
	var stdout, stderr bytes.Buffer
	if !d.Quiet {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	err := cmd.Run()
	if err != nil {
		select {
		case <-ctx.Done():
			return ctx.Err() // 返回超时错误
		default:
			return fmt.Errorf("wget failed: %w (stderr: %s)", err, stderr.String())
		}
	}
	return nil
}

// SetQuiet 设置静默模式
func (d *Downloader) SetQuiet(quiet bool) {
	d.Quiet = quiet
}

// SetRetryDelay 设置重试间隔时间
func (d *Downloader) SetRetryDelay(delay int) {
	d.RetryDelay = delay
}
