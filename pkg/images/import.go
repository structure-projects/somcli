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

		if err := utils.RunCommand("docker", "load", "-i", tempFile); err != nil {
			logrus.Warnf("Failed to load image from %s: %v", header.Name, err)
			continue
		}
	}

	logrus.Infof("Images imported from: %s", inputPath)
	return nil
}
