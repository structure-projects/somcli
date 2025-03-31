package images

import (
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/structure-projects/somcli/pkg/utils"
)

func Push(config Config) error {
	if err := validateScope(config.Scope); err != nil {
		return err
	}

	images, err := getImageList(config.Scope, config.InputFile)
	if err != nil {
		return err
	}

	for _, img := range images {
		localName := formatImageName(img, "")
		remoteName := formatImageName(img, config.Repo)

		if config.Repo != "" && !strings.HasPrefix(img.Name, config.Repo) {
			logrus.Infof("Tagging image %s as %s", localName, remoteName)
			if err := utils.RunCommand("docker", "tag", localName, remoteName); err != nil {
				logrus.Warnf("Failed to tag image %s as %s: %v", localName, remoteName, err)
				continue
			}
		}

		logrus.Infof("Pushing image: %s", remoteName)
		if err := utils.RunCommand("docker", "push", remoteName); err != nil {
			logrus.Warnf("Failed to push image %s: %v", remoteName, err)
			continue
		}
	}

	return nil
}
