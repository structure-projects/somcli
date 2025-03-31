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
