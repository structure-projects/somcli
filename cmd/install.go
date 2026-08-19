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
	"github.com/spf13/viper"
	"github.com/structure-projects/somcli/pkg/installer"
	"github.com/structure-projects/somcli/pkg/types"
)

var (
	installConfigFile   string
	installToolName     string
	installForce        bool
	installParallel     int
	uninstallConfigFile string
	uninstallToolName   string
	downloadConfig      string
	quiet               bool
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install system tools",
	Long: `Install resources declared in a config file.

Each resource is processed as: download urls -> distribute to hosts -> render
extra_files -> run pre_install -> apply method -> run post_install.

The ` + "`method`" + ` field selects how the resource is actually installed:

  script     (default) the pre_install / post_install scripts are the install
  binary     unpack the artifact if needed, install executables into install_dir
  package    hand the package name to the target machine's package manager
  container  pull image and drop a wrapper script named after the resource
  source     unpack the source archive into {{.SrcDir}} and run build:

A resource is skipped when its check: probe exits 0, or when state.json already
records it at the declared version. Bump version:, or pass --force, to re-apply.
` + "`on_error: continue`" + ` keeps the run going past a failed resource (the exit
code is still non-zero); the default aborts at the first failure.`,
	Example: `  # Batch install from config
  somcli install -f configs/tools.yaml

  # Install a single resource declared in the config
  somcli install -f configs/tools.yaml -n jq`,
	Run: runInstall,
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall resources by running their remove_scripts",
	Long: `Run the remove_scripts of resources declared in a config file.

Resources are processed in reverse declaration order, so things are torn down
before whatever they depend on. A resource without remove_scripts is skipped
with a warning.`,
	Example: `  # Uninstall everything declared in the config
  somcli uninstall -f configs/tools.yaml

  # Uninstall a single resource
  somcli uninstall -f configs/tools.yaml -n jq`,
	Run: runUninstall,
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().StringVarP(&installConfigFile, "file", "f", "", "Installation config file path")
	installCmd.Flags().StringVarP(&installToolName, "name", "n", "", "Only install the resource with this name")
	installCmd.Flags().BoolVar(&installForce, "force", false,
		"Re-apply resources even if check: hits or state.json already records them")
	installCmd.Flags().IntVar(&installParallel, "parallel", 1,
		"Run each script on up to N hosts concurrently (per-resource, hosts only)")

	// --force 与 --parallel 只挂在 install 上：uninstall 是拆环境，并发拆与
	// "强制再拆一遍"都没有已验证的用例，挂上去就是又一个没人走过的分支
	viper.BindPFlag("parallel", installCmd.Flags().Lookup("parallel"))

	rootCmd.AddCommand(uninstallCmd)
	uninstallCmd.Flags().StringVarP(&uninstallConfigFile, "file", "f", "", "Config file path (required)")
	uninstallCmd.Flags().StringVarP(&uninstallToolName, "name", "n", "", "Only uninstall the resource with this name")
	uninstallCmd.MarkFlagRequired("file")

	rootCmd.AddCommand(downloadCmd)

	downloadCmd.Flags().StringVarP(&downloadConfig, "file", "f", "", "Download configuration file (required)")
	downloadCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Quiet mode")

	downloadCmd.MarkFlagRequired("file")
}

func runUninstall(cmd *cobra.Command, args []string) {
	inst := installer.NewInstaller()

	if uninstallToolName != "" {
		if err := inst.UninstallTool(uninstallConfigFile, uninstallToolName, quiet); err != nil {
			fmt.Fprintf(os.Stderr, "Uninstall %s failed: %v\n", uninstallToolName, err)
			os.Exit(1)
		}
		return
	}
	if err := inst.UninstallFromFile(uninstallConfigFile, quiet); err != nil {
		fmt.Fprintf(os.Stderr, "Uninstall failed: %v\n", err)
		os.Exit(1)
	}
}

func runInstall(cmd *cobra.Command, args []string) {
	inst := installer.NewInstaller()
	inst.Force = installForce

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
