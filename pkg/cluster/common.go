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
	"os"
	"strconv"
	"strings"

	"github.com/structure-projects/somcli/pkg/docker"
	"github.com/structure-projects/somcli/pkg/utils"
	"gopkg.in/yaml.v2"
)

// ClusterConfig 集群配置结构体
type ClusterConfig struct {
	Cluster struct {
		Type        string      `yaml:"type"`
		Name        string      `yaml:"name"`
		Nodes       []Node      `yaml:"nodes"`
		K8sConfig   K8sConfig   `yaml:"k8sConfig,omitempty"`
		SwarmConfig SwarmConfig `yaml:"swarmConfig,omitempty"`
	} `yaml:"cluster"`
}

// Node 节点配置
type Node struct {
	Host   string `yaml:"host"`
	IP     string `yaml:"ip"`
	Role   string `yaml:"role"`
	User   string `yaml:"user"`
	SSHKey string `yaml:"sshKey"`
}

type K8sConfig struct {
	Version           string `yaml:"version"`
	PodNetworkCidr    string `yaml:"podNetworkCidr"`
	ServiceCidr       string `yaml:"serviceCidr"`
	ContainerdVersion string `yaml:"containerdVersion"`
	RuncVersion       string `yaml:"runcVersion"`
	CniVersion        string `yaml:"cniVersion"`
}

type SwarmConfig struct {
	AdvertiseAddr   string   `yaml:"advertiseAddr"`
	ListenAddr      string   `yaml:"listenAddr"`
	DefaultAddrPool []string `yaml:"defaultAddrPool"`
	SubnetSize      int      `yaml:"subnetSize"`
	DataPathPort    int      `yaml:"dataPathPort"`
}

// LoadConfig 加载集群配置文件
func LoadConfig(configFile string) (*ClusterConfig, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config ClusterConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// 验证配置
	if config.Cluster.Type == "" {
		return nil, fmt.Errorf("cluster type must be specified")
	}

	if len(config.Cluster.Nodes) == 0 {
		return nil, fmt.Errorf("at least one node must be specified")
	}

	return &config, nil
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

// RunCommandOnNode 在节点上执行命令（改为接收指针）
func RunCommandOnNode(node *Node, command string) (string, error) {
	if node.Host == "localhost" || node.Host == "127.0.0.1" {
		return utils.RunCommandWithOutput("sh", "-c", command)
	}

	sshKey := utils.ExpandPath(node.SSHKey)
	output, err := utils.SSHExec(node.User, node.Host, sshKey, command)
	if err != nil {
		return "", fmt.Errorf("failed to execute command '%s' on node %s (%s@%s): %w",
			command, node.Host, node.User, node.IP, err)
	}
	return strings.TrimSpace(string(output)), nil
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
func configureFirewall(node *Node) error {
	utils.PrintInfo("Configuring firewall on node %s...", node.Host)

	commands := []string{
		"systemctl stop firewalld || true",
		"systemctl disable firewalld || true",
		"ufw disable || true",
	}

	for _, cmd := range commands {
		if output, err := RunCommandOnNode(node, cmd); err != nil {
			utils.PrintWarning("Firewall command failed on node %s: %v\nOutput: %s", node.Host, err, output)
			return fmt.Errorf("firewall configuration failed")
		}
	}
	return nil
}

// configureHostsFile 配置节点hosts文件
func configureHostsFile(node *Node, entries string) error {
	utils.PrintInfo("Configuring hosts file on node %s...", node.Host)

	// 标记标识
	markerStart := "# ===== Cluster Nodes Start ====="
	markerEnd := "# ===== Cluster Nodes End ====="
	hostsContent := fmt.Sprintf("\n%s\n%s\n%s\n", markerStart, entries, markerEnd)

	// 1. 备份原有hosts文件
	if _, err := RunCommandOnNode(node, "cp /etc/hosts /etc/hosts.bak"); err != nil {
		return fmt.Errorf("failed to backup hosts file: %w", err)
	}

	// 2. 清理旧配置
	cleanCmd := fmt.Sprintf("sed -i '/%s/,/%s/d' /etc/hosts",
		strings.ReplaceAll(markerStart, "#", `\#`),
		strings.ReplaceAll(markerEnd, "#", `\#`))
	if _, err := RunCommandOnNode(node, cleanCmd); err != nil {
		return fmt.Errorf("failed to clean old hosts entries: %w", err)
	}

	// 3. 添加新配置
	cmd := fmt.Sprintf(`echo "%s" >> /etc/hosts`, strings.ReplaceAll(hostsContent, "\"", "\\\""))
	if _, err := RunCommandOnNode(node, cmd); err != nil {
		return fmt.Errorf("failed to update hosts file: %w", err)
	}

	// 4. 验证配置
	verifyCmd := fmt.Sprintf("grep -q '%s' /etc/hosts || echo 'failed'", markerStart)
	if output, err := RunCommandOnNode(node, verifyCmd); err != nil || strings.TrimSpace(output) == "failed" {
		return fmt.Errorf("hosts file verification failed")
	}

	return nil
}

func isValidIP(ip string) bool {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if num, err := strconv.Atoi(part); err != nil || num < 0 || num > 255 {
			return false
		}
	}
	return true
}

func convertToRemoteNode(node Node) docker.RemoteNode {
	if !isValidIP(node.IP) {
		utils.PrintWarning("Invalid IP address format for node %s: %s", node.Host, node.IP)
	}
	return docker.RemoteNode{
		IP:      node.IP,
		User:    node.User,
		SSHKey:  node.SSHKey,
		IsLocal: false,
	}
}
