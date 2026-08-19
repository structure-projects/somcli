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
	"strings"

	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
)

// LoadConfig 从统一配置里挑出要操作的那一套集群。
//
// 配置的 cluster: 是列表，一份文件可以同时描述 my-swarm 与 my-k8s；
// 挑哪一套按 name -> type -> "只有一套就是它" 依次判定，都定不下来就报错并列出候选，
// 而不是默默拿第一套去装。
func LoadConfig(configFile, name, clusterType string) (*types.ClusterConfig, error) {
	config, err := utils.LoadConfig(configFile)
	if err != nil {
		return nil, err
	}

	spec, err := selectCluster(config.Clusters, name, clusterType)
	if err != nil {
		return nil, err
	}

	if len(spec.Nodes) == 0 {
		return nil, fmt.Errorf("集群 %q 没有配置任何节点", spec.Name)
	}

	// 把节点登记进全局节点表。缺了这一步，utils.GetNode 查不到任何主机，
	// 声明为远程的安装会被解析失败或误当作本机执行。
	utils.SetNode(spec.Nodes)

	return &types.ClusterConfig{Cluster: *spec}, nil
}

func selectCluster(clusters []types.ClusterSpec, name, clusterType string) (*types.ClusterSpec, error) {
	if len(clusters) == 0 {
		return nil, fmt.Errorf("配置里没有 cluster: 段，没有可操作的集群")
	}

	var matched []*types.ClusterSpec
	for i := range clusters {
		c := &clusters[i]
		if name != "" && c.Name != name {
			continue
		}
		if clusterType != "" && c.Type != clusterType {
			continue
		}
		matched = append(matched, c)
	}

	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return nil, fmt.Errorf("配置里没有匹配的集群（name=%q type=%q），可选：%s",
			name, clusterType, describeClusters(clusters))
	default:
		return nil, fmt.Errorf("配置里有 %d 套集群同时匹配，请用 --cluster-name 指定：%s",
			len(matched), describeClusters(clusters))
	}
}

func describeClusters(clusters []types.ClusterSpec) string {
	var parts []string
	for _, c := range clusters {
		parts = append(parts, fmt.Sprintf("%s(%s)", c.Name, c.Type))
	}
	return strings.Join(parts, ", ")
}

func EnsureWorkDir() error {
	workDir := utils.GetWorkDir()
	if !utils.FileExists(workDir) {
		if err := utils.CreateDir(workDir); err != nil {
			return fmt.Errorf("failed to create work directory: %w", err)
		}
	}
	return nil
}

func GetClusterTypeName(t ClusterType) string {
	switch t {
	case TypeK8s:
		return "Kubernetes"
	case TypeSwarm:
		return "Docker Swarm"
	case TypeDocker:
		return "Docker"
	default:
		return "Unknown"
	}
}

// IsValidClusterType 检查集群类型是否有效
func IsValidClusterType(t string) bool {
	switch t {
	case TypeK8s, TypeSwarm, TypeDocker:
		return true
	default:
		return false
	}
}

// ===================== 封装的配置函数 =====================

// configureFirewall 配置节点防火墙
func configureFirewall(node *types.RemoteNode) error {
	utils.PrintInfo("Configuring firewall on node %s...", node.Host)

	commands := []string{
		"systemctl stop firewalld || true",
		"systemctl disable firewalld || true",
		"ufw disable || true",
	}

	for _, cmd := range commands {
		if output, err := utils.RunCommandOnNode(node, cmd); err != nil {
			utils.PrintWarning("Firewall command failed on node %s: %v\nOutput: %s", node.Host, err, output)
			return fmt.Errorf("firewall configuration failed")
		}
	}
	return nil
}

// configureHostsFile 配置节点hosts文件
func configureHostsFile(node *types.RemoteNode, entries string) error {
	utils.PrintInfo("Configuring hosts file on node %s...", node.Host)

	// 标记标识
	markerStart := "# ===== Cluster Nodes Start ====="
	markerEnd := "# ===== Cluster Nodes End ====="
	hostsContent := fmt.Sprintf("\n%s\n%s\n%s\n", markerStart, entries, markerEnd)

	// 1. 备份原有hosts文件
	if _, err := utils.RunCommandOnNode(node, "cp /etc/hosts /etc/hosts.bak"); err != nil {
		return fmt.Errorf("failed to backup hosts file: %w", err)
	}

	// 2. 清理旧配置
	cleanCmd := fmt.Sprintf("sed -i '/%s/,/%s/d' /etc/hosts",
		strings.ReplaceAll(markerStart, "#", `\#`),
		strings.ReplaceAll(markerEnd, "#", `\#`))
	if _, err := utils.RunCommandOnNode(node, cleanCmd); err != nil {
		return fmt.Errorf("failed to clean old hosts entries: %w", err)
	}

	// 3. 添加新配置
	cmd := fmt.Sprintf(`echo "%s" >> /etc/hosts`, strings.ReplaceAll(hostsContent, "\"", "\\\""))
	if _, err := utils.RunCommandOnNode(node, cmd); err != nil {
		return fmt.Errorf("failed to update hosts file: %w", err)
	}

	// 4. 验证配置
	verifyCmd := fmt.Sprintf("grep -q '%s' /etc/hosts || echo 'failed'", markerStart)
	if output, err := utils.RunCommandOnNode(node, verifyCmd); err != nil || strings.TrimSpace(output) == "failed" {
		return fmt.Errorf("hosts file verification failed")
	}

	return nil
}
