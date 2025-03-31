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

type Config struct {
	Scope      string
	Repo       string
	CustomFile string
	InputFile  string
	OutputFile string
}

type Image struct {
	Name string `yaml:"name"`
	Tag  string `yaml:"tag"`
}

const (
	ScopeHarbor = "harbor"
	ScopeK8s    = "k8s"
	ScopeAll    = "all"
)
