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
package registry

import (
	"fmt"
	"strings"

	"github.com/structure-projects/somcli/pkg/utils"
)

type RegistrySyncer struct {
	Source, Target, Username, Password string
	Concurrency                        int
}

func NewRegistrySyncer(source, target, username, password string, concurrency int) *RegistrySyncer {
	return &RegistrySyncer{
		Source:      source,
		Target:      target,
		Username:    username,
		Password:    password,
		Concurrency: concurrency,
	}
}

func (rs *RegistrySyncer) SyncImage(image string) error {
	targetHost := strings.ReplaceAll(rs.Target, "https://", "")
	targetHost = strings.ReplaceAll(targetHost, "http://", "")
	utils.PrintInfo("targetHost %s", targetHost)
	targetImage := image
	if strings.Contains(image, rs.Source) {
		targetImage = strings.ReplaceAll(image, rs.Source, targetHost)
	} else if !strings.Contains(image, "/") {
		targetImage = targetHost + "/library/" + image
	} else {
		targetImage = targetHost + "/" + image
	}
	utils.PrintInfo("targetImage %s", targetImage)

	// 拉取->打标签->推送->清理
	steps := []struct {
		cmd    string
		args   []string
		errMsg string
	}{
		{"docker", []string{"pull", image}, "pull"},
		{"docker", []string{"tag", image, targetImage}, "tag"},
		{"docker", []string{"push", targetImage}, "push"},
		{"docker", []string{"rmi", image, targetImage}, "cleanup"},
	}

	for _, step := range steps {
		if err := utils.RunCommand(step.cmd, step.args...); err != nil && step.errMsg != "cleanup" {
			return fmt.Errorf("failed to %s image %s: %v", step.errMsg, image, err)
		}
	}
	return nil
}

func (rs *RegistrySyncer) SyncAll(images []string) error {

	if err := rs.login(rs.Target, rs.Username, rs.Password); err != nil {
		return fmt.Errorf("target login failed: %v", err)
	}

	errChan := make(chan error, len(images))
	sem := make(chan struct{}, rs.Concurrency)

	for _, img := range images {
		sem <- struct{}{}
		go func(image string) {
			defer func() { <-sem }()
			if err := rs.SyncImage(image); err != nil {
				errChan <- err
			}
		}(img)
	}

	// 等待所有goroutine完成
	for i := 0; i < cap(sem); i++ {
		sem <- struct{}{}
	}
	close(errChan)

	var errs []string
	for err := range errChan {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("sync completed with %d errors:\n%s", len(errs), strings.Join(errs, "\n"))
	}
	return nil
}

func (rs *RegistrySyncer) login(registry string, username string, password string) error {
	if rs.Username == "" || rs.Password == "" {
		return nil
	}
	return utils.RunCommand("docker", "login", "-u", username, "-p", password, registry)
}
