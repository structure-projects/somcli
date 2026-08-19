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
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Downloader 下载器。
//
// 用 net/http 而不是 exec wget：somcli 是"装工具的工具"，典型输入就是一台刚开机、
// 什么都没有的机器。shell out 到 wget 意味着在那台机器上 somcli 装不了任何东西，
// 包括装不了 wget 本身 —— 而它的交付形态本来是"拷一个静态二进制过去就能跑"。
//
// 系统代理无需另找出路：net/http 的默认 Transport 走 http.ProxyFromEnvironment，
// HTTP_PROXY / HTTPS_PROXY / NO_PROXY 照样生效。
type Downloader struct {
	Proxy       string // GitHub 代理地址
	UserAgent   string // 用户代理
	Timeout     int    // 单次尝试超时(秒)
	MaxAttempts int    // 最大尝试次数
	Quiet       bool   // 是否静默模式
	RetryDelay  int    // 重试间隔时间(秒)
}

// NewDownloader 创建下载器
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

// errNotRetryable 标记不值得重试的失败。
// 4xx 不是瞬时故障，重试 5 次只是把"地址写错了"这件事推迟 20 秒才告诉用户。
var errNotRetryable = errors.New("not retryable")

// Download 下载文件到 destFile，带超时与重试。
// destFile 为相对路径时落在 cacheDir 下，绝对路径则原样使用。
func (d *Downloader) Download(urlStr, destFile string, cacheDir string) error {
	PrintDebug("urlStr -> %s , destFile -> %s , cacheDir-> %s", urlStr, destFile, cacheDir)

	if !filepath.IsAbs(destFile) {
		destFile = filepath.Join(cacheDir, destFile)
	}
	// 建 destFile 的父目录，而不是 cacheDir：target 里写 bin/kubectl 这类子路径时
	// 只建 cacheDir 是不够的，写文件会因为中间目录不存在而失败。
	if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
		return fmt.Errorf("创建目录 %s 失败: %w", filepath.Dir(destFile), err)
	}

	// 离线模式：只认缓存，缺了就明说缺哪个文件，不去联网也不静默成功
	if IsOffline() {
		if _, err := os.Stat(destFile); err != nil {
			return fmt.Errorf("离线模式下文件不存在: %s (请先在联网环境执行 somcli download 准备离线包)", destFile)
		}
		PrintInfo("离线模式命中缓存 -> %s", destFile)
		return nil
	}

	downloadURL, err := d.resolveURL(urlStr)
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 1; attempt <= d.MaxAttempts; attempt++ {
		if attempt > 1 {
			PrintInfo("尝试第 %d 次下载 (共 %d 次), %d 秒后重试...",
				attempt, d.MaxAttempts, d.RetryDelay)
			time.Sleep(time.Duration(d.RetryDelay) * time.Second)
		}

		err := d.fetch(downloadURL, destFile)
		if err == nil {
			var hashFn func() hash.Hash = sha256.New
			sum, err := CalculateLocalHash(destFile, hashFn)
			if err != nil {
				return err
			}
			PrintInfo("文件下载成功，校验和 -> sha256:%s", sum)
			return nil
		}

		lastErr = err
		PrintInfo("下载失败: %v (尝试 %d/%d)", err, attempt, d.MaxAttempts)
		if errors.Is(err, errNotRetryable) {
			return fmt.Errorf("下载失败: %w", err)
		}
	}

	return fmt.Errorf("下载失败，超过最大尝试次数 %d: %w", d.MaxAttempts, lastErr)
}

// resolveURL 应用 GitHub 代理。
func (d *Downloader) resolveURL(urlStr string) (string, error) {
	if d.Proxy == "" {
		return urlStr, nil
	}
	proxied, err := ApplyGitHubProxy(urlStr, d.Proxy)
	if err != nil {
		return "", fmt.Errorf("应用 GitHub 代理失败: %w", err)
	}
	return proxied, nil
}

// fetch 下载一次。
//
// 先写 destFile.part 再改名：中途失败（网络断、校验前进程被杀）不会留下一个大小不对
// 却看起来完整的文件 —— 那种残留会让下一次运行以为缓存已命中。
func (d *Downloader) fetch(urlStr, destFile string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(d.Timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return fmt.Errorf("%w: 地址不合法 %s: %v", errNotRetryable, urlStr, err)
	}
	if d.UserAgent != "" {
		req.Header.Set("User-Agent", d.UserAgent)
	}

	if !d.Quiet {
		PrintInfo("GET %s -> %s", urlStr, destFile)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return fmt.Errorf("%w: %s 返回 %s", errNotRetryable, urlStr, resp.Status)
		}
		return fmt.Errorf("%s 返回 %s", urlStr, resp.Status)
	}

	partFile := destFile + ".part"
	f, err := os.Create(partFile)
	if err != nil {
		return fmt.Errorf("创建临时文件 %s 失败: %w", partFile, err)
	}

	written, err := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if err != nil {
		_ = os.Remove(partFile)
		return fmt.Errorf("写入 %s 失败: %w", partFile, err)
	}
	if closeErr != nil {
		_ = os.Remove(partFile)
		return fmt.Errorf("关闭 %s 失败: %w", partFile, closeErr)
	}

	if err := os.Rename(partFile, destFile); err != nil {
		_ = os.Remove(partFile)
		return fmt.Errorf("重命名 %s 失败: %w", partFile, err)
	}

	if !d.Quiet {
		PrintInfo("已写入 %d 字节 -> %s", written, destFile)
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
