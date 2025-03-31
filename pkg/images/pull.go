package images

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/structure-projects/somcli/pkg/utils"
)

func Pull(config Config) error {
	if err := validateScope(config.Scope); err != nil {
		return err
	}

	images, err := getImageList(config.Scope, config.CustomFile)
	if err != nil {
		return err
	}

	for _, img := range images {
		fullName := formatImageName(img, config.Repo)
		logrus.Infof("Pulling image: %s", fullName)

		if err := utils.RunCommand("docker", "pull", fullName); err != nil {
			logrus.Warnf("Failed to pull image %s: %v", fullName, err)
			continue
		}

		if config.Repo != "" && !strings.HasPrefix(img.Name, config.Repo) {
			localName := formatImageName(img, "")
			if err := utils.RunCommand("docker", "tag", fullName, localName); err != nil {
				logrus.Warnf("Failed to tag image %s as %s: %v", fullName, localName, err)
				continue
			}
		}
	}

	if config.OutputFile != "" {
		if err := saveImageList(images, filepath.Join(config.OutputFile)); err != nil {
			return fmt.Errorf("failed to save image list: %v", err)
		}
	}

	return nil
}

func validateScope(scope string) error {
	switch scope {
	case ScopeHarbor, ScopeK8s, ScopeAll:
		return nil
	default:
		return fmt.Errorf("invalid scope: %s, must be one of: harbor, k8s, all", scope)
	}
}
