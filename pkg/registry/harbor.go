package registry

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"github.com/structure-projects/somcli/pkg/utils"
)

const (
	harborDefaultVersion = "v2.5.0"
	harborAppDir         = "harbor"
	AppNameDir           = "registry"
	harborCaDir          = "certs"
)

type HarborManager struct {
	Version    string
	Hostname   string
	CAPath     string
	InstallDir string
	Config     *viper.Viper
}

func NewHarborManager(version, hostname, caPath string, config *viper.Viper) *HarborManager {
	if version == "" {
		version = harborDefaultVersion
	}
	return &HarborManager{
		Version:    version,
		Hostname:   hostname,
		CAPath:     caPath,
		InstallDir: filepath.Join(utils.GetAppDir(), AppNameDir),
		Config:     config,
	}
}

func (hm *HarborManager) Install() error {
	// 1. 验证环境
	if err := hm.validateEnvironment(); err != nil {
		return err
	}

	// 2. 准备安装目录
	if err := hm.prepareInstallDir(); err != nil {
		return err
	}

	// 3. 下载Harbor安装包
	if err := hm.downloadHarbor(); err != nil {
		return err
	}

	// 4. 配置Harbor
	if err := hm.configureHarbor(); err != nil {
		return err
	}

	// 5. 安装Harbor
	if err := hm.runInstallScript(); err != nil {
		return err
	}

	return nil
}

func (hm *HarborManager) Uninstall() error {
	// 停止Harbor服务
	if err := utils.RunCommand("docker-compose", "-f", filepath.Join(hm.InstallDir, "docker-compose.yml"), "down"); err != nil {
		return fmt.Errorf("failed to stop Harbor: %v", err)
	}

	// 删除安装目录
	if err := os.RemoveAll(hm.InstallDir); err != nil {
		return fmt.Errorf("failed to remove Harbor directory: %v", err)
	}

	return nil
}

func (hm *HarborManager) validateEnvironment() error {
	// 检查Docker是否安装
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker is not installed: %v", err)
	}

	// 检查Docker Compose是否安装
	if _, err := exec.LookPath("docker-compose"); err != nil {
		return fmt.Errorf("docker-compose is not installed: %v", err)
	}

	return nil
}

func (hm *HarborManager) prepareInstallDir() error {
	// 创建安装目录
	if err := os.MkdirAll(hm.InstallDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %v", err)
	}
	return nil
}

func (hm *HarborManager) downloadHarbor() error {
	// 使用统一的下载管理器
	downloader := utils.NewDownloader(hm.Config.GetString("github_proxy"))

	originalURL := fmt.Sprintf("https://github.com/goharbor/harbor/releases/download/%s/harbor-offline-installer-%s.tgz",
		hm.Version, hm.Version)

	tarFile := fmt.Sprintf("harbor-offline-installer-%s.tgz", hm.Version)
	downpath := filepath.Join(utils.GetDownloadDir(), "harbor", hm.Version, tarFile)
	utils.CreateDir(filepath.Join(utils.GetDownloadDir(), "harbor", hm.Version))
	utils.PrintInfo("Downloading Harbor...")
	if err := downloader.Download(originalURL, downpath, hm.InstallDir); err != nil {
		return fmt.Errorf("failed to download Harbor: %v", err)
	}

	// 解压安装包
	if err := utils.RunCommand("tar", "xzvf",
		downpath,
		"-C", hm.InstallDir); err != nil {
		return fmt.Errorf("failed to extract Harbor package: %v", err)
	}

	hm.InstallDir = filepath.Join(hm.InstallDir, harborAppDir)

	return nil
}

func (hm *HarborManager) configureHarbor() error {
	// 准备配置文件
	configFile := filepath.Join(hm.InstallDir, "harbor.yml.tmpl")
	newConfigFile := filepath.Join(hm.InstallDir, "harbor.yml")

	// 读取模板配置
	content, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read Harbor config template: %v", err)
	}

	// 替换配置变量
	configContent := strings.ReplaceAll(string(content), "hostname: reg.mydomain.com",
		fmt.Sprintf("hostname: %s", hm.Hostname))

	// 替换数据盘
	dataDir := filepath.Join(hm.InstallDir, "data")
	utils.CreateDir(dataDir)
	configContent = strings.ReplaceAll(configContent, "data_volume: /data", fmt.Sprintf("data_volume: %s", dataDir))
	// 替换日志盘
	logDir := filepath.Join(hm.InstallDir, "logs")
	utils.CreateDir(logDir)
	configContent = strings.ReplaceAll(configContent, "location: /var/log/harbor", fmt.Sprintf("location: %s", logDir))
	// 如果未提供了CA证书，则生成证书配置TLS
	if hm.CAPath == "" {
		caDir := filepath.Join(hm.InstallDir, harborCaDir)
		generatorCa := utils.NewCertGenerator(hm.Hostname, utils.CertConfig{
			OutputDir: caDir,
		})
		if err := generatorCa.Generate(); err != nil {
			log.Fatalf("生成证书失败: %v", err)
		}
		fmt.Printf("证书已生成到: %s\n", generatorCa.OutputDir)
		hm.CAPath = caDir
	}
	//替换证书逻辑
	// configContent = strings.ReplaceAll(configContent,
	// 	"# https:", "https:")
	configContent = strings.ReplaceAll(configContent,
		"  certificate: /your/certificate/path",
		fmt.Sprintf("  certificate: %s", filepath.Join(hm.CAPath, hm.Hostname+".crt")))
	configContent = strings.ReplaceAll(configContent,
		"  private_key: /your/private/key/path",
		fmt.Sprintf("  private_key: %s", filepath.Join(hm.CAPath, hm.Hostname+".key")))
	//拷贝客户端证书

	// 写入新配置
	if err := os.WriteFile(newConfigFile, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write Harbor config: %v", err)
	}

	return nil
}

func (hm *HarborManager) runInstallScript() error {
	// 运行安装脚本
	cmd := exec.Command(filepath.Join(hm.InstallDir, "install.sh"))
	cmd.Dir = hm.InstallDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Harbor: %v", err)
	}
	return nil
}
