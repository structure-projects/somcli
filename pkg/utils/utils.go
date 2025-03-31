package utils

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/viper"
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
	PrintInfo("workdir -> %s", workdir)

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
