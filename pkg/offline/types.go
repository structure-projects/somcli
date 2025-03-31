package offline

// DownloadConfig 下载配置文件结构
type DownloadConfig struct {
	Proxy     string             `yaml:"proxy"` // 可选代理
	Resources []DownloadResource `yaml:"download,omitempty"`
}

// DownloadResource 单个资源定义
type DownloadResource struct {
	Name     string   `yaml:"name"`
	Version  string   `yaml:"version"`
	URLs     []string `yaml:"urls"`
	Target   string   `yaml:"target"`   // 相对缓存目录的路径
	Checksum string   `yaml:"checksum"` // 可选校验和
}

// DownloadResult 下载结果
type DownloadResult struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	URL       string `json:"url"`
	LocalPath string `json:"local_path"` // 相对路径
	Error     error  `json:"error,omitempty"`
}
