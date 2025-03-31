package utils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileExists 检查文件是否存在
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// CreateDir 创建目录（如果不存在）
func CreateDir(path string) error {
	if FileExists(path) {
		return nil
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	return nil
}

// CopyFile 复制文件
func CopyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer source.Close()

	// 创建目标目录
	if err := CreateDir(filepath.Dir(dst)); err != nil {
		return err
	}

	destination, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// ReadFileToString 读取文件内容为字符串
func ReadFileToString(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return string(content), nil
}

// WriteStringToFile 写入字符串到文件
func WriteStringToFile(path, content string) error {
	// 创建目录
	if err := CreateDir(filepath.Dir(path)); err != nil {
		return err
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}

// AskForConfirmation 请求用户确认
func AskForConfirmation(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s [y/n]: ", prompt)

		response, err := reader.ReadString('\n')
		if err != nil {
			return false
		}

		response = strings.ToLower(strings.TrimSpace(response))

		if response == "y" || response == "yes" {
			return true
		} else if response == "n" || response == "no" {
			return false
		}
	}
}

// MoveFile 安全地移动/重命名文件
func MoveFile(sourcePath, destPath string) error {
	// 创建目标目录（如果不存在）
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %v", err)
	}

	// 尝试直接重命名（最快速的方式，但要求在同一文件系统）
	err := os.Rename(sourcePath, destPath)
	if err == nil {
		return nil
	}

	// 如果重命名失败（跨设备等情况），则使用复制+删除的方式
	inputFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer inputFile.Close()

	outputFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %v", err)
	}
	defer outputFile.Close()

	_, err = io.Copy(outputFile, inputFile)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %v", err)
	}

	// 确保目标文件有正确的权限
	if err := os.Chmod(destPath, 0755); err != nil {
		return fmt.Errorf("failed to set file permissions: %v", err)
	}

	// 删除原始文件
	if err := os.Remove(sourcePath); err != nil {
		return fmt.Errorf("failed to remove source file: %v", err)
	}

	return nil
}

// IsDirectory 检查路径是否是目录
func IsDirectory(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fileInfo.IsDir()
}
