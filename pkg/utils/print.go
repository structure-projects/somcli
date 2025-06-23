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
	"os"
	"strings"
)

// 颜色定义
const (
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorReset   = "\033[0m"
)

// DebugMode 控制是否输出调试信息
var DebugMode = false

// PrintError 打印错误信息（红色）
func PrintError(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintf(os.Stderr, "%s[ERROR]%s %s\n", ColorRed, ColorReset, msg)
}

// PrintSuccess 打印成功信息（绿色）
func PrintSuccess(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintf(os.Stdout, "%s[SUCCESS]%s %s\n", ColorGreen, ColorReset, msg)
}

// PrintInfo 打印信息（蓝色）
func PrintInfo(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintf(os.Stdout, "%s[INFO]%s %s\n", ColorBlue, ColorReset, msg)
}

// PrintWarning 打印警告信息（黄色）
func PrintWarning(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintf(os.Stdout, "%s[WARNING]%s %s\n", ColorYellow, ColorReset, msg)
}

// PrintBanner 打印横幅信息
func PrintBanner(text string) {
	border := strings.Repeat("=", len(text)+4)
	fmt.Printf("\n%s\n  %s  \n%s\n\n", border, text, border)
}

// PrintStage 打印阶段信息（青色），用于标识重要阶段
func PrintStage(format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintf(os.Stdout, "%s[STAGE]%s %s\n", ColorCyan, ColorReset, msg)
}

// PrintDebug 打印调试信息（洋红色），仅在DebugMode为true时输出
func PrintDebug(format string, a ...interface{}) {
	if DebugMode {
		msg := fmt.Sprintf(format, a...)
		fmt.Fprintf(os.Stdout, "%s[DEBUG]%s %s\n", ColorMagenta, ColorReset, msg)
	}
}

// SetDebugMode 设置调试模式开关
func SetDebugMode(debug bool) {
	DebugMode = debug
}

// IsDebugMode 返回当前调试模式状态
func IsDebugMode() bool {
	return DebugMode
}
