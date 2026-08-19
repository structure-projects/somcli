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
package images

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/structure-projects/somcli/pkg/types"
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
		{Name: "library/nginx", Tag: "latest"},
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
		{Name: "k8s.gcr.io/pause", Tag: "3.7"},
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

// loadCustomImageList 认三种写法，按"信息量从多到少"依次尝试：
//  1. 统一配置：整份 somcli 配置，取它的 images: 段
//  2. 裸列表：- name: nginx / tag: latest
//  3. 纯文本：每行一个 name:tag（README 里的 image-list.txt 就是这种）
func loadCustomImageList(filePath string) ([]Image, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read image list file: %v", err)
	}

	var config types.ResourceConfig
	if err := yaml.UnmarshalStrict(data, &config); err == nil {
		if len(config.Images) == 0 {
			return nil, fmt.Errorf("%s 是一份 somcli 配置，但里面没有 images: 段", filePath)
		}
		return config.Images, nil
	}

	var images []Image
	if err := yaml.UnmarshalStrict(data, &images); err == nil {
		return images, nil
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, tag, ok := splitNameTag(line)
		if !ok {
			logrus.Warnf("Invalid image format: %s", line)
			continue
		}
		images = append(images, Image{Name: name, Tag: tag})
	}
	if len(images) == 0 {
		return nil, fmt.Errorf("%s 里没有可用的镜像条目", filePath)
	}

	return images, nil
}

// splitNameTag 从 name:tag 一行里切出镜像名与 tag。
//
// 必须切最后一个冒号：registry:5000/nginx:latest 里第一个冒号是仓库端口，
// 切在那里会得到 name=registry、tag=5000/nginx:latest，然后拿这个错名字去 pull。
// 最后一段还含 / 的说明那个冒号也是端口，此时整行没有 tag。
func splitNameTag(line string) (name, tag string, ok bool) {
	idx := strings.LastIndex(line, ":")
	if idx < 0 {
		return "", "", false
	}
	name = strings.TrimSpace(line[:idx])
	tag = strings.TrimSpace(line[idx+1:])
	if name == "" || tag == "" || strings.Contains(tag, "/") {
		return "", "", false
	}
	return name, tag, true
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
