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
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"html/template"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"io/ioutil"

	"github.com/spf13/viper"
	"github.com/structure-projects/somcli/pkg/types"

	"gopkg.in/yaml.v2"
)

var (
	offlineMode bool // 是否离线模式

)

// GetHomeDir 获取用户主目录
func GetHomeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := os.UserHomeDir(); err == nil {
		return usr
	}
	return ""
}

// ExpandPath 展开路径中的 ~ 和环境变量
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		path = filepath.Join(GetHomeDir(), path[2:])
	}
	return os.ExpandEnv(path)
}

// GetCurrentDir 获取当前工作目录
func GetCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}

// IsURL 检查字符串是否是URL
func IsURL(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// GetOS 获取当前操作系统
func GetOS() string {
	return runtime.GOOS
}

// GetArch 获取当前系统架构
func GetArch() string {
	return runtime.GOARCH
}

// StringInSlice 检查字符串是否在切片中
func StringInSlice(str string, list []string) bool {
	for _, v := range list {
		if v == str {
			return true
		}
	}
	return false
}

// MergeMaps 合并多个map
func MergeMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// IsExecutable 检查文件是否存在并且具有可执行权限
// 参数：
//
//	filePath - 要检查的文件路径
//
// 返回值：
//
//	bool - 如果文件存在且可执行返回true，否则返回false
func IsExecutable(filePath string) bool {
	// 首先检查文件是否存在（复用已有的FileExists函数）
	if !FileExists(filePath) {
		return false
	}

	// 获取文件的详细信息
	info, err := os.Stat(filePath)
	if err != nil {
		return false
	}

	// 检查：
	// 1. 是常规文件（不是目录、符号链接等特殊文件）
	// 2. 任意执行权限位被设置（owner/group/others任一有x权限）
	//    - 0111(八进制)对应二进制000 001 001 001，表示三个执行权限位
	//    - 通过与运算检查是否设置了任一执行位
	return info.Mode().IsRegular() &&
		(info.Mode()&0111 != 0)
}
func GetWorkDir() string {
	workdir := viper.GetString("workdir")
	// PrintInfo("workdir -> %s", workdir)

	if workdir == "" {
		return filepath.Join(GetCurrentDir(), "somwork")
	} else {
		return workdir
	}
}

func GetDownloadDir() string {
	return filepath.Join(GetWorkDir(), "download")
}

func GetDataDir() string {
	return filepath.Join(GetWorkDir(), "data")
}

func GetAppDir() string {
	return filepath.Join(GetWorkDir(), "apps")
}

func GetLogDir() string {
	return filepath.Join(GetWorkDir(), "logs")
}

func GetScriptDir() string {
	return filepath.Join(GetWorkDir(), "scripts")
}

func GetImagesDir() string {
	return filepath.Join(GetWorkDir(), "images")
}

func GetTmpDir() string {
	return filepath.Join("/tmp")
}

func GetWorkTmpDir() string {
	return filepath.Join(GetWorkDir(), "tmp")
}

func NormalizeVersion(version string) string {
	if !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
}

// 公共解析
func ParseStr(tmpl string, res types.Resource) (string, error) {
	tpl, err := template.New("").Parse(tmpl)

	if err != nil {
		return "", err
	}

	data := struct {
		Name        string
		Version     string
		Platform    string
		Arch        string
		DownloadDir string
		AppDir      string
		HostDir     string
		WorkDir     string
		DataDir     string
		TmpDir      string
		ImagesDir   string
		ScriptDir   string
		CacheDir    string
	}{
		Name:        res.Name,
		Version:     res.Version,
		Platform:    GetPlatform(),
		Arch:        GetArch(),
		DownloadDir: GetDownloadDir(),
		AppDir:      GetAppDir(),
		HostDir:     GetHomeDir(),
		WorkDir:     GetWorkDir(),
		DataDir:     GetDataDir(),
		TmpDir:      GetTmpDir(),
		ImagesDir:   GetImagesDir(),
		ScriptDir:   GetScriptDir(),
		CacheDir:    filepath.Join(GetDownloadDir(), res.Name, res.Version),
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// --- 内部辅助函数 ---

func ParseTargetPath(tmpl, url string, res types.Resource) (string, error) {

	tpl, err := template.New("").Parse(tmpl)
	if err != nil {
		return "", err
	}

	data := struct {
		Name        string
		Version     string
		Platform    string
		Arch        string
		DownloadDir string
		AppDir      string
		HostDir     string
		WorkDir     string
		DataDir     string
		TmpDir      string
		ImagesDir   string
		ScriptDir   string
		CacheDir    string
		Filename    string
		Ext         string
	}{
		Name:        res.Name,
		Version:     res.Version,
		Platform:    GetPlatform(),
		Arch:        GetArch(),
		DownloadDir: GetDownloadDir(),
		AppDir:      GetAppDir(),
		HostDir:     GetHomeDir(),
		WorkDir:     GetWorkDir(),
		DataDir:     GetDataDir(),
		TmpDir:      GetTmpDir(),
		ImagesDir:   GetImagesDir(),
		ScriptDir:   GetScriptDir(),
		CacheDir:    filepath.Join(GetDownloadDir(), res.Name, res.Version),
		Filename:    filepath.Base(url),
		Ext:         filepath.Ext(url),
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func SetEnv(name string, value string) {
	err := os.Setenv(name, value)
	if err != nil {
		fmt.Println("Error setting environment variable:", err)
	} else {
		fmt.Println("Environment variable set successfully")
	}
}

func SetOffline(isOffline bool) {
	offlineMode = isOffline
}

func IsOffline() bool {
	return offlineMode || os.Getenv("SOMCLI_OFFLINE") == "true"
}

var Config types.ResourceConfig

// 加载配置文件
func LoadConfig(path string) (*types.ResourceConfig, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, &Config); err != nil {
		return nil, err
	}

	return &Config, nil
}

func SetNode(nodes []types.RemoteNode) {
	Config.Nodes = nodes
}
func GetNode(hostname string) types.RemoteNode {

	for _, host := range Config.Nodes {
		if host.Host == hostname || host.IP == hostname {
			return host
		}
	}
	return types.RemoteNode{
		IP: "127.0.0.1",
	}
}

func GetNodes() []types.RemoteNode {
	return Config.Nodes
}

func IsValidIP(ip string) bool {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if num, err := strconv.Atoi(part); err != nil || num < 0 || num > 255 {
			return false
		}
	}
	return true
}

//

// Verify 验证校验和（支持本地文件和远程URL）
// target: 文件路径或URL
// checksum: 校验和字符串，格式可以是：
//   - "" (空字符串): 跳过验证
//   - "sha256:abcdef...": 指定哈希算法的校验和
//   - "abcdef...": 默认使用sha256算法
//
// 返回: 验证成功返回nil，失败返回错误
func VerifyChecksum(filePath, checksum string) error {
	PrintDebug("验证文件信息")
	if checksum == "" {
		return nil
	}

	// 解析哈希算法和校验值
	hashFn, expectedSum, err := ParseChecksum(checksum)
	if err != nil {
		return err
	}
	//本地验证
	actualSum, err := CalculateLocalHash(filePath, hashFn)
	if err != nil {
		return err
	}
	PrintDebug("输出文件信息 -> %s,%s", actualSum, expectedSum)
	// 比较校验和
	if !strings.EqualFold(actualSum, expectedSum) {
		return fmt.Errorf("checksum mismatch\n  expected: %s\n  actual:   %s",
			expectedSum, actualSum)
	}

	return nil
}

// calculateLocalHash 计算本地文件哈希
func CalculateLocalHash(filePath string, hashFn func() hash.Hash) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := hashFn()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// GetRemoteFileChecksum 远程获取文件的 checksum (默认为 sha256)
func GetRemoteFileChecksum(user, ip, keyPath, filePath string, algo ...string) (string, error) {
	// 默认使用 sha256sum，也可以指定其他算法
	checksumAlgo := "sha256sum"
	if len(algo) > 0 {
		switch algo[0] {
		case "md5":
			checksumAlgo = "md5sum"
		case "sha1":
			checksumAlgo = "sha1sum"
		case "sha256":
			checksumAlgo = "sha256sum"
		case "sha512":
			checksumAlgo = "sha512sum"
		default:
			return "", fmt.Errorf("不支持的 checksum 算法: %s", algo[0])
		}
	}

	// 构建远程命令
	cmd := fmt.Sprintf("%s %s | awk '{print $1}'", checksumAlgo, filePath)

	// 执行远程命令
	output, err := SSHMCmd(user, ip, keyPath, cmd)
	if err != nil {
		return "", fmt.Errorf("获取远程文件 checksum 失败: %w", err)
	}

	// 清理输出结果
	checksum := strings.TrimSpace(output)
	if checksum == "" {
		return "", fmt.Errorf("远程文件不存在或 checksum 计算失败")
	}

	return checksum, nil
}

// parseChecksum 解析校验和字符串
func ParseChecksum(checksum string) (func() hash.Hash, string, error) {
	var hashFn func() hash.Hash
	var expectedSum string

	// 检查是否包含算法前缀
	if parts := strings.SplitN(checksum, ":", 2); len(parts) == 2 {
		// 有明确算法前缀的情况 (如 "sha256:abcdef...")
		switch parts[0] {
		case "md5":
			hashFn = md5.New
		case "sha1":
			hashFn = sha1.New
		case "sha256":
			hashFn = sha256.New
		case "sha512":
			hashFn = sha512.New
		default:
			return nil, "", fmt.Errorf("unsupported hash algorithm: %s", parts[0])
		}
		expectedSum = parts[1]
	} else {
		// 没有算法前缀，默认使用sha256
		hashFn = sha256.New
		expectedSum = checksum
	}

	return hashFn, expectedSum, nil
}
