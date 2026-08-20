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
	"strings"

	"github.com/structure-projects/somcli/pkg/registry"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// RegistryCmd 是registry命令的根命令
var RegistryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Manage container image registries",
	Long:  "Commands for managing container image registries including Harbor installation and image synchronization",
}

var (
	harborVersion string
	harborHost    string
	caPath        string

	// uninstall 用自己的变量，不再蹭 install 的包级变量：过去它一个标志都没注册，
	// 却在 PreRunE 里校验那两个只有 install 才会填的变量，于是 `registry uninstall`
	// 无论怎么写都先死在"invalid hostname format"上，根本到不了卸载逻辑（E6）
	harborUninstallVersion string
	harborUninstallHost    string
)

var registryInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Harbor registry",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if !strings.Contains(harborHost, ".") && harborHost != "localhost" {
			return fmt.Errorf("invalid hostname format, must be a domain name or localhost")
		}
		if !strings.HasPrefix(harborVersion, "v") {
			return fmt.Errorf("harbor version must start with 'v'")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		manager := registry.NewHarborManager(
			harborVersion,
			harborHost,
			caPath,
			viper.GetViper(),
		)

		if err := manager.Install(); err != nil {
			fmt.Printf("Error installing Harbor: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Harbor installed successfully at %s\n", harborHost)
	},
}

var unInstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall Harbor registry",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// 卸载靠的是安装目录（`GetAppDir()/harbor`），既不看版本也不看主机名，
		// 所以这两个标志是可选的，只在用户真给了值时才校验格式
		if harborUninstallHost != "" &&
			!strings.Contains(harborUninstallHost, ".") && harborUninstallHost != "localhost" {
			return fmt.Errorf("invalid hostname format, must be a domain name or localhost")
		}
		if harborUninstallVersion != "" && !strings.HasPrefix(harborUninstallVersion, "v") {
			return fmt.Errorf("harbor version must start with 'v'")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		manager := registry.NewHarborManager(
			harborUninstallVersion,
			harborUninstallHost,
			"",
			viper.GetViper(),
		)

		if err := manager.Uninstall(); err != nil {
			fmt.Printf("Error UnInstall Harbor: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Harbor UnInstalled successfully")
	},
}

var (
	sourceReg   string
	targetReg   string
	username    string
	password    string
	concurrency int
	imageList   string
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync images between registries",
	PreRunE: func(cmd *cobra.Command, args []string) error {

		if !strings.HasPrefix(targetReg, "http://") && !strings.HasPrefix(targetReg, "https://") {
			return fmt.Errorf("target registry must start with http:// or https://")
		}
		if concurrency < 1 || concurrency > 10 {
			return fmt.Errorf("concurrency must be between 1 and 10")
		}
		if _, err := os.Stat(imageList); os.IsNotExist(err) {
			return fmt.Errorf("image list file does not exist")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if password == "" {
			password = os.Getenv("REGISTRY_PASSWORD")
			if password == "" {
				fmt.Println("Error: password must be provided via -p flag or REGISTRY_PASSWORD environment variable")
				os.Exit(1)
			}
		}

		syncer := registry.NewRegistrySyncer(
			sourceReg,
			targetReg,
			username,
			password,
			concurrency,
		)

		images, err := readImageList(imageList)
		if err != nil {
			fmt.Printf("Error reading image list: %v\n", err)
			os.Exit(1)
		}

		if len(images) == 0 {
			fmt.Println("No images found in the image list file")
			os.Exit(1)
		}

		if err := syncer.SyncAll(images); err != nil {
			fmt.Printf("Error syncing images: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Image sync completed successfully")
	},
}

func readImageList(file string) ([]string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read image list file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	var images []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			images = append(images, line)
		}
	}
	return images, nil
}

func init() {
	// sync命令参数
	syncCmd.Flags().StringVarP(&sourceReg, "source", "s", "", "Source registry URL (e.g. registry-1.docker.io) (required)")
	syncCmd.Flags().StringVarP(&targetReg, "target", "t", "", "Target registry URL (e.g. https://harbor.example.com) (required)")
	syncCmd.Flags().StringVarP(&username, "username", "u", "", "Registry username")
	syncCmd.Flags().StringVarP(&password, "password", "p", "", "Registry password (or use REGISTRY_PASSWORD env)")
	syncCmd.Flags().IntVarP(&concurrency, "concurrency", "c", 3, "Number of concurrent sync operations (1-10)")
	syncCmd.Flags().StringVarP(&imageList, "image-list", "f", "", "Path to file containing list of images to sync (one per line) (required)")

	syncCmd.MarkFlagRequired("source")
	syncCmd.MarkFlagRequired("target")
	syncCmd.MarkFlagRequired("image-list")

	// install命令参数 - 确保短标志不重复
	registryInstallCmd.Flags().StringVarP(&harborVersion, "version", "v", "v2.5.0", "Harbor version to install (e.g. v2.5.0)")
	registryInstallCmd.Flags().StringVarP(&harborHost, "hostname", "H", "", "Harbor hostname (e.g. harbor.example.com) (required)") // 将 'h' 改为 'H'
	registryInstallCmd.Flags().StringVar(&caPath, "ca-path", "", "Path to CA certificate files directory")
	registryInstallCmd.MarkFlagRequired("hostname")

	// uninstall 命令参数 —— 与 install 同名同短标志，但绑到自己的变量；
	// hostname 不设 required：卸载只需要安装目录
	unInstallCmd.Flags().StringVarP(&harborUninstallVersion, "version", "v", "", "Harbor version that was installed (optional)")
	unInstallCmd.Flags().StringVarP(&harborUninstallHost, "hostname", "H", "", "Harbor hostname that was installed (optional)")

	RegistryCmd.AddCommand(syncCmd)
	RegistryCmd.AddCommand(registryInstallCmd)
	RegistryCmd.AddCommand(unInstallCmd)
	rootCmd.AddCommand(RegistryCmd)
}
