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
	"os"

	"github.com/spf13/cobra"
	"github.com/structure-projects/somcli/pkg/installer"
	"github.com/structure-projects/somcli/pkg/types"
)

var (
	installConfigFile string
	installToolName   string
	downloadConfig    string
	quiet             bool
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install system tools",
	Long: `Supported installation methods:
- package: Use system package manager (apt/yum)
- binary:  Install pre-built binaries
- source:  Compile from source code
- container: Run as container`,
	Example: `  # Batch install from config
  somcli install -f configs/install.yaml

  # Install a single resource declared in the config
  somcli install -f configs/install.yaml -n kubectl`,
	Run: runInstall,
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().StringVarP(&installConfigFile, "file", "f", "", "Installation config file path")
	installCmd.Flags().StringVarP(&installToolName, "name", "n", "", "Only install the resource with this name")

	rootCmd.AddCommand(downloadCmd)

	downloadCmd.Flags().StringVarP(&downloadConfig, "file", "f", "", "Download configuration file (required)")
	downloadCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Quiet mode")

	downloadCmd.MarkFlagRequired("file")
}

func runInstall(cmd *cobra.Command, args []string) {
	inst := installer.NewInstaller()

	switch {
	// -n 只装配置里被点名的那一个资源。InstallTool 早就写好，只是没有入口接上。
	case installConfigFile != "" && installToolName != "":
		if err := inst.InstallTool(installConfigFile, installToolName, quiet); err != nil {
			fmt.Fprintf(os.Stderr, "Install %s failed: %v\n", installToolName, err)
			os.Exit(1)
		}

	case installConfigFile != "":
		if err := inst.InstallFromFile(installConfigFile, quiet); err != nil {
			fmt.Fprintf(os.Stderr, "Batch install failed: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintln(os.Stderr, "Error: must specify --file")
		cmd.Help()
		os.Exit(1)
	}
}

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download offline resources",
	Long:  `Download all required resources based on configuration file`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := executeOfflineDownload(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

// 离线下载
func executeOfflineDownload() error {
	config, err := installer.LoadDownloadConfig(downloadConfig)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 先打印明细再返回错误，便于定位是哪个资源失败
	results, downloadErr := installer.DownloadResources(config, quiet)
	printDownloadResults(results)
	if downloadErr != nil {
		return fmt.Errorf("download failed: %w", downloadErr)
	}
	return nil
}

// 打印下载结果
func printDownloadResults(results []types.DownloadResult) {
	fmt.Println("\nDownload results:")

	success := 0
	for _, res := range results {
		if res.Error == nil {
			success++
			fmt.Printf("  ✓ %s-%s: %s\n", res.Name, res.Version, res.LocalPath)
		} else {
			fmt.Printf("  ✗ %s-%s: %v\n", res.Name, res.Version, res.Error)
		}
	}

	fmt.Printf("\nSummary: %d/%d succeeded\n", success, len(results))
}
