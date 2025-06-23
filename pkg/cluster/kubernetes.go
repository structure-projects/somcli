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
	"strconv"
	"strings"
	"time"

	"github.com/structure-projects/somcli/pkg/installer"
	"github.com/structure-projects/somcli/pkg/types"
	"github.com/structure-projects/somcli/pkg/utils"
)

const (
	containerdServiceTemplate = `[Unit]
Description=containerd container runtime
Documentation=https://containerd.io
After=network.target local-fs.target

[Service]
ExecStartPre=-/sbin/modprobe overlay
ExecStart=/usr/local/bin/containerd
Restart=always
RestartSec=5
Delegate=yes
KillMode=process
OOMScoreAdjust=-999
LimitNOFILE=1048576
LimitNPROC=infinity
LimitCORE=infinity

[Install]
WantedBy=multi-user.target`
)

// CreateK8sCluster 创建Kubernetes集群
func CreateK8sCluster(config *types.ClusterConfig, force bool, skipPrecheck bool) error {
	startTime := time.Now()
	utils.PrintBanner(fmt.Sprintf("正在创建Kubernetes集群: %s", config.Cluster.Name))
	utils.PrintInfo("开始时间: %s", startTime.Format("2006-01-02 15:04:05"))
	utils.PrintInfo("集群配置详情:")
	utils.PrintInfo("  集群名称: %s", config.Cluster.Name)
	utils.PrintInfo("  Kubernetes版本: %s", config.Cluster.K8sConfig.Version)
	utils.PrintInfo("  Pod网络CIDR: %s", config.Cluster.K8sConfig.PodNetworkCidr)
	utils.PrintInfo("  服务CIDR: %s", config.Cluster.K8sConfig.ServiceCidr)
	utils.PrintInfo("  容器运行时: %s", config.Cluster.K8sConfig.ContainerRuntime)
	utils.PrintInfo("  Docker版本: %s", config.Cluster.K8sConfig.DockerVersion)
	utils.PrintInfo("  Containerd版本: %s", config.Cluster.K8sConfig.ContainerdVersion)

	// 1. 准备阶段
	utils.PrintStage("== 集群准备阶段 ==")
	if err := prepareK8sCluster(config, skipPrecheck); err != nil {
		utils.PrintError("集群准备失败: %v", err)
		return fmt.Errorf("集群准备失败: %w", err)
	}
	utils.PrintSuccess("✓ 集群准备完成")

	// 2. 依赖安装阶段
	if err := installDependencies(config); err != nil {
		utils.PrintError("依赖安装失败: %v", err)
		return fmt.Errorf("依赖安装失败: %w", err)
	}
	utils.PrintSuccess("✓ 依赖安装完成")

	// 3. 主节点初始化
	utils.PrintStage("== 主节点初始化 ==")
	masterNode := findFirstMasterNode(config)
	if masterNode == nil {
		err := fmt.Errorf("配置中没有找到主节点")
		utils.PrintError("未找到主节点: %v", err)
		return err
	}
	utils.PrintInfo("已选择主节点: %s (%s)", masterNode.Host, masterNode.IP)

	if err := initK8sMaster(masterNode, config); err != nil {
		utils.PrintError("主节点初始化失败: %v", err)
		return fmt.Errorf("主节点初始化失败: %w", err)
	}
	utils.PrintSuccess("✓ 主节点初始化完成")

	// 4. 工作节点加入
	utils.PrintStage("== 工作节点加入 ==")
	if err := joinWorkerNodes(config); err != nil {
		utils.PrintError("工作节点加入失败: %v", err)
		return fmt.Errorf("工作节点加入失败: %w", err)
	}
	utils.PrintSuccess("✓ 工作节点加入完成")

	// 5. 集群信息展示
	utils.PrintStage("== 集群信息展示 ==")
	if err := printK8sClusterInfo(config, masterNode); err != nil {
		utils.PrintError("集群信息展示失败: %v", err)
		return fmt.Errorf("集群信息展示失败: %w", err)
	}

	duration := time.Since(startTime)
	utils.PrintSuccess("\n✓ Kubernetes集群 '%s' 创建成功!", config.Cluster.Name)
	utils.PrintInfo("总执行时间: %v", duration.Round(time.Second))

	return nil
}

// installDependencies
func installDependencies(config *types.ClusterConfig) error {
	utils.PrintInfo("正在准备安装Kubernetes %s...", config.Cluster.K8sConfig.Version)

	// 获取所有节点IP
	hosts := getAllNodesIP(config)

	// 1. 安装基础依赖
	if err := installBaseDependencies(config, hosts); err != nil {
		return err
	}

	// 2. 安装容器运行时
	runtime := config.Cluster.K8sConfig.ContainerRuntime
	if runtime == "" {
		runtime = "containerd" // 默认使用containerd
	}

	switch runtime {
	case "docker":
		if err := installDocker(config, hosts); err != nil {
			return err
		}

	case "containerd":
		if err := installContainerd(config, hosts); err != nil {
			return err
		}
	default:
		return fmt.Errorf("不支持的容器运行时: %s", runtime)
	}

	// 3. 安装Kubernetes组件
	return installK8sComponents(config, hosts)
}

// installDocker 安装Docker
func installDocker(config *types.ClusterConfig, hosts []string) error {
	utils.PrintInfo("正在安装Docker...")

	dockerVersion := config.Cluster.K8sConfig.DockerVersion

	// 定义Docker资源
	dockerResource := types.Resource{
		Name:    "docker",
		Version: dockerVersion,
		Method:  "binary",
		URLs: []string{
			"https://download.docker.com/linux/static/stable/x86_64/docker-{{.Version}}.tgz",
		},
		PostInstall: []string{
			" tar xzvf {{.CacheDir}}/{{.Name}}-{{.Version}}.tgz -C /usr/local/bin",
			" chmod +x /usr/local/bin/docker*",
			" groupadd docker || true",
			" usermod -aG docker $USER",
			" mkdir -p /etc/docker",
			" systemctl enable docker",
			" systemctl start docker",
		},
		ExtraFiles: map[string]string{
			"/etc/docker/daemon.json": `{
                "exec-opts": ["native.cgroupdriver=systemd"],
                "log-driver": "json-file",
                "log-opts": {"max-size": "100m"},
                "storage-driver": "overlay2"
            }`,
		},
		Hosts:  hosts,
		Target: "{{.Name}}-{{.Version}}.tgz",
	}

	installer := installer.NewInstaller()

	return installer.Install(dockerResource, true)
}

// installContainerd 安装Containerd
func installContainerd(config *types.ClusterConfig, hosts []string) error {
	utils.PrintInfo("正在安装Containerd...")

	installer := installer.NewInstaller()

	// 定义CNI资源
	cniResource := types.Resource{
		Name:    "containerd",
		Version: config.Cluster.K8sConfig.CniPluginsVersion,
		Method:  "binary",
		URLs: []string{
			"https://github.com/containernetworking/plugins/releases/download/v{{.Version}}/cni-plugins-linux-amd64-v{{.Version}}.tgz",
		},
		PostInstall: []string{
			" mkdir -p /opt/cni/bin",
			" tar Cxzvf /opt/cni/bin {{.CacheDir}}/cni-plugins-linux-amd64-v{{.Version}}.tgz",
		},
		Hosts:  hosts,
		Target: "{{.Filename}}",
	}
	installer.Install(cniResource, false)

	// 定义Containerd资源
	runcResource := types.Resource{
		Name:    "runc",
		Version: config.Cluster.K8sConfig.RuncVersion,
		Method:  "binary",
		URLs: []string{
			"https://github.com/opencontainers/runc/releases/download/v{{.Version}}/runc.amd64",
		},
		PostInstall: []string{
			" install -m 755 {{.CacheDir}}/runc.amd64 /usr/local/sbin/runc",
		},
		Hosts:  hosts,
		Target: "{{.Filename}}",
	}

	installer.Install(runcResource, false)

	// 定义Containerd资源
	containerdResource := types.Resource{
		Name:    "containerd",
		Version: config.Cluster.K8sConfig.ContainerdVersion,
		Method:  "binary",
		URLs: []string{
			"https://github.com/containerd/containerd/releases/download/v{{.Version}}/containerd-{{.Version}}-linux-amd64.tar.gz",
		},
		PostInstall: []string{
			"tar Cxzvf /usr/local {{.CacheDir}}/containerd-{{.Version}}-linux-amd64.tar.gz",
			"mkdir -p /etc/containerd",
			"containerd config default |  tee /etc/containerd/config.toml >/dev/null",
			"sed -i 's|k8s.gcr.io|" + config.Cluster.K8sConfig.ImageRepository + "|g' /etc/containerd/config.toml",
			fmt.Sprintf(" sed -i 's|sandbox_image = \".*\"|sandbox_image = \"%s/pause:%s\"|g' /etc/containerd/config.toml",
				config.Cluster.K8sConfig.ImageRepository, config.Cluster.K8sConfig.PauseImageVersion),
			"systemctl daemon-reload",
			"systemctl enable --now containerd",
		},
		ExtraFiles: map[string]string{
			"/etc/systemd/system/containerd.service": containerdServiceTemplate,
		},
		Hosts:  hosts,
		Target: "{{.Filename}}",
	}

	return installer.Install(containerdResource, false)
}

// installK8sComponents 安装Kubernetes组件
func installK8sComponents(config *types.ClusterConfig, hosts []string) error {
	utils.PrintInfo("正在安装Kubernetes组件...")

	k8sVersion := config.Cluster.K8sConfig.Version

	// 定义Kubernetes组件资源
	k8sResource := types.Resource{
		Name:    "kubernetes",
		Version: k8sVersion,
		Method:  "binary",
		URLs: []string{
			"https://dl.k8s.io/v{{.Version}}/bin/linux/amd64/kubeadm",
			"https://dl.k8s.io/v{{.Version}}/bin/linux/amd64/kubelet",
			"https://dl.k8s.io/v{{.Version}}/bin/linux/amd64/kubectl",
			"https://structured.oss-cn-beijing.aliyuncs.com/somwork/service/kubelet.service",
		},
		PostInstall: []string{
			" install -o root -g root -m 0755 {{.CacheDir}}/kubeadm /usr/local/bin/kubeadm",
			" install -o root -g root -m 0755 {{.CacheDir}}/kubelet /usr/local/bin/kubelet",
			" install -o root -g root -m 0755 {{.CacheDir}}/kubectl /usr/local/bin/kubectl",
			" mkdir -p /etc/systemd/system/kubelet.service.d",
			" install -o root -g root -m 0644 {{.CacheDir}}/kubelet.service /etc/systemd/system/kubelet.service",
			" systemctl daemon-reload",
			" systemctl enable --now kubelet",
		},
		Hosts:  hosts,
		Target: "{{.Filename}}",
	}

	installer := installer.NewInstaller()
	return installer.Install(k8sResource, false)
}

// initK8sMaster 初始化 Kubernetes 主节点
func initK8sMaster(node *types.RemoteNode, config *types.ClusterConfig) error {
	// 构建初始化命令
	initCmd := fmt.Sprintf(
		"kubeadm init --kubernetes-version=%s --apiserver-advertise-address=%s --pod-network-cidr=%s --service-cidr=%s",
		config.Cluster.K8sConfig.Version,
		node.IP,
		config.Cluster.K8sConfig.PodNetworkCidr,
		config.Cluster.K8sConfig.ServiceCidr,
	)

	// 添加容器运行时配置
	runtime := config.Cluster.K8sConfig.ContainerRuntime
	if runtime == "" {
		runtime = "containerd"
	}

	if runtime == "docker" {
		initCmd += " --cri-socket unix:///var/run/dockershim.sock"
	} else {
		initCmd += " --cri-socket unix:///var/run/containerd/containerd.sock"
	}

	// 添加镜像仓库配置
	if config.Cluster.K8sConfig.ImageRepository != "" {
		initCmd += fmt.Sprintf(" --image-repository=%s", config.Cluster.K8sConfig.ImageRepository)
	}

	utils.PrintInfo("正在使用以下命令初始化主节点:")
	utils.PrintInfo("  %s", initCmd)

	startTime := time.Now()
	output, err := utils.RunCommandOnNode(node, initCmd)
	if err != nil {
		utils.PrintError("主节点初始化失败: %v", err)
		utils.PrintDebug("命令输出:\n%s", output)
		return fmt.Errorf("主节点初始化失败: %w\n输出: %s", err, output)
	}

	joinCommand := extractJoinCommand(output)
	if joinCommand == "" {
		err := fmt.Errorf("无法从kubeadm init输出中提取加入命令")
		utils.PrintError("提取加入命令失败: %v", err)
		return err
	}

	joinFile := filepath.Join(utils.GetWorkTmpDir(), config.Cluster.Name+"_k8s-join-command.txt")
	if err := utils.WriteStringToFile(joinFile, joinCommand); err != nil {
		utils.PrintError("保存加入命令失败: %v", err)
		return fmt.Errorf("保存加入命令失败: %w", err)
	}
	utils.PrintInfo("加入命令已保存到: %s", joinFile)

	utils.PrintInfo("正在配置kubectl...")
	cmds := []string{
		"mkdir -p $HOME/.kube",
		" cp -i /etc/kubernetes/admin.conf $HOME/.kube/config",
		" chown $(id -u):$(id -g) $HOME/.kube/config",
	}

	for _, cmd := range cmds {
		if _, err := utils.RunCommandOnNode(node, cmd); err != nil {
			utils.PrintError("命令执行失败: %s: %v", cmd, err)
			return fmt.Errorf("kubectl配置失败: %w", err)
		}
	}

	// 检查是否其他主节点，同步主节点配置
	masterList := findMasterNodes(config)
	for _, masterNode := range masterList {
		if masterNode.IP != findFirstMasterNode(config).IP {
			err := joinMaster(masterNode, config)
			if nil != err {
				utils.PrintWarning("master %s join failed!", masterNode.IP)
			}
		}
	}

	duration := time.Since(startTime)
	utils.PrintSuccess("✓ 主节点初始化完成，耗时: %v", duration.Round(time.Second))
	return nil
}

// generateKubeadmConfig 生成kubeadm配置文件
func generateKubeadmConfig(node *types.RemoteNode, config *types.ClusterConfig) (string, error) {
	runtime := config.Cluster.K8sConfig.ContainerRuntime
	if runtime == "" {
		runtime = "containerd"
	}

	criSocket := "unix:///var/run/containerd/containerd.sock"
	if runtime == "docker" {
		criSocket = "unix:///var/run/dockershim.sock"
	}

	kubeadmConfig := fmt.Sprintf(`apiVersion: kubeadm.k8s.io/v1beta3
kind: InitConfiguration
nodeRegistration:
  criSocket: %s
  name: %s
---
apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
kubernetesVersion: %s
apiServer:
  certSANs:
  - "%s"
controlPlaneEndpoint: "%s:6443"
networking:
  podSubnet: "%s"
  serviceSubnet: "%s"
`, criSocket, node.Host,
		config.Cluster.K8sConfig.Version,
		node.IP, node.IP,
		config.Cluster.K8sConfig.PodNetworkCidr,
		config.Cluster.K8sConfig.ServiceCidr)

	// 添加镜像仓库配置
	if config.Cluster.K8sConfig.ImageRepository != "" {
		kubeadmConfig += fmt.Sprintf("imageRepository: %s\n", config.Cluster.K8sConfig.ImageRepository)
	}

	return kubeadmConfig, nil
}

// getAllNodesIP 获取所有节点IP
func getAllNodesIP(config *types.ClusterConfig) []string {
	hosts := []string{}
	for _, node := range config.Cluster.Nodes {
		hosts = append(hosts, node.IP)
	}
	return hosts
}

// installBaseDependencies 安装基础依赖
func installBaseDependencies(config *types.ClusterConfig, hosts []string) error {
	utils.PrintInfo("正在安装基础依赖...")

	commands := []string{
		" yum install -y socat conntrack ebtables ipset",
		" swapoff -a",
		" sed -i '/ swap / s/^/#/' /etc/fstab",
		" modprobe overlay",
		" modprobe br_netfilter",
		" sysctl --system",
	}

	baseDeps := types.Resource{
		Name:        "base-dependencies",
		Version:     "",
		Method:      "package",
		PostInstall: commands,
		Hosts:       hosts,
	}

	installer := installer.NewInstaller()
	return installer.Install(baseDeps, true)
}

// prepareK8sCluster 准备Kubernetes集群
func prepareK8sCluster(config *types.ClusterConfig, skipPrecheck bool) error {
	utils.PrintInfo("正在创建工作目录...")
	if err := EnsureWorkDir(); err != nil {
		utils.PrintError("创建工作目录失败: %v", err)
		return fmt.Errorf("创建工作目录失败: %w", err)
	}

	if skipPrecheck {
		utils.PrintWarning("⚠ 跳过环境预检查，这可能导致安装失败")
		return nil
	}

	utils.PrintInfo("正在验证集群配置...")
	if err := validateK8sClusterConfig(config); err != nil {
		utils.PrintError("集群配置验证失败: %v", err)
		return fmt.Errorf("集群配置验证失败: %w", err)
	}
	utils.PrintSuccess("✓ 集群配置验证通过")

	utils.PrintInfo("集群节点配置:")
	for i, node := range config.Cluster.Nodes {
		utils.PrintInfo("  节点%d: 主机名=%s, IP=%s, 角色=%s", i+1, node.Host, node.IP, node.Role)
	}

	utils.PrintInfo("正在准备节点...")
	if err := prepareK8sNodes(config); err != nil {
		utils.PrintError("节点准备失败: %v", err)
		return fmt.Errorf("节点准备失败: %w", err)
	}

	return nil
}

// prepareK8sNodes 准备所有Kubernetes节点
func prepareK8sNodes(config *types.ClusterConfig) error {
	var hostsEntries strings.Builder
	for _, node := range config.Cluster.Nodes {
		hostsEntries.WriteString(fmt.Sprintf("%s\t%s\n", node.IP, node.Host))
	}

	for _, node := range config.Cluster.Nodes {
		utils.PrintStage(fmt.Sprintf("准备节点: %s (%s)", node.Host, node.IP))
		startTime := time.Now()

		utils.PrintInfo("正在检查操作系统...")
		if err := checkAndConfigureOS(&node); err != nil {
			utils.PrintError("操作系统配置失败: %v", err)
			return fmt.Errorf("节点%s操作系统配置失败: %w", node.Host, err)
		}

		utils.PrintInfo("正在配置hosts文件...")
		if err := configureHostsFile(&node, hostsEntries.String()); err != nil {
			utils.PrintError("hosts配置失败: %v", err)
			return fmt.Errorf("节点%s hosts配置失败: %w", node.Host, err)
		}

		duration := time.Since(startTime)
		utils.PrintSuccess("✓ 节点%s准备完成，耗时: %v", node.Host, duration.Round(time.Second))
	}

	return nil
}

// checkAndConfigureOS 检查并配置操作系统
func checkAndConfigureOS(node *types.RemoteNode) error {
	utils.PrintInfo("正在检查操作系统类型...")
	osType, err := utils.RunCommandOnNode(node, "uname -s")
	if err != nil {
		return fmt.Errorf("检查OS类型失败: %w", err)
	}
	if !strings.Contains(strings.ToLower(osType), "linux") {
		return fmt.Errorf("不支持的OS类型: %s，仅支持Linux", osType)
	}
	utils.PrintInfo("操作系统类型: %s", strings.TrimSpace(osType))

	utils.PrintInfo("正在检查CPU架构...")
	arch, err := utils.RunCommandOnNode(node, "uname -m")
	if err != nil {
		return fmt.Errorf("检查CPU架构失败: %w", err)
	}
	if !strings.Contains(strings.ToLower(arch), "x86_64") && !strings.Contains(strings.ToLower(arch), "amd64") {
		return fmt.Errorf("不支持的CPU架构: %s，仅支持x86_64/amd64", arch)
	}
	utils.PrintInfo("CPU架构: %s", strings.TrimSpace(arch))

	utils.PrintInfo("正在检查内存大小...")
	memInfo, err := utils.RunCommandOnNode(node, "free -b")
	if err != nil {
		return fmt.Errorf("检查内存大小失败: %w", err)
	}

	lines := strings.Split(memInfo, "\n")
	if len(lines) < 2 {
		return fmt.Errorf("内存信息格式无效")
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 2 {
		return fmt.Errorf("内存信息格式无效")
	}

	totalMem, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return fmt.Errorf("解析内存大小失败: %w", err)
	}

	minMem := int64(2 * 1024 * 1024 * 1024) // 2GB
	if totalMem < minMem {
		return fmt.Errorf("内存不足: %d字节(最低要求: %d字节)", totalMem, minMem)
	}
	utils.PrintInfo("总内存: %.2fGB", float64(totalMem)/float64(1024*1024*1024))

	utils.PrintInfo("正在禁用交换分区...")
	if _, err := utils.RunCommandOnNode(node, " swapoff -a"); err != nil {
		return fmt.Errorf("禁用交换分区失败: %w", err)
	}

	if _, err := utils.RunCommandOnNode(node, " sed -i '/ swap / s/^/#/' /etc/fstab"); err != nil {
		return fmt.Errorf("永久禁用交换分区失败: %w", err)
	}

	return nil
}

// validateK8sClusterConfig 验证 Kubernetes 集群配置
func validateK8sClusterConfig(config *types.ClusterConfig) error {
	if config.Cluster.Name == "" {
		return fmt.Errorf("集群名称不能为空")
	}

	if len(config.Cluster.Nodes) == 0 {
		return fmt.Errorf("集群配置中没有定义节点")
	}

	masterCount := 0
	for _, node := range config.Cluster.Nodes {
		if !utils.IsValidIP(node.IP) {
			return fmt.Errorf("节点%s的IP地址格式无效: %s", node.Host, node.IP)
		}

		if strings.ToLower(node.Role) == "master" {
			masterCount++
		}
	}

	if masterCount == 0 {
		return fmt.Errorf("至少需要一个主节点")
	}

	if config.Cluster.K8sConfig.PodNetworkCidr == "" {
		return fmt.Errorf("Pod网络CIDR不能为空")
	}

	if config.Cluster.K8sConfig.ServiceCidr == "" {
		return fmt.Errorf("服务CIDR不能为空")
	}

	return nil
}

// 其他的master 上执行加入master
func joinMaster(node types.RemoteNode, config *types.ClusterConfig) error {
	//加入master节点
	utils.PrintDebug("node %v, config, %v", node, config)
	return nil
}

// joinWorkerNodes 加入工作节点
func joinWorkerNodes(config *types.ClusterConfig) error {
	joinFile := filepath.Join(utils.GetWorkTmpDir(), config.Cluster.Name+"_k8s-join-command.txt")
	joinCommand, err := utils.ReadFileToString(joinFile)
	if err != nil {
		utils.PrintError("读取加入命令失败: %v", err)
		return fmt.Errorf("读取加入命令失败: %w", err)
	}

	for _, node := range config.Cluster.Nodes {
		if node.Role != "worker" {
			continue
		}

		utils.PrintStage(fmt.Sprintf("正在加入工作节点: %s", node.Host))
		startTime := time.Now()

		output, err := utils.RunCommandOnNode(&node, " "+joinCommand)
		if err != nil {
			utils.PrintError("工作节点加入失败: %v", err)
			return fmt.Errorf("工作节点%s加入失败: %w\n输出: %s", node.Host, err, output)
		}

		duration := time.Since(startTime)
		utils.PrintSuccess("✓ 节点%s加入成功，耗时: %v", node.Host, duration.Round(time.Second))
	}

	return nil
}

// printK8sClusterInfo 打印 Kubernetes 集群信息
func printK8sClusterInfo(config *types.ClusterConfig, masterNode *types.RemoteNode) error {
	utils.PrintInfo("正在获取集群节点信息...")
	output, err := utils.RunCommandOnNode(masterNode, "kubectl get nodes")
	if err != nil {
		return fmt.Errorf("获取集群节点失败: %w", err)
	}

	utils.PrintSuccess("\n✓ Kubernetes集群创建成功!")
	utils.PrintInfo("\n=== 集群信息 ===")
	utils.PrintInfo("集群名称: %s", config.Cluster.Name)
	utils.PrintInfo("主节点: %s (%s)", masterNode.Host, masterNode.IP)
	utils.PrintInfo("\n集群节点:")
	fmt.Println(output)

	utils.PrintInfo("\n=== 验证指南 ===")
	utils.PrintInfo("1. 检查集群状态:")
	utils.PrintInfo("   kubectl get nodes")
	utils.PrintInfo("   kubectl get pods --all-namespaces")

	utils.PrintInfo("\n2. 部署测试应用:")
	utils.PrintInfo("   kubectl create deployment nginx --image=nginx")
	utils.PrintInfo("   kubectl expose deployment nginx --port=80 --type=NodePort")

	infoFile := filepath.Join(utils.GetWorkDir(), "k8s-cluster-info.txt")
	infoContent := fmt.Sprintf("集群名称: %s\n主节点: %s\n\n节点:\n%s",
		config.Cluster.Name, masterNode.Host, output)
	if err := utils.WriteStringToFile(infoFile, infoContent); err != nil {
		utils.PrintWarning("保存集群信息失败: %v", err)
	} else {
		utils.PrintInfo("\n集群信息已保存到: %s", infoFile)
	}

	return nil
}

// extractJoinCommand 从 kubeadm init 输出中提取 join 命令
func extractJoinCommand(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "kubeadm join") {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// findMasterNodes 查找所有主节点，返回主节点列表
func findMasterNodes(config *types.ClusterConfig) []types.RemoteNode {
	masterList := make([]types.RemoteNode, 0, len(config.Cluster.Nodes)/2)
	for i := range config.Cluster.Nodes {
		if strings.ToLower(config.Cluster.Nodes[i].Role) == "master" {
			masterList = append(masterList, config.Cluster.Nodes[i])
		}
	}
	return masterList
}

// findFirstMasterNode 查找第一个主节点，如果没有找到则返回nil
func findFirstMasterNode(config *types.ClusterConfig) *types.RemoteNode {
	masterList := findMasterNodes(config)
	if len(masterList) > 0 {
		return &masterList[0]
	}
	return nil
}

// 移除 Kubernetes 集群
func RemoveK8sCluster(config *types.ClusterConfig, force bool) error {
	startTime := time.Now()
	utils.PrintBanner(fmt.Sprintf("正在移除Kubernetes集群: %s", config.Cluster.Name))
	utils.PrintInfo("开始时间: %s", startTime.Format("2006-01-02 15:04:05"))

	if !force {
		if !utils.AskForConfirmation("确定要移除Kubernetes集群吗？") {
			utils.PrintWarning("操作已取消")
			return fmt.Errorf("操作已取消")
		}
	}

	utils.PrintStage("== 集群移除流程 ==")
	for _, node := range config.Cluster.Nodes {
		utils.PrintStage(fmt.Sprintf("正在重置节点: %s (%s)", node.Host, node.IP))
		startTime := time.Now()

		utils.PrintInfo("正在执行kubeadm reset...")
		if _, err := utils.RunCommandOnNode(&node, " kubeadm reset -f"); err != nil {
			utils.PrintError("节点重置失败: %v", err)
			return fmt.Errorf("节点%s重置失败: %w", node.Host, err)
		}

		utils.PrintInfo("正在清理配置...")
		cleanupCmds := []string{
			" rm -rf /etc/cni/net.d",
			" rm -rf $HOME/.kube",
			" rm -rf /etc/kubernetes",
		}

		for _, cmd := range cleanupCmds {
			if _, err := utils.RunCommandOnNode(&node, cmd); err != nil {
				utils.PrintWarning("清理操作失败: %s: %v", cmd, err)
			}
		}

		duration := time.Since(startTime)
		utils.PrintSuccess("✓ 节点%s重置完成，耗时: %v", node.Host, duration.Round(time.Second))
	}

	duration := time.Since(startTime)
	utils.PrintSuccess("\n✓ Kubernetes集群 '%s' 移除成功!", config.Cluster.Name)
	utils.PrintInfo("总执行时间: %v", duration.Round(time.Second))

	return nil
}
