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

func Export(config Config) error {
	if err := validateScope(config.Scope); err != nil {
		return err
	}

	images, err := getImageList(config.Scope, config.CustomFile)
	if err != nil {
		return err
	}

	outputPath := config.OutputFile
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}

	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)

	// 半截归档比没有归档更危险：它看着像个能用的离线包，要等到目标机上 import 才暴露。
	// 所以任何失败都连带删掉产物 —— 与 F14 的"有失败就不写镜像清单"是同一条判据。
	discard := func(cause error) error {
		_ = tarWriter.Close()
		_ = gzipWriter.Close()
		_ = file.Close()
		_ = os.Remove(outputPath)
		return cause
	}

	tempDir, err := newTempDir("export-")
	if err != nil {
		return discard(err)
	}
	defer os.RemoveAll(tempDir)

	var failed failures
	for _, img := range images {
		fullName := formatImageName(img, config.Repo)
		tempFile := filepath.Join(tempDir, sanitizeImageName(fullName)+".tar")

		logrus.Infof("Saving image: %s to %s", fullName, tempFile)

		if err := utils.RunCommand("docker", "save", "-o", tempFile, fullName); err != nil {
			failed.add(fmt.Sprintf("save %s", fullName), err)
			continue
		}

		if err := addFileToTar(tarWriter, tempFile, filepath.Base(tempFile)); err != nil {
			failed.add(fmt.Sprintf("archive %s", fullName), err)
			continue
		}
	}

	if err := failed.err("export", len(images)); err != nil {
		return discard(err)
	}

	// Close 必须逐层检查：真正把剩余数据刷进文件的正是 gzip 的 Close，
	// 用 defer 忽略它的返回值等于把"磁盘满"写成"导出成功"。
	if err := tarWriter.Close(); err != nil {
		return discard(fmt.Errorf("failed to finalize tar stream: %v", err))
	}
	if err := gzipWriter.Close(); err != nil {
		return discard(fmt.Errorf("failed to finalize gzip stream: %v", err))
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(outputPath)
		return fmt.Errorf("failed to close output file: %v", err)
	}

	logrus.Infof("Images exported to: %s", outputPath)
	return nil
}

func addFileToTar(tw *tar.Writer, filePath, tarPath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:    tarPath,
		Size:    stat.Size(),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if _, err := io.Copy(tw, file); err != nil {
		return err
	}

	return nil
}

func sanitizeImageName(name string) string {
	return strings.NewReplacer("/", "_", ":", "_").Replace(name)
}
