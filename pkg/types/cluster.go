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
package types

// ClusterConfig 集群配置结构体
type ClusterConfig struct {
	Cluster struct {
		Type        string       `yaml:"type"`
		Name        string       `yaml:"name"`
		Nodes       []RemoteNode `yaml:"nodes"`
		K8sConfig   K8sConfig    `yaml:"k8sConfig,omitempty"`
		SwarmConfig SwarmConfig  `yaml:"swarmConfig,omitempty"`
	} `yaml:"cluster"`
}

type K8sConfig struct {
	Version           string `yaml:"version"`
	PodNetworkCidr    string `yaml:"podNetworkCidr"`
	ServiceCidr       string `yaml:"serviceCidr"`
	DockerVersion     string `yaml:"dockerVersion"`
	ContainerdVersion string `yaml:"containerdVersion"`
	ContainerRuntime  string `yaml:"containerRuntime"` // "docker" 或 "containerd"
	ImageRepository   string `yaml:"imageRepository"`  // 镜像仓库地址 registry.aliyuncs.com/google_containers
	PauseImageVersion string `yaml:"pauseImageVersion"`
	CniPluginsVersion string `yaml:"cniPluginsVersion"`
	RuncVersion       string `yaml:"runcVersion"`
}

type SwarmConfig struct {
	AdvertiseAddr   string   `yaml:"advertiseAddr"`
	ListenAddr      string   `yaml:"listenAddr"`
	DefaultAddrPool []string `yaml:"defaultAddrPool"`
	SubnetSize      int      `yaml:"subnetSize"`
	DataPathPort    int      `yaml:"dataPathPort"`
}
