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
