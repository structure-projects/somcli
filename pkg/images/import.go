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

	"github.com/sirupsen/logrus"
	"github.com/structure-projects/somcli/pkg/utils"
)

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

	tempDir := filepath.Join("temp-import")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
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

		if header.Typeflag == tar.TypeDir {
			continue
		}

		tempFile := filepath.Join(tempDir, header.Name)
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
