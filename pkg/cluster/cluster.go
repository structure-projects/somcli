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
