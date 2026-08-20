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
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/structure-projects/somcli/pkg/utils"
)

// secureJoin 把归档内的条目名解到 dir 之内，越界的名字一律拒绝。
//
// 归档是外部输入（离线镜像包常在机器之间传递），而 somcli 多数场景以 root 运行：
// 一个条目名写成 ../../etc/cron.d/x 就足以让"导入镜像"变成往任意路径写攻击者提供的内容。
// 判据取 filepath.Rel 的结果而不是看名字里有没有 ".."：后者既会漏掉 a/../../b 这类
// 绕过写法，也会误伤名字里恰好含 ".." 的正常文件。
func secureJoin(dir, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("归档条目名为空")
	}
	if filepath.IsAbs(name) || strings.HasPrefix(filepath.ToSlash(name), "/") {
		return "", fmt.Errorf("归档条目名不能是绝对路径: %s", name)
	}
	joined := filepath.Join(dir, name)
	rel, err := filepath.Rel(dir, joined)
	if err != nil {
		return "", fmt.Errorf("归档条目名不合法: %s", name)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("归档条目越出解压目录: %s", name)
	}
	return joined, nil
}

func Import(config Config) error {
	if err := validateScope(config.Scope); err != nil {
		return err
	}

	inputPath := filepath.Join(config.InputFile)
	file, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %v", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %v", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	tempDir, err := newTempDir("import-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	var failed failures
	loaded := 0
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %v", err)
		}

		// 只收常规文件：docker load 要的就是归档里的 .tar。目录、软链接、设备节点
		// 一律跳过 —— 尤其软链接，收下之后后一个条目可以顺着它写到解压目录之外。
		if header.Typeflag != tar.TypeReg {
			continue
		}

		tempFile, err := secureJoin(tempDir, header.Name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(tempFile), 0755); err != nil {
			return fmt.Errorf("failed to create directory for image: %v", err)
		}

		file, err := os.Create(tempFile)
		if err != nil {
			return fmt.Errorf("failed to create temp file: %v", err)
		}

		if _, err := io.Copy(file, tarReader); err != nil {
			file.Close()
			return fmt.Errorf("failed to extract image: %v", err)
		}
		file.Close()

		logrus.Infof("Loading image from: %s", header.Name)

		loaded++
		if err := utils.RunCommand("docker", "load", "-i", tempFile); err != nil {
			failed.add(fmt.Sprintf("load %s", header.Name), err)
			continue
		}
	}

	if err := failed.err("import", loaded); err != nil {
		return err
	}

	logrus.Infof("Images imported from: %s", inputPath)
	return nil
}
