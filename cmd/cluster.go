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
		clusterType, _ := cmd.Flags().GetString("cluster-type")
		force, _ := cmd.Flags().GetBool("force")
		skipPrecheck, _ := cmd.Flags().GetBool("skip-precheck")

		// 验证配置文件存在
		if !utils.FileExists(configFile) {
			utils.PrintError("Config file %s does not exist", configFile)
			os.Exit(1)
		}

		// 创建集群
		err := cluster.CreateCluster(configFile, clusterType, force, skipPrecheck)
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
		force, _ := cmd.Flags().GetBool("force")

		// 验证配置文件存在
		if !utils.FileExists(configFile) {
			utils.PrintError("Config file %s does not exist", configFile)
			os.Exit(1)
		}

		// 移除集群
		err := cluster.RemoveCluster(configFile, force)
		if err != nil {
			utils.PrintError("Failed to remove cluster: %v", err)
			os.Exit(1)
		}

		utils.PrintSuccess("Cluster removed successfully")
	},
}

func init() {
	// 创建命令
	clusterCreateCmd.Flags().StringP("file", "f", "", "Cluster configuration file (required)")
	clusterCreateCmd.Flags().String("cluster-type", "", "Override cluster type in config (k8s|swarm)")
	clusterCreateCmd.Flags().Bool("force", false, "Force creation even if prechecks fail")
	clusterCreateCmd.Flags().Bool("skip-precheck", false, "Skip pre-installation checks")
	_ = clusterCreateCmd.MarkFlagRequired("file")

	// 移除命令
	clusterRemoveCmd.Flags().StringP("file", "f", "", "Cluster configuration file (required)")
	clusterRemoveCmd.Flags().Bool("force", false, "Force removal without confirmation")
	_ = clusterRemoveCmd.MarkFlagRequired("file")

	// 添加子命令
	clusterCmd.AddCommand(clusterCreateCmd)
	clusterCmd.AddCommand(clusterRemoveCmd)

	// 添加到根命令
	rootCmd.AddCommand(clusterCmd)
}
