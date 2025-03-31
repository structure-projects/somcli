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

	outputPath := filepath.Join(config.OutputFile)
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	tempDir := filepath.Join("temp-export")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	for _, img := range images {
		fullName := formatImageName(img, config.Repo)
		tempFile := filepath.Join(tempDir, sanitizeImageName(fullName)+".tar")

		logrus.Infof("Saving image: %s to %s", fullName, tempFile)

		if err := utils.RunCommand("docker", "save", "-o", tempFile, fullName); err != nil {
			logrus.Warnf("Failed to save image %s: %v", fullName, err)
			continue
		}

		if err := addFileToTar(tarWriter, tempFile, filepath.Base(tempFile)); err != nil {
			logrus.Warnf("Failed to add image %s to archive: %v", fullName, err)
			continue
		}
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
