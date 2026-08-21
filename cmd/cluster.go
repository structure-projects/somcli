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
	"os"

	"github.com/spf13/cobra"
	"github.com/structure-projects/somcli/pkg/cluster"
	"github.com/structure-projects/somcli/pkg/utils"
)

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Manage container clusters",
	Long:  `Create and manage container clusters including Kubernetes and Docker Swarm.`,
}

var clusterCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new cluster",
	Long:  `Create a new Kubernetes or Docker Swarm cluster based on configuration file.`,
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		configFile, _ := cmd.Flags().GetString("file")
		clusterName, _ := cmd.Flags().GetString("cluster-name")
		clusterType, _ := cmd.Flags().GetString("cluster-type")
		force, _ := cmd.Flags().GetBool("force")
		skipPrecheck, _ := cmd.Flags().GetBool("skip-precheck")

		// 验证配置文件存在
		if !utils.FileExists(configFile) {
			utils.PrintError("Config file %s does not exist", configFile)
			os.Exit(1)
		}

		// 创建集群
		err := cluster.CreateCluster(configFile, clusterName, clusterType, force, skipPrecheck)
		if err != nil {
			utils.PrintError("Failed to create cluster: %v", err)
			os.Exit(1)
		}

		utils.PrintSuccess("Cluster created successfully")
	},
}

var clusterRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove an existing cluster",
	Long:  `Remove an existing Kubernetes or Docker Swarm cluster based on configuration file.`,
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		configFile, _ := cmd.Flags().GetString("file")
		clusterName, _ := cmd.Flags().GetString("cluster-name")
		clusterType, _ := cmd.Flags().GetString("cluster-type")
		force, _ := cmd.Flags().GetBool("force")

		// 验证配置文件存在
		if !utils.FileExists(configFile) {
			utils.PrintError("Config file %s does not exist", configFile)
			os.Exit(1)
		}

		// 移除集群
		err := cluster.RemoveCluster(configFile, clusterName, clusterType, force)
		if err != nil {
			utils.PrintError("Failed to remove cluster: %v", err)
			os.Exit(1)
		}

		utils.PrintSuccess("Cluster removed successfully")
	},
}

// 扩缩容单独成命令，不复用 create/remove：
// 重跑 cluster create 会在第一台 master 上再跑一遍 kubeadm init，必然失败。
var clusterAddNodeCmd = &cobra.Command{
	Use:   "add-node",
	Short: "Add a node to an existing cluster",
	Long: `Add a node to an existing cluster.

The node must already be listed under cluster[].nodes in the configuration file
(that is where its address, credentials and role live); --node names which one.`,
	Args: cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		configFile, _ := cmd.Flags().GetString("file")
		clusterName, _ := cmd.Flags().GetString("cluster-name")
		clusterType, _ := cmd.Flags().GetString("cluster-type")
		host, _ := cmd.Flags().GetString("node")
		force, _ := cmd.Flags().GetBool("force")

		if !utils.FileExists(configFile) {
			utils.PrintError("Config file %s does not exist", configFile)
			os.Exit(1)
		}

		if err := cluster.AddNode(configFile, clusterName, clusterType, host, force); err != nil {
			utils.PrintError("Failed to add node: %v", err)
			os.Exit(1)
		}

		utils.PrintSuccess("Node added successfully")
	},
}

var clusterRemoveNodeCmd = &cobra.Command{
	Use:   "remove-node",
	Short: "Remove a node from an existing cluster",
	Long: `Remove a node from an existing cluster: drain it, reset it, then delete the Node object.

Use "cluster remove" to tear down the whole cluster; this command refuses to
remove the first master.`,
	Args: cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		configFile, _ := cmd.Flags().GetString("file")
		clusterName, _ := cmd.Flags().GetString("cluster-name")
		clusterType, _ := cmd.Flags().GetString("cluster-type")
		host, _ := cmd.Flags().GetString("node")
		force, _ := cmd.Flags().GetBool("force")

		if !utils.FileExists(configFile) {
			utils.PrintError("Config file %s does not exist", configFile)
			os.Exit(1)
		}

		if err := cluster.RemoveNode(configFile, clusterName, clusterType, host, force); err != nil {
			utils.PrintError("Failed to remove node: %v", err)
			os.Exit(1)
		}

		utils.PrintSuccess("Node removed successfully")
	},
}

func init() {
	// 创建命令
	clusterCreateCmd.Flags().StringP("file", "f", "", "Cluster configuration file (required)")
	clusterCreateCmd.Flags().String("cluster-name", "", "Which cluster in the config to create (see cluster[].name)")
	clusterCreateCmd.Flags().String("cluster-type", "", "Which cluster type in the config to create (k8s|swarm)")
	// 原来的说明是"Force creation even if prechecks fail"，但它从来不影响预检
	// （跳预检是 --skip-precheck），而 k8s 流程里根本没人读这个标志。
	// 现在与 install --force 同义：不看幂等状态重装一遍。
	clusterCreateCmd.Flags().Bool("force", false,
		"Re-apply resources even if check: hits or state.json already records them")
	clusterCreateCmd.Flags().Bool("skip-precheck", false, "Skip pre-installation checks")
	_ = clusterCreateCmd.MarkFlagRequired("file")

	// 移除命令
	clusterRemoveCmd.Flags().StringP("file", "f", "", "Cluster configuration file (required)")
	clusterRemoveCmd.Flags().String("cluster-name", "", "Which cluster in the config to remove (see cluster[].name)")
	clusterRemoveCmd.Flags().String("cluster-type", "", "Which cluster type in the config to remove (k8s|swarm)")
	clusterRemoveCmd.Flags().Bool("force", false, "Force removal without confirmation")
	_ = clusterRemoveCmd.MarkFlagRequired("file")

	// 扩容命令
	clusterAddNodeCmd.Flags().StringP("file", "f", "", "Cluster configuration file (required)")
	clusterAddNodeCmd.Flags().String("cluster-name", "", "Which cluster in the config to add to (see cluster[].name)")
	clusterAddNodeCmd.Flags().String("cluster-type", "", "Which cluster type in the config to add to (k8s|swarm)")
	clusterAddNodeCmd.Flags().String("node", "", "Hostname of the node to add, as written in cluster[].nodes (required)")
	clusterAddNodeCmd.Flags().Bool("force", false,
		"Re-apply resources even if check: hits or state.json already records them")
	_ = clusterAddNodeCmd.MarkFlagRequired("file")
	_ = clusterAddNodeCmd.MarkFlagRequired("node")

	// 缩容命令
	clusterRemoveNodeCmd.Flags().StringP("file", "f", "", "Cluster configuration file (required)")
	clusterRemoveNodeCmd.Flags().String("cluster-name", "", "Which cluster in the config to remove from (see cluster[].name)")
	clusterRemoveNodeCmd.Flags().String("cluster-type", "", "Which cluster type in the config to remove from (k8s|swarm)")
	clusterRemoveNodeCmd.Flags().String("node", "", "Hostname of the node to remove, as written in cluster[].nodes (required)")
	clusterRemoveNodeCmd.Flags().Bool("force", false, "Force removal without confirmation")
	_ = clusterRemoveNodeCmd.MarkFlagRequired("file")
	_ = clusterRemoveNodeCmd.MarkFlagRequired("node")

	// 添加子命令
	clusterCmd.AddCommand(clusterCreateCmd)
	clusterCmd.AddCommand(clusterRemoveCmd)
	clusterCmd.AddCommand(clusterAddNodeCmd)
	clusterCmd.AddCommand(clusterRemoveNodeCmd)

	// 添加到根命令
	rootCmd.AddCommand(clusterCmd)
}
