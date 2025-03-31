package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type CertGenerator struct {
	Domain       string
	Country      string
	State        string
	Locality     string
	Organization string
	Unit         string
	Days         int
	KeySize      int
	OutputDir    string // 新增输出目录字段
}

type CertConfig struct {
	Country      string
	State        string
	Locality     string
	Organization string
	Unit         string
	Days         int
	KeySize      int
	OutputDir    string // 配置中增加输出目录
}

var DefaultConfig = CertConfig{
	Country:      "CN",
	State:        "Beijing",
	Locality:     "Beijing",
	Organization: "example",
	Unit:         "Personal",
	Days:         3650,
	KeySize:      4096,
	OutputDir:    ".", // 默认当前目录
}

func NewCertGenerator(domain string, config ...CertConfig) *CertGenerator {
	cfg := DefaultConfig
	if len(config) > 0 {
		// 仅覆盖提供的配置字段
		if config[0].Country != "" {
			cfg.Country = config[0].Country
		}
		if config[0].State != "" {
			cfg.State = config[0].State
		}
		if config[0].Locality != "" {
			cfg.Locality = config[0].Locality
		}
		if config[0].Organization != "" {
			cfg.Organization = config[0].Organization
		}
		if config[0].Unit != "" {
			cfg.Unit = config[0].Unit
		}
		if config[0].Days > 0 {
			cfg.Days = config[0].Days
		}
		if config[0].KeySize > 0 {
			cfg.KeySize = config[0].KeySize
		}
		if config[0].OutputDir != "" {
			cfg.OutputDir = config[0].OutputDir
		}
	}

	return &CertGenerator{
		Domain:       domain,
		Country:      cfg.Country,
		State:        cfg.State,
		Locality:     cfg.Locality,
		Organization: cfg.Organization,
		Unit:         cfg.Unit,
		Days:         cfg.Days,
		KeySize:      cfg.KeySize,
		OutputDir:    cfg.OutputDir,
	}
}

func (cg *CertGenerator) Generate() error {
	// 确保输出目录存在
	if err := os.MkdirAll(cg.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// 1. Generate CA private key
	if err := cg.execOpenSSL("genrsa", "-out", cg.outputPath("ca.key"), fmt.Sprintf("%d", cg.KeySize)); err != nil {
		return fmt.Errorf("failed to generate CA key: %v", err)
	}

	// 2. Generate CA certificate
	caSubject := fmt.Sprintf("/C=%s/ST=%s/L=%s/O=%s/OU=%s/CN=%s",
		cg.Country, cg.State, cg.Locality, cg.Organization, cg.Unit, cg.Domain)
	if err := cg.execOpenSSL("req", "-x509", "-new", "-nodes", "-sha512", "-days",
		fmt.Sprintf("%d", cg.Days), "-subj", caSubject, "-key", cg.outputPath("ca.key"),
		"-out", cg.outputPath("ca.crt")); err != nil {
		return fmt.Errorf("failed to generate CA cert: %v", err)
	}

	// 3. Generate server private key
	serverKey := cg.Domain + ".key"
	if err := cg.execOpenSSL("genrsa", "-out", cg.outputPath(serverKey), fmt.Sprintf("%d", cg.KeySize)); err != nil {
		return fmt.Errorf("failed to generate server key: %v", err)
	}

	// 4. Generate certificate signing request
	csrFile := cg.Domain + ".csr"
	if err := cg.execOpenSSL("req", "-sha512", "-new", "-subj", caSubject,
		"-key", cg.outputPath(serverKey), "-out", cg.outputPath(csrFile)); err != nil {
		return fmt.Errorf("failed to generate CSR: %v", err)
	}

	// 5. Create v3.ext file
	v3Ext := `authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1=` + cg.Domain + "\n"

	if err := os.WriteFile(cg.outputPath("v3.ext"), []byte(v3Ext), 0644); err != nil {
		return fmt.Errorf("failed to create v3.ext file: %v", err)
	}

	// 6. Generate server certificate
	crtFile := cg.Domain + ".crt"
	if err := cg.execOpenSSL("x509", "-req", "-sha512", "-days",
		fmt.Sprintf("%d", cg.Days), "-extfile", cg.outputPath("v3.ext"), "-CA", cg.outputPath("ca.crt"),
		"-CAkey", cg.outputPath("ca.key"), "-CAcreateserial", "-in", cg.outputPath(csrFile),
		"-out", cg.outputPath(crtFile)); err != nil {
		return fmt.Errorf("failed to generate server cert: %v", err)
	}

	// 7. Generate .cert file
	certFile := cg.Domain + ".cert"
	if err := cg.execOpenSSL("x509", "-inform", "PEM", "-in", cg.outputPath(crtFile),
		"-out", cg.outputPath(certFile)); err != nil {
		return fmt.Errorf("failed to generate .cert file: %v", err)
	}

	return nil
}

func (cg *CertGenerator) execOpenSSL(args ...string) error {
	cmd := exec.Command("openssl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// 辅助方法：生成输出文件的完整路径
func (cg *CertGenerator) outputPath(filename string) string {
	return filepath.Join(cg.OutputDir, filename)
}
