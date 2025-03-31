package utils

import (
	"fmt"
	"net/url"
	"strings"
)

// ProxyConfig 代理配置
type ProxyConfig struct {
	GitHubProxy string // GitHub代理地址，如 "https://gh-proxy.com/"
}

// ApplyGitHubProxy 应用GitHub代理到原始URL
func ApplyGitHubProxy(rawURL, proxy string) (string, error) {
	PrintInfo("用户使用代理： -> %s", proxy)
	if proxy == "" {
		return rawURL, nil
	}

	// 确保代理URL以/结尾
	if !strings.HasSuffix(proxy, "/") {
		proxy += "/"
	}

	// 解析原始URL
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// 只处理github.com的URL
	if !strings.Contains(u.Host, "github.com") {
		return rawURL, nil
	}

	// 构建代理URL
	proxyURL := proxy + u.Host + u.Path
	return proxyURL, nil
}
