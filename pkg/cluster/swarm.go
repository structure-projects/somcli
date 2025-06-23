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
package cluster

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/structure-projects/somcli/pkg/docker"
	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
)

// CreateSwarmCluster 创建 Docker Swarm 集群
func CreateSwarmCluster(config *types.ClusterConfig, force bool, skipPrecheck bool) error {
	utils.PrintBanner("Creating Docker Swarm Cluster: " + config.Cluster.Name)

	// 1. 验证集群配置
	if err := validateClusterConfig(config); err != nil {
		return fmt.Errorf("invalid cluster configuration: %w", err)
	}

	// 2. 打印节点信息
	utils.PrintInfo("Cluster nodes configuration:")
	for _, node := range config.Cluster.Nodes {
		utils.PrintInfo("Node: %s, IP: %s, Role: %s", node.Host, node.IP, node.Role)
	}

	// 3. 准备工作（包含防火墙和hosts配置）
	if err := prepareSwarmCluster(config, skipPrecheck); err != nil {
		return err
	}

	// 4. 初始化 Swarm
	masterNode := findManagerNode(config)
	if masterNode == nil {
		var roles []string
		for _, node := range config.Cluster.Nodes {
			roles = append(roles, node.Role)
		}
		return fmt.Errorf("no manager node found in configuration. Existing roles: %v", roles)
	}

	if err := initSwarm(masterNode, config); err != nil {
		return err
	}

	// 5. 加入工作节点
	if err := joinSwarmNodes(config, masterNode); err != nil {
		return err
	}

	// 6. 输出集群信息和验证指南
	if err := printClusterInfoAndGuide(config, masterNode); err != nil {
		return err
	}

	return nil
}

// ===================== 集群准备函数 =====================

func prepareSwarmCluster(config *types.ClusterConfig, skipPrecheck bool) error {
	if skipPrecheck {
		return nil
	}

	installer := docker.NewInstaller(true, true)

	// 生成所有节点的hosts记录
	var hostsEntries strings.Builder
	for _, node := range config.Cluster.Nodes {
		hostsEntries.WriteString(fmt.Sprintf("%s\t%s\n", node.IP, node.Host))
	}

	for _, node := range config.Cluster.Nodes {

		// 1. 配置防火墙
		if err := configureFirewall(&node); err != nil {
			return fmt.Errorf("firewall configuration failed for node %s: %w", node.Host, err)
		}

		// 2. 检查并安装 Docker
		if _, err := utils.RunCommandOnNode(&node, "docker --version"); err != nil {
			utils.PrintInfo("Installing Docker on node %s...", node.Host)
			if err := installer.Install("latest", node); err != nil {
				return fmt.Errorf("failed to install Docker on node %s: %w", node.Host, err)
			}
		}

		// 3. 启动 Docker 服务
		if _, err := utils.RunCommandOnNode(&node, "systemctl start docker"); err != nil {
			return fmt.Errorf("failed to start Docker on node %s: %w", node.Host, err)
		}

		// 4. 配置hosts文件
		if err := configureHostsFile(&node, hostsEntries.String()); err != nil {
			return fmt.Errorf("failed to configure hosts file on node %s: %w", node.Host, err)
		}

		// 5. 检查网络连通性
		utils.PrintInfo("Checking network connectivity for node %s...", node.Host)
		for _, peer := range config.Cluster.Nodes {
			if peer.Host == node.Host {
				continue
			}
			checkCmd := fmt.Sprintf("ping -c 1 -W 1 %s", peer.IP)
			if output, err := utils.RunCommandOnNode(&node, checkCmd); err != nil {
				utils.PrintWarning("Node %s cannot reach %s (%s)\nOutput: %s",
					node.Host, peer.Host, peer.IP, output)
			}
		}
	}

	return nil
}

// ===================== 其他辅助函数 =====================

func validateClusterConfig(config *types.ClusterConfig) error {
	if config.Cluster.Name == "" {
		return fmt.Errorf("cluster name cannot be empty")
	}

	if len(config.Cluster.Nodes) == 0 {
		return fmt.Errorf("no nodes defined in cluster configuration")
	}

	managerCount := 0
	for _, node := range config.Cluster.Nodes {
		if !utils.IsValidIP(node.IP) {
			return fmt.Errorf("invalid IP address format for node %s: %s", node.Host, node.IP)
		}

		if strings.ToLower(node.Role) == "manager" {
			managerCount++
		}
	}

	if managerCount == 0 {
		return fmt.Errorf("at least one manager node is required")
	}

	return nil
}

func findManagerNode(config *types.ClusterConfig) *types.RemoteNode {
	for i := range config.Cluster.Nodes {
		if strings.ToLower(config.Cluster.Nodes[i].Role) == "manager" {
			return &config.Cluster.Nodes[i]
		}
	}
	return nil
}

func initSwarm(node *types.RemoteNode, config *types.ClusterConfig) error {
	utils.PrintInfo("Initializing Swarm on manager node %s...", node.Host)

	initCmd := fmt.Sprintf(
		"docker swarm init --advertise-addr %s --listen-addr %s",
		config.Cluster.SwarmConfig.AdvertiseAddr,
		config.Cluster.SwarmConfig.ListenAddr,
	)

	output, err := utils.RunCommandOnNode(node, initCmd)
	if err != nil {
		return fmt.Errorf("failed to initialize swarm: %w\nOutput: %s", err, output)
	}

	joinTokens, err := extractSwarmJoinTokens(node)
	if err != nil {
		return fmt.Errorf("failed to get swarm join tokens: %w", err)
	}

	joinFile := filepath.Join(utils.GetWorkDir(), "swarm-join-command.txt")
	joinContent := fmt.Sprintf("Manager: %s\nWorker: %s",
		joinTokens["manager"],
		joinTokens["worker"])

	if err := utils.WriteStringToFile(joinFile, joinContent); err != nil {
		return fmt.Errorf("failed to save join command: %w", err)
	}

	utils.PrintSuccess("Swarm initialized successfully")
	return nil
}

func joinSwarmNodes(config *types.ClusterConfig, masterNode *types.RemoteNode) error {
	joinFile := filepath.Join(utils.GetWorkDir(), "swarm-join-command.txt")
	joinContent, err := utils.ReadFileToString(joinFile)
	if err != nil {
		return fmt.Errorf("failed to read join command: %w", err)
	}

	// 确保使用IP地址而非主机名
	joinContent = strings.ReplaceAll(joinContent, masterNode.Host, masterNode.IP)

	joinTokens := make(map[string]string)
	lines := strings.Split(joinContent, "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if key != "" && value != "" {
				joinTokens[key] = value
			}
		}
	}

	if joinTokens["Manager"] == "" || joinTokens["Worker"] == "" {
		return fmt.Errorf("invalid join tokens. Manager: %t, Worker: %t",
			joinTokens["Manager"] != "", joinTokens["Worker"] != "")
	}

	for _, node := range config.Cluster.Nodes {
		if node.Host == masterNode.Host {
			continue
		}

		utils.PrintInfo("\nJoining node %s (%s) as %s...", node.Host, node.IP, node.Role)

		var joinCmd string
		switch strings.ToLower(node.Role) {
		case "manager":
			joinCmd = joinTokens["Manager"]
		case "worker":
			joinCmd = joinTokens["Worker"]
		default:
			return fmt.Errorf("unknown node role: %s", node.Role)
		}

		output, err := utils.RunCommandOnNode(&node, joinCmd)
		if err != nil {
			return fmt.Errorf("failed to join node %s: %w\nCommand: %s\nOutput: %s",
				node.Host, err, joinCmd, output)
		}

		if !strings.Contains(output, "This node joined a swarm") {
			return fmt.Errorf("node %s may not have joined successfully. Output: %s",
				node.Host, output)
		}

		utils.PrintSuccess("Node %s joined successfully as %s", node.Host, node.Role)
	}

	return nil
}

func extractSwarmJoinTokens(node *types.RemoteNode) (map[string]string, error) {
	tokens := make(map[string]string)

	managerOutput, err := utils.RunCommandOnNode(node, "docker swarm join-token manager")
	if err != nil {
		return nil, fmt.Errorf("failed to get manager token: %w\nOutput: %s", err, managerOutput)
	}
	managerToken := extractTokenFromOutput(managerOutput)
	if managerToken == "" {
		return nil, fmt.Errorf("empty manager token, output: %s", managerOutput)
	}
	tokens["manager"] = managerToken

	workerOutput, err := utils.RunCommandOnNode(node, "docker swarm join-token worker")
	if err != nil {
		return nil, fmt.Errorf("failed to get worker token: %w\nOutput: %s", err, workerOutput)
	}
	workerToken := extractTokenFromOutput(workerOutput)
	if workerToken == "" {
		return nil, fmt.Errorf("empty worker token, output: %s", workerOutput)
	}
	tokens["worker"] = workerToken

	return tokens, nil
}

func extractTokenFromOutput(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "docker swarm join") {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func printClusterInfoAndGuide(config *types.ClusterConfig, masterNode *types.RemoteNode) error {
	output, err := utils.RunCommandOnNode(masterNode, "docker node ls")
	if err != nil {
		return fmt.Errorf("failed to get cluster nodes: %w", err)
	}

	utils.PrintSuccess("\nDocker Swarm Cluster created successfully!")
	utils.PrintInfo("\n=== Cluster Information ===")
	utils.PrintInfo("Cluster Name: %s", config.Cluster.Name)
	utils.PrintInfo("Manager Node: %s (%s)", masterNode.Host, masterNode.IP)
	utils.PrintInfo("\nCluster Nodes:")
	fmt.Println(output)

	utils.PrintInfo("\n=== Hosts Configuration ===")
	utils.PrintInfo("Nodes' /etc/hosts has been configured with following entries:")
	utils.PrintInfo("# ===== Cluster Nodes Start =====")
	for _, node := range config.Cluster.Nodes {
		utils.PrintInfo("%s\t%s", node.IP, node.Host)
	}
	utils.PrintInfo("# ===== Cluster Nodes End =====")

	utils.PrintInfo("\n=== Verification Guide ===")
	utils.PrintInfo("1. Check cluster status:")
	utils.PrintInfo("   docker node ls")
	utils.PrintInfo("   docker service ls")

	utils.PrintInfo("\n2. Test hosts resolution:")
	utils.PrintInfo("   ping %s", config.Cluster.Nodes[0].Host)

	utils.PrintInfo("\n3. Deploy test service:")
	utils.PrintInfo("   docker service create --name test --replicas 2 nginx")

	infoFile := filepath.Join(utils.GetWorkDir(), "cluster-info.txt")
	infoContent := fmt.Sprintf("Cluster Name: %s\nManager Node: %s\n\nNodes:\n%s\n\nHosts Entries:\n%s",
		config.Cluster.Name, masterNode.Host, output, getHostsEntries(config))
	if err := utils.WriteStringToFile(infoFile, infoContent); err != nil {
		utils.PrintWarning("Failed to save cluster info: %v", err)
	} else {
		utils.PrintInfo("\nCluster information saved to: %s", infoFile)
	}

	return nil
}

func getHostsEntries(config *types.ClusterConfig) string {
	var builder strings.Builder
	builder.WriteString("# ===== Cluster Nodes Start =====\n")
	for _, node := range config.Cluster.Nodes {
		builder.WriteString(fmt.Sprintf("%s\t%s\n", node.IP, node.Host))
	}
	builder.WriteString("# ===== Cluster Nodes End =====")
	return builder.String()
}

func RemoveSwarmCluster(config *types.ClusterConfig, force bool) error {
	utils.PrintBanner("Removing Docker Swarm Cluster: " + config.Cluster.Name)

	if !force {
		if !utils.AskForConfirmation("Are you sure you want to remove the Swarm cluster?") {
			return fmt.Errorf("cluster removal cancelled")
		}
	}

	installer := docker.NewInstaller(false, false)

	for _, node := range config.Cluster.Nodes {
		utils.PrintInfo("Processing node %s...", node.Host)

		// 离开 Swarm 集群
		utils.PrintInfo("Leaving Swarm on node %s...", node.Host)
		var leaveCmd string
		if strings.ToLower(node.Role) == "manager" {
			leaveCmd = "docker swarm leave --force"
		} else {
			leaveCmd = "docker swarm leave"
		}

		if _, err := utils.RunCommandOnNode(&node, leaveCmd); err != nil {
			utils.PrintWarning("Failed to leave swarm on node %s: %v", node.Host, err)
		} else {
			utils.PrintSuccess("Node %s left swarm successfully", node.Host)
		}

		// 可选：卸载 Docker
		if force {
			utils.PrintInfo("Uninstalling Docker from node %s...", node.Host)
			if err := installer.Uninstall(node); err != nil {
				utils.PrintWarning("Failed to uninstall Docker from node %s: %v", node.Host, err)
			} else {
				utils.PrintSuccess("Docker uninstalled from node %s", node.Host)
			}
		}
	}

	return nil
}
