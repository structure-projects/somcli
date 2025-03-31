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
package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// 版本信息变量，这些可以通过构建时注入
var (
	Version   = "v1.0.0"     // 默认版本号
	GitCommit = "feat-1.0.1" // Git提交哈希
	BuildDate = "2025-03-24" // 构建时间
)

type versionInfo struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
	Compiler  string `json:"compiler"`
	Platform  string `json:"platform"`
}

func NewVersionCmd() *cobra.Command {
	var (
		short bool
		json  bool
	)

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Long:  `Print the version information for somcli command line tool.`,
		Run: func(cmd *cobra.Command, args []string) {
			info := versionInfo{
				Version:   Version,
				GitCommit: GitCommit,
				BuildDate: BuildDate,
				GoVersion: runtime.Version(),
				Compiler:  runtime.Compiler,
				Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			}

			if short {
				fmt.Println(Version)
				return
			}

			if json {
				fmt.Printf(`{
  "version": "%s",
  "gitCommit": "%s",
  "buildDate": "%s",
  "goVersion": "%s",
  "compiler": "%s",
  "platform": "%s"
}
`, info.Version, info.GitCommit, info.BuildDate, info.GoVersion, info.Compiler, info.Platform)
				return
			}

			fmt.Printf(`somcli - Structure-Ops CLI

Version:      %s
Git commit:   %s
Built:        %s
Go version:   %s
Compiler:     %s
Platform:     %s
`, info.Version, info.GitCommit, info.BuildDate, info.GoVersion, info.Compiler, info.Platform)
		},
	}

	cmd.Flags().BoolVarP(&short, "short", "s", false, "Print just the version number")
	cmd.Flags().BoolVarP(&json, "json", "j", false, "Print the version in JSON format")

	return cmd
}
