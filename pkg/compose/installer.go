package compose

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/spf13/viper"
	"github.com/structure-projects/somcli/pkg/utils"
)

const (
	defaultInstallPath = "/usr/local/bin/docker-compose"
	// daocloudMirror     = "https://get.daocloud.io/docker/compose/releases/download"
	// https://github.com/docker/compose/releases/download/v2.24.0/docker-compose-linux-x86_64
	githubMirror = "https://github.com/docker/compose/releases/download"
)

type Installer struct {
	silent      bool
	installPath string
	Config      *viper.Viper
}

func NewInstaller(silent bool, config *viper.Viper) *Installer {
	return &Installer{
		silent:      silent,
		installPath: defaultInstallPath,
		Config:      config,
	}
}

func (i *Installer) SetInstallPath(path string) {
	i.installPath = path
}

func (i *Installer) Install(version string) error {
	// 使用统一的下载管理器
	cacheDir := filepath.Join(utils.GetDownloadDir(), "docker-comopose", version)
	downloader := utils.NewDownloader(i.Config.GetString("github_proxy"))

	version = normalizeVersion(version)
	binaryName := i.getBinaryName()
	if binaryName == "" {
		return fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	if i.isInstalled() {
		if !i.silent {
			fmt.Printf("ℹ️ Docker Compose is already installed at %s\n", i.installPath)
		}
		return nil
	}

	downloadURL := fmt.Sprintf("%s/%s/docker-compose-%s",
		githubMirror,
		version,
		binaryName)

	cachedFile := fmt.Sprintf("docker-compose-%s-%s", version, binaryName)

	if !i.silent {
		fmt.Printf("⌛ Downloading from: %s\n", downloadURL)
	}

	if err := downloader.Download(downloadURL, cachedFile, cacheDir); err != nil {
		return fmt.Errorf("download failed: %v", err)
	}

	srcFile := filepath.Join(cacheDir, cachedFile)

	if err := utils.CopyFile(srcFile, i.installPath); err != nil {
		return fmt.Errorf("failed to install docker-compose: %v", err)
	}

	if err := os.Chmod(i.installPath, 0755); err != nil {
		return fmt.Errorf("failed to set executable permissions: %v", err)
	}

	if !i.silent {
		fmt.Printf("✅ Docker Compose %s installed successfully to %s\n", version, i.installPath)
	}
	return nil
}

func (i *Installer) Uninstall() error {
	if !i.isInstalled() {
		if !i.silent {
			fmt.Println("ℹ️ Docker Compose is not installed")
		}
		return nil
	}

	if err := os.Remove(i.installPath); err != nil {
		return fmt.Errorf("failed to uninstall docker-compose: %v", err)
	}

	if !i.silent {
		fmt.Println("✅ Docker Compose uninstalled successfully")
	}
	return nil
}

func (i *Installer) Version() (string, error) {
	if !i.isInstalled() {
		return "", fmt.Errorf("docker compose is not installed")
	}

	cmd := exec.Command(i.installPath, "version", "--short")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get version: %v", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func (i *Installer) Passthrough(args []string) error {
	if !i.isInstalled() {
		if !i.silent {
			fmt.Println("⚡ Docker Compose not found, attempting to install...")
		}
		if err := i.Install("v2.24.0"); err != nil {
			return fmt.Errorf("auto-install failed: %v\nPlease install manually first", err)
		}
	}

	args = i.processArgs(args)
	execPath := i.installPath
	if runtime.GOOS == "windows" && !strings.HasSuffix(execPath, ".exe") {
		execPath += ".exe"
	}

	cmd := exec.Command(execPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()

	i.handleSignals(cmd)

	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			os.Exit(status.ExitStatus())
		}
		return exitErr
	}
	return err
}

func (i *Installer) processArgs(args []string) []string {
	if len(args) > 0 {
		switch args[0] {
		case "up", "down", "restart":
			if !contains(args, "-d") && !contains(args, "--detach") {
				args = append(args, "-d")
			}
		case "ps":
			if !contains(args, "-a") && !contains(args, "--all") {
				args = append(args, "-a")
			}
		}
	}
	return args
}

func (i *Installer) handleSignals(cmd *exec.Cmd) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		if cmd.Process != nil {
			cmd.Process.Signal(sig)
		}
	}()
}

func (i *Installer) isInstalled() bool {
	_, err := os.Stat(i.installPath)
	return !os.IsNotExist(err)
}

func (i *Installer) getBinaryName() string {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	switch osName {
	case "darwin":
		if arch == "arm64" {
			return "Darwin-aarch64"
		}
		return "Darwin-x86_64"
	case "linux":
		if arch == "arm64" {
			return "Linux-aarch64"
		}
		return "Linux-x86_64"
	case "windows":
		return "Windows-x86_64.exe"
	default:
		return ""
	}
}

func normalizeVersion(version string) string {
	if !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
