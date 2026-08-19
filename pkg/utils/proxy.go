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
	"fmt"
	"net/url"
	"strings"
)

// ProxyConfig 代理配置
type ProxyConfig struct {
	GitHubProxy string // GitHub代理地址，如 "https://gh-proxy.com/"
}

// ApplyGitHubProxy 应用GitHub代理到原始URL。非 GitHub 系地址原样返回。
func ApplyGitHubProxy(rawURL, proxy string) (string, error) {
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

	if !isGitHubHost(u.Hostname()) {
		return rawURL, nil
	}

	PrintInfo("用户使用代理： -> %s", proxy)

	// 构建代理URL
	proxyURL := proxy + u.Host + u.Path
	return proxyURL, nil
}

// githubHosts 会被代理改写的主机。
//
// 判据必须是主机名精确匹配或子域，不能用 strings.Contains：
// 后者会把 github.com.evil.com 也算成 GitHub，把内网地址连同凭据一起送给第三方代理；
// 而在调用侧对整个 URL 做 Contains 还会命中 https://example.com/github.com/x 这种路径。
var githubHosts = []string{
	"github.com",
	"raw.githubusercontent.com",
	"objects.githubusercontent.com",
	"codeload.github.com",
}

func isGitHubHost(host string) bool {
	host = strings.ToLower(host)
	for _, known := range githubHosts {
		if host == known || strings.HasSuffix(host, "."+known) {
			return true
		}
	}
	return false
}
