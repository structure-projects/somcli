package cluster

import (
	"fmt"
)

// CreateCluster 创建集群
func CreateCluster(configFile, clusterType string, force, skipPrecheck bool) error {
	config, err := LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 如果命令行指定了集群类型，则覆盖配置文件中的设置
	if clusterType != "" {
		config.Cluster.Type = clusterType
	}

	switch config.Cluster.Type {
	case "k8s":
		return CreateK8sCluster(config, force, skipPrecheck)
	case "swarm":
		return CreateSwarmCluster(config, force, skipPrecheck)
	default:
		return fmt.Errorf("unsupported cluster type: %s", config.Cluster.Type)
	}
}

// RemoveCluster 移除集群
func RemoveCluster(configFile string, force bool) error {
	config, err := LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	switch config.Cluster.Type {
	case "k8s":
		return RemoveK8sCluster(config, force)
	case "swarm":
		return RemoveSwarmCluster(config, force)
	default:
		return fmt.Errorf("unsupported cluster type: %s", config.Cluster.Type)
	}
}
