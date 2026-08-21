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

// ClusterSpec 一套集群的定义。配置里 cluster: 是它的列表，
// 一份配置可以同时描述多套集群（my-swarm / my-k8s），由 --cluster-name / --cluster-type 选定。
type ClusterSpec struct {
	Type        string       `yaml:"type"`
	Name        string       `yaml:"name"`
	Nodes       []RemoteNode `yaml:"nodes"`
	K8sConfig   K8sConfig    `yaml:"k8sConfig,omitempty"`
	SwarmConfig SwarmConfig  `yaml:"swarmConfig,omitempty"`
}

// ClusterConfig 是"已选定的那一套集群"，集群逻辑只关心这个。
// 从配置里挑哪一套由 cluster.LoadConfig 决定。
type ClusterConfig struct {
	Cluster ClusterSpec
}

type K8sConfig struct {
	Version           string `yaml:"version"`
	PodNetworkCidr    string `yaml:"podNetworkCidr"`
	ServiceCidr       string `yaml:"serviceCidr"`
	DockerVersion     string `yaml:"dockerVersion"`
	ContainerdVersion string `yaml:"containerdVersion"`
	ContainerRuntime  string `yaml:"containerRuntime"` // "docker" 或 "containerd"
	// ControlPlaneEndpoint 是 apiserver 的稳定入口（VIP 或负载均衡），形如 "10.0.0.100:6443"。
	// 多 master 时必填：三个 master 各有各的 IP，kubeconfig 与证书必须指向一个不随单机存亡的地址，
	// 否则第一个 master 一挂，集群就没了入口。单 master 留空即可。
	ControlPlaneEndpoint string `yaml:"controlPlaneEndpoint"`
	// Cni 选网络插件："flannel"（默认）或 "calico"。
	// 不装 CNI 的集群节点会永久 NotReady，所以这里没有"不装"这个选项。
	Cni string `yaml:"cni"`
	// CniVersion 是网络插件自身的版本（不是 cniPluginsVersion —— 那个是
	// /opt/cni/bin 下的二进制插件包）。留空则用 somcli 内置的默认版本。
	CniVersion        string `yaml:"cniVersion"`
	ImageRepository   string `yaml:"imageRepository"` // 镜像仓库地址 registry.aliyuncs.com/google_containers
	PauseImageVersion string `yaml:"pauseImageVersion"`
	CniPluginsVersion string `yaml:"cniPluginsVersion"`
	RuncVersion       string `yaml:"runcVersion"`
	// Resources 指定这套集群按什么顺序装哪些东西，按 configs/k8s/ 里的资源名引用。
	// 留空则按 containerRuntime 取默认组合（见 defaultK8sResources），
	// 想加一样自己的东西时把默认那几项抄下来再往后追加。
	// 网络插件不写在这里：它由 cni 选定，且必须夹在 kubeadm init 与 worker 加入之间。
	Resources []string `yaml:"resources"`
}

type SwarmConfig struct {
	AdvertiseAddr   string   `yaml:"advertiseAddr"`
	ListenAddr      string   `yaml:"listenAddr"`
	DefaultAddrPool []string `yaml:"defaultAddrPool"`
	SubnetSize      int      `yaml:"subnetSize"`
	DataPathPort    int      `yaml:"dataPathPort"`
}
