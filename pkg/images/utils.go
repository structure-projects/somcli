package images

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/structure-projects/somcli/pkg/utils"
	"gopkg.in/yaml.v2"
)

func getImageList(scope, customFile string) ([]Image, error) {
	if customFile != "" {
		return loadCustomImageList(customFile)
	}

	switch scope {
	case ScopeHarbor:
		return getDefaultHarborImages()
	case ScopeK8s:
		return getDefaultK8sImages()
	case ScopeAll:
		harborImages, _ := getDefaultHarborImages()
		k8sImages, _ := getDefaultK8sImages()
		return append(harborImages, k8sImages...), nil
	default:
		return nil, fmt.Errorf("invalid scope: %s", scope)
	}
}

func getDefaultHarborImages() ([]Image, error) {
	if defaultFile, err := getDefaultImageFile("harbor-images.yaml"); err == nil {
		if images, err := loadCustomImageList(defaultFile); err == nil {
			return images, nil
		}
	}

	return []Image{
		{"library/nginx", "latest"},
		// {"library/redis", "alpine"},
		// {"library/postgres", "13"},
		// {"library/mysql", "8.0"},
	}, nil
}

func getDefaultK8sImages() ([]Image, error) {
	if defaultFile, err := getDefaultImageFile("k8s-images.yaml"); err == nil {
		if images, err := loadCustomImageList(defaultFile); err == nil {
			return images, nil
		}
	}

	return []Image{
		{"k8s.gcr.io/pause", "3.7"},
		// {"k8s.gcr.io/kube-apiserver", "v1.25.0"},
		// {"k8s.gcr.io/kube-controller-manager", "v1.25.0"},
		// {"k8s.gcr.io/kube-scheduler", "v1.25.0"},
		// {"k8s.gcr.io/kube-proxy", "v1.25.0"},
		// {"k8s.gcr.io/coredns/coredns", "v1.9.3"},
	}, nil
}

func getDefaultImageFile(filename string) (string, error) {
	// Look in current directory
	if _, err := os.Stat(filename); err == nil {
		return filename, nil
	}

	// Look in config directory
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "somcli")
	if _, err := os.Stat(filepath.Join(configDir, filename)); err == nil {
		return filepath.Join(configDir, filename), nil
	}

	// Look in executable directory
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(exeDir, filename)); err == nil {
			return filepath.Join(exeDir, filename), nil
		}
	}

	return "", fmt.Errorf("default image file not found: %s", filename)
}

func loadCustomImageList(filePath string) ([]Image, error) {
	utils.PrintWarning(filePath)
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image list file: %v", err)
	}

	var images []Image
	if err := yaml.Unmarshal(data, &images); err != nil {
		// If YAML unmarshal fails, try to parse as plain text
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.Split(line, ":")
			if len(parts) != 2 {
				logrus.Warnf("Invalid image format: %s", line)
				continue
			}
			images = append(images, Image{
				Name: strings.TrimSpace(parts[0]),
				Tag:  strings.TrimSpace(parts[1]),
			})
		}
	}

	return images, nil
}

func saveImageList(images []Image, filePath string) error {
	data, err := yaml.Marshal(images)
	if err != nil {
		return fmt.Errorf("failed to marshal image list: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write image list file: %v", err)
	}

	return nil
}

func formatImageName(img Image, repo string) string {
	if repo == "" {
		return fmt.Sprintf("%s:%s", img.Name, img.Tag)
	}
	return fmt.Sprintf("%s/%s:%s", repo, img.Name, img.Tag)
}
