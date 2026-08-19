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
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"

	"io/ioutil"

	"github.com/spf13/viper"
	"github.com/structure-projects/somcli/pkg/types"

	"gopkg.in/yaml.v2"
)

var (
	offlineMode bool // 是否离线模式

	// 用户自定义模板变量，模板里以 {{.Vars.xxx}} 访问。
	// 分两层存：--set 必须能覆盖配置里的 vars，而配置是在命令跑起来之后才读的，
	// 合并成一个 map 就分不清哪个值来自哪一层了。
	configVars   = map[string]string{}
	overrideVars = map[string]string{}
)

// SetConfigVars 记录配置文件 vars: 段声明的变量。
func SetConfigVars(vars map[string]string) {
	configVars = vars
}

// SetOverrideVars 记录 --set k=v 传入的变量，优先级高于配置文件。
func SetOverrideVars(vars map[string]string) {
	overrideVars = vars
}

// TemplateVars 返回合并后的自定义变量：--set 覆盖配置文件。
func TemplateVars() map[string]string {
	merged := make(map[string]string, len(configVars)+len(overrideVars))
	for k, v := range configVars {
		merged[k] = v
	}
	for k, v := range overrideVars {
		merged[k] = v
	}
	return merged
}

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

// TemplateContext 是模板可见的全部变量。
//
// 只有这一份：脚本上下文与 target/URL 上下文各自写一份结构体的时候，两份必然漂移 ——
// {{.Filename}} 只在 target 里能用，写进 post_install 就报 "can't evaluate field"（E5）。
//
// url 为空时 Filename/Ext 退化到资源的第一个 URL。脚本上下文没有"当前 URL"的概念，
// 但资源基本都只有一个 URL，在 post_install 里写 {{.Filename}} 的意图是明确的。
func TemplateContext(res types.Resource, url string) map[string]any {
	if url == "" && len(res.URLs) > 0 {
		url = res.URLs[0]
	}
	name, ext := urlFilename(url)
	cacheDir := filepath.Join(GetDownloadDir(), res.Name, res.Version)

	return map[string]any{
		"Name":        res.Name,
		"Version":     res.Version,
		"Platform":    GetPlatform(),
		"Arch":        GetArch(),
		"DownloadDir": GetDownloadDir(),
		"AppDir":      GetAppDir(),
		"HostDir":     GetHomeDir(),
		"WorkDir":     GetWorkDir(),
		"DataDir":     GetDataDir(),
		"TmpDir":      GetTmpDir(),
		"ImagesDir":   GetImagesDir(),
		"ScriptDir":   GetScriptDir(),
		"CacheDir":    cacheDir,
		"InstallDir":  InstallDir(res),
		"SrcDir":      filepath.Join(cacheDir, "src"),
		"Filename":    name,
		"Ext":         ext,
		"Vars":        TemplateVars(),
	}
}

// InstallDir 是 method: binary / container 把产物放到哪儿。
//
// 缺省 /usr/local/bin：PATH 里的惯例位置，也正是现有配置的 post_install 手写的目标。
// 用例里改成工作目录下的 bin 就不需要 root。
func InstallDir(res types.Resource) string {
	if res.InstallDir != "" {
		return res.InstallDir
	}
	return "/usr/local/bin"
}

// urlFilename 从下载地址里取文件名与扩展名。
// query 与 fragment 必须先去掉：GitHub release 之类的地址常带 ?token=...，
// 直接 filepath.Base 会把它算进文件名。
func urlFilename(rawURL string) (name, ext string) {
	if rawURL == "" {
		return "", ""
	}
	trimmed := rawURL
	if i := strings.IndexAny(trimmed, "?#"); i >= 0 {
		trimmed = trimmed[:i]
	}
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return "", ""
	}
	name = path.Base(trimmed)
	return name, path.Ext(name)
}

// renderTemplate 用统一的上下文渲染一段模板。
//
// missingkey=error 是必须的：上下文改成 map 之后，text/template 默认把不存在的键渲染成
// "<no value>" 而不是报错（结构体字段才会报错）。少了这个选项，{{.Workdir}} 这种大小写
// 写错会静默产出 <no value>，产物落到谁也找不到的路径上 —— 正是 SC-F06 要挡的事。
func renderTemplate(tmpl string, ctx map[string]any) (string, error) {
	tpl, err := template.New("").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ParseStr 渲染脚本、命令这类不绑定具体 URL 的模板。
func ParseStr(tmpl string, res types.Resource) (string, error) {
	return renderTemplate(tmpl, TemplateContext(res, ""))
}

// ParseTargetPath 渲染 target 模板，额外绑定当次下载的 URL（{{.Filename}} / {{.Ext}}）。
func ParseTargetPath(tmpl, url string, res types.Resource) (string, error) {
	return renderTemplate(tmpl, TemplateContext(res, url))
}

// --- 内部辅助函数 ---

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

	// UnmarshalStrict：未知字段直接报错。默认的 Unmarshal 会静默忽略拼错的键，
	// 用户以为配了某个能力，实际从未生效。
	if err := yaml.UnmarshalStrict(data, &Config); err != nil {
		return nil, fmt.Errorf("解析配置 %s 失败: %w", path, err)
	}

	applyGlobalSettings(&Config)

	return &Config, nil
}

// applyGlobalSettings 让配置文件里的全局设置真正生效。
// 优先级：命令行标志 > ~/.somcli.yaml > 这里的资源配置。判据是 viper 里已有非空值就不动，
// 所以挡住它的不只是显式传的标志，全局配置文件里的同名项也会挡住。
func applyGlobalSettings(cfg *types.ResourceConfig) {
	if cfg.Offline {
		SetOffline(true)
	}
	if cfg.Debug {
		SetDebugMode(true)
	}
	if cfg.GithubProxy != "" && viper.GetString("github_proxy") == "" {
		viper.Set("github_proxy", cfg.GithubProxy)
	}
	if cfg.WorkDir != "" && viper.GetString("workdir") == "" {
		viper.Set("workdir", cfg.WorkDir)
	}
	if len(cfg.MirrorsSource) > 0 {
		InitSource(cfg.MirrorsSource)
	}
	SetConfigVars(cfg.Vars)
}

func SetNode(nodes []types.RemoteNode) {
	Config.Nodes = nodes
}

// LocalNodeIP 标识"在操作机本地执行"的节点。
const LocalNodeIP = "127.0.0.1"

// isLocalHostLiteral 判断 hosts 条目是否为显式的本机字面量。
func isLocalHostLiteral(hostname string) bool {
	switch hostname {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

// GetNode 按主机名或 IP 在已声明的 nodes 中查找节点。
//
// 查不到时返回错误而不是兜底到本机：兜底会让声明为远程的编排静默地在操作机上
// 执行 swapoff / 包管理器安装 / systemctl enable 这类破坏性动作。
// 只有 hosts 里显式写 localhost / 127.0.0.1 / ::1 才被当作本机目标。
func GetNode(hostname string) (types.RemoteNode, error) {
	for _, host := range Config.Nodes {
		if host.Host == hostname || host.IP == hostname {
			return host, nil
		}
	}

	if isLocalHostLiteral(hostname) {
		return types.RemoteNode{Host: hostname, IP: LocalNodeIP}, nil
	}

	if len(Config.Nodes) == 0 {
		return types.RemoteNode{}, fmt.Errorf(
			"无法解析主机 %q：配置中没有声明任何节点，请在配置顶层添加 nodes: 列表", hostname)
	}
	return types.RemoteNode{}, fmt.Errorf(
		"无法解析主机 %q：不在已声明的节点中（已声明：%s）", hostname, strings.Join(declaredNodeRefs(), ", "))
}

// declaredNodeRefs 返回已声明节点的可读引用，用于错误提示。
func declaredNodeRefs() []string {
	refs := make([]string, 0, len(Config.Nodes))
	for _, n := range Config.Nodes {
		switch {
		case n.Host != "" && n.IP != "":
			refs = append(refs, fmt.Sprintf("%s(%s)", n.Host, n.IP))
		case n.Host != "":
			refs = append(refs, n.Host)
		default:
			refs = append(refs, n.IP)
		}
	}
	return refs
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
