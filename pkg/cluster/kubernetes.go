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
	"path/filepath"
	"strconv"
	"strings"

	"github.com/structure-projects/somcli/pkg/utils"
)

const (
	k8sBaseURL           = "https://storage.googleapis.com/kubernetes-release/release"
	k8sCacheDir          = "k8s"
	k8sComponents        = "kubeadm kubelet kubectl"
	containerdReleaseURL = "https://github.com/containerd/containerd/releases/download/v%s/containerd-%s-linux-amd64.tar.gz"
	containerdServiceURL = "https://raw.githubusercontent.com/containerd/containerd/main/containerd.service"
	runcURL              = "https://github.com/opencontainers/runc/releases/download/%s/runc.amd64"
	cniPluginsURL        = "https://github.com/containernetworking/plugins/releases/download/%s/cni-plugins-linux-amd64-%s.tgz"
)

// CreateK8sCluster 创建 Kubernetes 集群（支持离线/在线混合模式）
func CreateK8sCluster(config *ClusterConfig, force bool, skipPrecheck bool) error {
	utils.PrintBanner("Creating Kubernetes Cluster (Hybrid Mode): " + config.Cluster.Name)

	// 1. 准备工作
	if err := prepareK8sCluster(config, skipPrecheck); err != nil {
		return fmt.Errorf("preparation failed: %w", err)
	}

	// 2. 在所有节点上安装依赖
	if err := installK8sDependencies(config); err != nil {
		return fmt.Errorf("dependency installation failed: %w", err)
	}

	// 3. 在主节点上初始化集群
	masterNode := findMasterNode(config)
	if masterNode == nil {
		return fmt.Errorf("no master node found in configuration")
	}

	if err := initK8sMaster(masterNode, config); err != nil {
		return fmt.Errorf("master initialization failed: %w", err)
	}

	// 4. 加入工作节点
	if err := joinWorkerNodes(config, masterNode); err != nil {
		return fmt.Errorf("worker node join failed: %w", err)
	}

	// 5. 输出集群信息
	if err := printK8sClusterInfo(config, masterNode); err != nil {
		return fmt.Errorf("failed to print cluster info: %w", err)
	}

	return nil
}

func prepareK8sCluster(config *ClusterConfig, skipPrecheck bool) error {
	if err := EnsureWorkDir(); err != nil {
		return fmt.Errorf("failed to ensure work directory: %w", err)
	}

	if skipPrecheck {
		utils.PrintWarning("Skipping environment prechecks. This may cause installation failures.")
		return nil
	}

	if err := validateK8sClusterConfig(config); err != nil {
		return fmt.Errorf("invalid cluster configuration: %w", err)
	}

	utils.PrintInfo("Cluster nodes configuration:")
	for _, node := range config.Cluster.Nodes {
		utils.PrintInfo("Node: %s, IP: %s, Role: %s", node.Host, node.IP, node.Role)
	}

	if err := prepareK8sNodes(config); err != nil {
		return fmt.Errorf("node preparation failed: %w", err)
	}

	if err := prepareK8sComponents(config.Cluster.K8sConfig.Version); err != nil {
		return fmt.Errorf("failed to prepare Kubernetes components: %w", err)
	}

	return nil
}

func prepareK8sComponents(version string) error {
	cacheDir := filepath.Join(utils.GetWorkDir(), k8sCacheDir, version)
	if err := utils.CreateDir(cacheDir); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	downloader := utils.NewDownloader("")

	for _, component := range strings.Split(k8sComponents, " ") {
		localFile := filepath.Join(cacheDir, component)

		if utils.FileExists(localFile) && utils.IsExecutable(localFile) {
			utils.PrintInfo("Kubernetes component %s already exists in cache", component)
			continue
		}

		url := fmt.Sprintf("%s/v%s/bin/linux/amd64/%s", k8sBaseURL, version, component)
		utils.PrintInfo("Downloading %s from %s...", component, url)

		if err := downloader.Download(url, component, cacheDir); err != nil {
			return fmt.Errorf("failed to download %s: %w", component, err)
		}

		if err := utils.RunCommand("chmod", "+x", localFile); err != nil {
			return fmt.Errorf("failed to make %s executable: %w", component, err)
		}
	}

	utils.PrintSuccess("All Kubernetes components prepared successfully")
	return nil
}

func prepareContainerdPackages(config *K8sConfig) error {
	if config.ContainerdVersion == "" {
		return nil
	}

	localCacheDir := filepath.Join(utils.GetWorkDir(), "offline-packages", "containerd", config.ContainerdVersion)
	if err := utils.CreateDir(localCacheDir); err != nil {
		return fmt.Errorf("failed to create containerd cache directory: %w", err)
	}

	// 准备 containerd 二进制包
	pkgName := fmt.Sprintf("containerd-%s-linux-amd64.tar.gz", config.ContainerdVersion)
	localPkgPath := filepath.Join(localCacheDir, pkgName)

	if !utils.FileExists(localPkgPath) {
		utils.PrintInfo("Containerd package not found in cache, downloading...")
		downloader := utils.NewDownloader("")
		pkgURL := fmt.Sprintf(containerdReleaseURL, config.ContainerdVersion, config.ContainerdVersion)

		if err := downloader.Download(pkgURL, pkgName, localCacheDir); err != nil {
			return fmt.Errorf("failed to download containerd package: %w", err)
		}
	}

	// 准备 containerd.service 文件
	serviceFile := filepath.Join(localCacheDir, "containerd.service")
	if !utils.FileExists(serviceFile) {
		utils.PrintInfo("Containerd service file not found in cache, downloading...")
		downloader := utils.NewDownloader("")

		if err := downloader.Download(containerdServiceURL, "containerd.service", localCacheDir); err != nil {
			return fmt.Errorf("failed to download containerd.service: %w", err)
		}
	}

	// 准备 runc
	if config.RuncVersion != "" {
		runcFile := filepath.Join(localCacheDir, "runc")
		if !utils.FileExists(runcFile) {
			utils.PrintInfo("Runc binary not found in cache, downloading...")
			downloader := utils.NewDownloader("")
			runcURL := fmt.Sprintf(runcURL, config.RuncVersion)

			if err := downloader.Download(runcURL, "runc", localCacheDir); err != nil {
				return fmt.Errorf("failed to download runc: %w", err)
			}

			if err := utils.RunCommand("chmod", "+x", runcFile); err != nil {
				return fmt.Errorf("failed to make runc executable: %w", err)
			}
		}
	}

	// 准备 CNI 插件
	if config.CniVersion != "" {
		cniPkg := fmt.Sprintf("cni-plugins-linux-amd64-%s.tgz", config.CniVersion)
		cniPath := filepath.Join(localCacheDir, cniPkg)

		if !utils.FileExists(cniPath) {
			utils.PrintInfo("CNI plugins not found in cache, downloading...")
			downloader := utils.NewDownloader("")
			cniURL := fmt.Sprintf(cniPluginsURL, config.CniVersion, config.CniVersion)

			if err := downloader.Download(cniURL, cniPkg, localCacheDir); err != nil {
				return fmt.Errorf("failed to download CNI plugins: %w", err)
			}
		}
	}

	utils.PrintSuccess("All container runtime packages prepared successfully")
	return nil
}

func installContainerdHybrid(node *Node, config *ClusterConfig) error {
	if config.Cluster.K8sConfig.ContainerdVersion == "" {
		return nil
	}

	if err := prepareContainerdPackages(&config.Cluster.K8sConfig); err != nil {
		return fmt.Errorf("failed to prepare containerd packages: %w", err)
	}

	localCacheDir := filepath.Join(utils.GetWorkDir(), "offline-packages", "containerd", config.Cluster.K8sConfig.ContainerdVersion)
	remoteTmpDir := "/tmp/containerd-install"

	if _, err := RunCommandOnNode(node, fmt.Sprintf(
		"sudo mkdir -p %s && sudo chmod 777 %s", remoteTmpDir, remoteTmpDir)); err != nil {
		return fmt.Errorf("failed to create temp directory on node: %w", err)
	}

	// 上传必要文件
	filesToUpload := []string{
		fmt.Sprintf("containerd-%s-linux-amd64.tar.gz", config.Cluster.K8sConfig.ContainerdVersion),
		"containerd.service",
	}

	if config.Cluster.K8sConfig.RuncVersion != "" {
		filesToUpload = append(filesToUpload, "runc")
	}

	if config.Cluster.K8sConfig.CniVersion != "" {
		filesToUpload = append(filesToUpload,
			fmt.Sprintf("cni-plugins-linux-amd64-%s.tgz", config.Cluster.K8sConfig.CniVersion))
	}

	for _, file := range filesToUpload {
		localPath := filepath.Join(localCacheDir, file)
		remotePath := filepath.Join(remoteTmpDir, file)

		if node.Host == "localhost" || node.Host == "127.0.0.1" {
			if err := utils.RunCommand("sudo", "cp", localPath, remotePath); err != nil {
				return fmt.Errorf("failed to copy %s to node: %w", file, err)
			}
		} else {
			content, err := os.ReadFile(localPath)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", file, err)
			}

			sshKey := utils.ExpandPath(node.SSHKey)
			if err := utils.SSHCopy(node.User, node.Host, sshKey,
				strings.NewReader(string(content)), remotePath); err != nil {
				return fmt.Errorf("failed to upload %s to node: %w", file, err)
			}
		}
	}

	// 构建安装脚本
	installScript := buildContainerdInstallScript(config.Cluster.K8sConfig, remoteTmpDir)

	// 上传并执行安装脚本
	scriptPath := filepath.Join(remoteTmpDir, "install-containerd.sh")
	if node.Host == "localhost" || node.Host == "127.0.0.1" {
		if err := utils.WriteStringToFile("/tmp/install-containerd.sh", installScript); err != nil {
			return fmt.Errorf("failed to create install script: %w", err)
		}
		if err := utils.RunCommand("sudo", "cp", "/tmp/install-containerd.sh", scriptPath); err != nil {
			return fmt.Errorf("failed to copy install script: %w", err)
		}
	} else {
		sshKey := utils.ExpandPath(node.SSHKey)
		if err := utils.SSHCopy(node.User, node.Host, sshKey,
			strings.NewReader(installScript), scriptPath); err != nil {
			return fmt.Errorf("failed to upload install script: %w", err)
		}
	}

	if _, err := RunCommandOnNode(node, fmt.Sprintf(
		"sudo bash %s", scriptPath)); err != nil {
		return fmt.Errorf("failed to execute install script: %w", err)
	}

	if _, err := RunCommandOnNode(node, "sudo containerd --version"); err != nil {
		return fmt.Errorf("containerd installation verification failed: %w", err)
	}

	utils.PrintSuccess("Containerd installed successfully on node %s", node.Host)
	return nil
}

func buildContainerdInstallScript(config K8sConfig, tmpDir string) string {
	script := `#!/bin/bash
set -e

# 安装 containerd
sudo tar Cxzvf /usr/local %s/containerd-%s-linux-amd64.tar.gz

# 安装 systemd 服务
sudo mkdir -p /usr/local/lib/systemd/system
sudo cp %s/containerd.service /usr/local/lib/systemd/system/

# 安装 runc
if [ -f "%s/runc" ]; then
    sudo install -m 755 %s/runc /usr/local/sbin/runc
fi

# 安装 CNI 插件
if [ -f "%s/cni-plugins-linux-amd64-%s.tgz" ]; then
    sudo mkdir -p /opt/cni/bin
    sudo tar -xzf %s/cni-plugins-linux-amd64-%s.tgz -C /opt/cni/bin
fi

# 配置 containerd
sudo mkdir -p /etc/containerd
containerd config default | sudo tee /etc/containerd/config.toml >/dev/null

# 启动服务
sudo systemctl daemon-reload
sudo systemctl enable --now containerd

# 清理临时文件
sudo rm -rf %s
`

	return fmt.Sprintf(script,
		tmpDir, config.ContainerdVersion,
		tmpDir,
		tmpDir, tmpDir,
		tmpDir, config.CniVersion,
		tmpDir, config.CniVersion,
		tmpDir)
}

func installK8sDependencies(config *ClusterConfig) error {
	for _, node := range config.Cluster.Nodes {
		utils.PrintInfo("Installing dependencies on node %s...", node.Host)

		// 安装 Kubernetes 组件
		cacheDir := filepath.Join(utils.GetWorkDir(), k8sCacheDir, config.Cluster.K8sConfig.Version)
		for _, component := range strings.Split(k8sComponents, " ") {
			localFile := filepath.Join(cacheDir, component)
			remoteFile := filepath.Join("/usr/local/bin", component)

			if node.Host == "localhost" || node.Host == "127.0.0.1" {
				if err := utils.RunCommand("sudo", "cp", localFile, remoteFile); err != nil {
					return fmt.Errorf("failed to copy %s to %s: %w", component, remoteFile, err)
				}
			} else {
				content, err := os.ReadFile(localFile)
				if err != nil {
					return fmt.Errorf("failed to read %s: %w", localFile, err)
				}

				sshKey := utils.ExpandPath(node.SSHKey)
				if err := utils.SSHCopy(node.User, node.Host, sshKey,
					strings.NewReader(string(content)), remoteFile); err != nil {
					return fmt.Errorf("failed to upload %s to node %s: %w", component, node.Host, err)
				}
			}

			if _, err := RunCommandOnNode(&node, fmt.Sprintf("sudo chmod +x %s", remoteFile)); err != nil {
				return fmt.Errorf("failed to make %s executable on node %s: %w", component, node.Host, err)
			}
		}

		// 安装 containerd
		if err := installContainerdHybrid(&node, config); err != nil {
			return fmt.Errorf("failed to install containerd: %w", err)
		}

		// 配置系统参数
		commands := []string{
			"sudo swapoff -a",
			"sudo sed -i '/ swap / s/^/#/' /etc/fstab",
			"sudo modprobe overlay",
			"sudo modprobe br_netfilter",
			"sudo sysctl --system",
		}

		for _, cmd := range commands {
			if _, err := RunCommandOnNode(&node, cmd); err != nil {
				return fmt.Errorf("failed to configure system on node %s: %w", node.Host, err)
			}
		}

		utils.PrintSuccess("All dependencies installed successfully on node %s", node.Host)
	}

	return nil
}

func initK8sMaster(node *Node, config *ClusterConfig) error {
	utils.PrintInfo("Initializing Kubernetes master on %s...", node.Host)

	initCmd := fmt.Sprintf(
		"sudo kubeadm init --pod-network-cidr=%s --service-cidr=%s",
		config.Cluster.K8sConfig.PodNetworkCidr,
		config.Cluster.K8sConfig.ServiceCidr,
	)

	output, err := RunCommandOnNode(node, initCmd)
	if err != nil {
		return fmt.Errorf("failed to initialize Kubernetes master: %w\nOutput: %s", err, output)
	}

	joinCommand := extractJoinCommand(output)
	if joinCommand == "" {
		return fmt.Errorf("failed to extract join command from kubeadm init output")
	}

	joinFile := filepath.Join(utils.GetWorkDir(), "k8s-join-command.txt")
	if err := utils.WriteStringToFile(joinFile, joinCommand); err != nil {
		return fmt.Errorf("failed to save join command: %w", err)
	}

	cmds := []string{
		"mkdir -p $HOME/.kube",
		"sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config",
		"sudo chown $(id -u):$(id -g) $HOME/.kube/config",
	}

	for _, cmd := range cmds {
		if _, err := RunCommandOnNode(node, cmd); err != nil {
			return fmt.Errorf("failed to configure kubectl: %w", err)
		}
	}

	utils.PrintSuccess("Kubernetes master initialized successfully")
	return nil
}

func joinWorkerNodes(config *ClusterConfig, masterNode *Node) error {
	joinFile := filepath.Join(utils.GetWorkDir(), "k8s-join-command.txt")
	joinCommand, err := utils.ReadFileToString(joinFile)
	if err != nil {
		return fmt.Errorf("failed to read join command: %w", err)
	}

	for _, node := range config.Cluster.Nodes {
		if node.Role != "worker" {
			continue
		}

		utils.PrintInfo("Joining worker node %s...", node.Host)
		output, err := RunCommandOnNode(&node, "sudo "+joinCommand)
		if err != nil {
			return fmt.Errorf("failed to join worker node %s: %w\nOutput: %s", node.Host, err, output)
		}

		utils.PrintSuccess("Node %s joined successfully", node.Host)
	}

	return nil
}

func printK8sClusterInfo(config *ClusterConfig, masterNode *Node) error {
	output, err := RunCommandOnNode(masterNode, "kubectl get nodes")
	if err != nil {
		return fmt.Errorf("failed to get cluster nodes: %w", err)
	}

	utils.PrintSuccess("\nKubernetes Cluster created successfully!")
	utils.PrintInfo("\n=== Cluster Information ===")
	utils.PrintInfo("Cluster Name: %s", config.Cluster.Name)
	utils.PrintInfo("Master Node: %s (%s)", masterNode.Host, masterNode.IP)
	utils.PrintInfo("\nCluster Nodes:")
	fmt.Println(output)

	utils.PrintInfo("\n=== Verification Guide ===")
	utils.PrintInfo("1. Check cluster status:")
	utils.PrintInfo("   kubectl get nodes")
	utils.PrintInfo("   kubectl get pods --all-namespaces")

	utils.PrintInfo("\n2. Deploy test application:")
	utils.PrintInfo("   kubectl create deployment nginx --image=nginx")
	utils.PrintInfo("   kubectl expose deployment nginx --port=80 --type=NodePort")

	infoFile := filepath.Join(utils.GetWorkDir(), "k8s-cluster-info.txt")
	infoContent := fmt.Sprintf("Cluster Name: %s\nMaster Node: %s\n\nNodes:\n%s",
		config.Cluster.Name, masterNode.Host, output)
	if err := utils.WriteStringToFile(infoFile, infoContent); err != nil {
		utils.PrintWarning("Failed to save cluster info: %v", err)
	} else {
		utils.PrintInfo("\nCluster information saved to: %s", infoFile)
	}

	return nil
}

func extractJoinCommand(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "kubeadm join") {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func findMasterNode(config *ClusterConfig) *Node {
	for i := range config.Cluster.Nodes {
		if strings.ToLower(config.Cluster.Nodes[i].Role) == "master" {
			return &config.Cluster.Nodes[i]
		}
	}
	return nil
}

func validateK8sClusterConfig(config *ClusterConfig) error {
	if config.Cluster.Name == "" {
		return fmt.Errorf("cluster name cannot be empty")
	}

	if len(config.Cluster.Nodes) == 0 {
		return fmt.Errorf("no nodes defined in cluster configuration")
	}

	masterCount := 0
	for _, node := range config.Cluster.Nodes {
		if !isValidIP(node.IP) {
			return fmt.Errorf("invalid IP address format for node %s: %s", node.Host, node.IP)
		}

		if strings.ToLower(node.Role) == "master" {
			masterCount++
		}
	}

	if masterCount == 0 {
		return fmt.Errorf("at least one master node is required")
	}

	if config.Cluster.K8sConfig.PodNetworkCidr == "" {
		return fmt.Errorf("pod network CIDR cannot be empty")
	}

	if config.Cluster.K8sConfig.ServiceCidr == "" {
		return fmt.Errorf("service CIDR cannot be empty")
	}

	return nil
}

func checkAndConfigureOS(node *Node) error {
	utils.PrintInfo("Checking and configuring OS on node %s...", node.Host)

	osType, err := RunCommandOnNode(node, "uname -s")
	if err != nil {
		return fmt.Errorf("failed to check OS type: %w", err)
	}
	if !strings.Contains(strings.ToLower(osType), "linux") {
		return fmt.Errorf("unsupported OS type: %s, only Linux is supported", osType)
	}

	arch, err := RunCommandOnNode(node, "uname -m")
	if err != nil {
		return fmt.Errorf("failed to check CPU architecture: %w", err)
	}
	if !strings.Contains(strings.ToLower(arch), "x86_64") && !strings.Contains(strings.ToLower(arch), "amd64") {
		return fmt.Errorf("unsupported CPU architecture: %s, only x86_64/amd64 is supported", arch)
	}

	memInfo, err := RunCommandOnNode(node, "free -b")
	if err != nil {
		return fmt.Errorf("failed to check memory size: %w", err)
	}

	lines := strings.Split(memInfo, "\n")
	if len(lines) < 2 {
		return fmt.Errorf("invalid memory info format")
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 2 {
		return fmt.Errorf("invalid memory info format")
	}

	totalMem, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse memory size: %w", err)
	}

	minMem := int64(2 * 1024 * 1024 * 1024)
	if totalMem < minMem {
		return fmt.Errorf("insufficient memory: %d bytes (minimum required: %d bytes)", totalMem, minMem)
	}

	if _, err := RunCommandOnNode(node, "sudo swapoff -a"); err != nil {
		return fmt.Errorf("failed to disable swap: %w", err)
	}

	if _, err := RunCommandOnNode(node, "sudo sed -i '/ swap / s/^/#/' /etc/fstab"); err != nil {
		return fmt.Errorf("failed to disable swap in fstab: %w", err)
	}

	return nil
}

func checkAndConfigureKernelOnNode(node *Node) error {
	utils.PrintInfo("Configuring kernel parameters on node %s...", node.Host)

	requiredModules := []string{"br_netfilter", "overlay"}
	for _, module := range requiredModules {
		if _, err := RunCommandOnNode(node, fmt.Sprintf("sudo modprobe %s", module)); err != nil {
			return fmt.Errorf("failed to load kernel module %s: %w", module, err)
		}

		moduleFile := fmt.Sprintf("/etc/modules-load.d/%s.conf", module)
		loadCmd := fmt.Sprintf("echo %s | sudo tee %s", module, moduleFile)
		if _, err := RunCommandOnNode(node, loadCmd); err != nil {
			return fmt.Errorf("failed to create module load file %s: %w", moduleFile, err)
		}
	}

	requiredParams := map[string]string{
		"net.bridge.bridge-nf-call-iptables":  "1",
		"net.bridge.bridge-nf-call-ip6tables": "1",
		"net.ipv4.ip_forward":                 "1",
	}

	sysctlFile := "/etc/sysctl.d/99-kubernetes-cri.conf"
	var content strings.Builder
	for param, value := range requiredParams {
		content.WriteString(fmt.Sprintf("%s = %s\n", param, value))
	}

	tempFile := "/tmp/99-kubernetes-cri.conf"
	if err := utils.WriteStringToFile(tempFile, content.String()); err != nil {
		return fmt.Errorf("failed to create temp sysctl config: %w", err)
	}

	if node.Host == "localhost" || node.Host == "127.0.0.1" {
		if err := utils.RunCommand("sudo", "cp", tempFile, sysctlFile); err != nil {
			return fmt.Errorf("failed to copy sysctl config: %w", err)
		}
	} else {
		configContent, err := os.ReadFile(tempFile)
		if err != nil {
			return fmt.Errorf("failed to read temp file: %w", err)
		}

		sshKey := utils.ExpandPath(node.SSHKey)
		if err := utils.SSHCopy(node.User, node.Host, sshKey,
			strings.NewReader(string(configContent)), sysctlFile); err != nil {
			return fmt.Errorf("failed to upload sysctl config: %w", err)
		}
	}

	if _, err := RunCommandOnNode(node, "sudo sysctl --system"); err != nil {
		return fmt.Errorf("failed to apply sysctl settings: %w", err)
	}

	return nil
}

func prepareK8sNodes(config *ClusterConfig) error {
	var hostsEntries strings.Builder
	for _, node := range config.Cluster.Nodes {
		hostsEntries.WriteString(fmt.Sprintf("%s\t%s\n", node.IP, node.Host))
	}

	for _, node := range config.Cluster.Nodes {
		utils.PrintInfo("\nPreparing node: %s (%s)", node.Host, node.IP)

		if err := checkAndConfigureOS(&node); err != nil {
			return fmt.Errorf("OS configuration failed for node %s: %w", node.Host, err)
		}

		if err := checkAndConfigureKernelOnNode(&node); err != nil {
			return fmt.Errorf("kernel configuration failed for node %s: %w", node.Host, err)
		}

		if err := configureHostsFile(&node, hostsEntries.String()); err != nil {
			return fmt.Errorf("failed to configure hosts file on node %s: %w", node.Host, err)
		}

		if err := checkNetworkConnectivity(&node, config); err != nil {
			return fmt.Errorf("network connectivity check failed for node %s: %w", node.Host, err)
		}

		if err := checkPortAvailabilityOnNode(&node); err != nil {
			return fmt.Errorf("port availability check failed for node %s: %w", node.Host, err)
		}

		utils.PrintSuccess("Node %s prepared successfully", node.Host)
	}

	return nil
}

func checkNetworkConnectivity(node *Node, config *ClusterConfig) error {
	utils.PrintInfo("Checking network connectivity for node %s...", node.Host)

	for _, peer := range config.Cluster.Nodes {
		if peer.Host == node.Host {
			continue
		}

		checkCmd := fmt.Sprintf("ping -c 1 -W 1 %s", peer.IP)
		if output, err := RunCommandOnNode(node, checkCmd); err != nil {
			return fmt.Errorf("node %s cannot reach %s (%s)\nOutput: %s",
				node.Host, peer.Host, peer.IP, output)
		}
	}

	return nil
}

func checkPortAvailabilityOnNode(node *Node) error {
	utils.PrintInfo("Checking port availability on node %s...", node.Host)

	requiredPorts := []int{
		6443, 2379, 2380, 10250, 10259, 10257,
	}

	for _, port := range requiredPorts {
		checkCmd := fmt.Sprintf("sudo netstat -tuln | grep ':%d ' || true", port)
		output, err := RunCommandOnNode(node, checkCmd)
		if err != nil {
			return fmt.Errorf("failed to check port %d: %w", port, err)
		}

		if strings.TrimSpace(output) != "" {
			return fmt.Errorf("required port %d is already in use: %s", port, output)
		}
	}

	return nil
}

func RemoveK8sCluster(config *ClusterConfig, force bool) error {
	utils.PrintBanner("Removing Kubernetes Cluster: " + config.Cluster.Name)

	if !force {
		if !utils.AskForConfirmation("Are you sure you want to remove the Kubernetes cluster?") {
			return fmt.Errorf("cluster removal cancelled")
		}
	}

	for _, node := range config.Cluster.Nodes {
		utils.PrintInfo("Resetting node %s...", node.Host)

		if _, err := RunCommandOnNode(&node, "sudo kubeadm reset -f"); err != nil {
			return fmt.Errorf("failed to reset node %s: %w", node.Host, err)
		}

		cleanupCmds := []string{
			"sudo rm -rf /etc/cni/net.d",
			"sudo rm -rf $HOME/.kube",
			"sudo rm -rf /etc/kubernetes",
		}

		for _, cmd := range cleanupCmds {
			if _, err := RunCommandOnNode(&node, cmd); err != nil {
				utils.PrintWarning("Failed to cleanup on node %s: %v", node.Host, err)
			}
		}

		utils.PrintSuccess("Node %s reset successfully", node.Host)
	}

	return nil
}
