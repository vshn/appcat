package vshnopenbao

import (
	"fmt"

	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclwrite"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type OpenBaoConfig struct {
	UI          bool            `hcl:"ui,optional"`
	LogLevel    string          `hcl:"log_level,optional"`
	LogFormat   string          `hcl:"log_format,optional"`
	ClusterName string          `hcl:"cluster_name,optional"`
	APIAddr     string          `hcl:"api_addr,optional"`
	ClusterAddr string          `hcl:"cluster_addr,optional"`
	PidFile     string          `hcl:"pid_file,optional"`
	Listeners   []ListenerBlock `hcl:"listener,block"`
}

type OpenBaoConfigOption func(*OpenBaoConfig)

type ListenerBlock struct {
	Type               string `hcl:"type,label"`
	Address            string `hcl:"address"`
	ClusterAddress     string `hcl:"cluster_address,optional"`
	TLSDisable         bool   `hcl:"tls_disable,optional"`
	TLSCertFile        string `hcl:"tls_cert_file,optional"`
	TLSKeyFile         string `hcl:"tls_key_file,optional"`
	TLSMinVersion      string `hcl:"tls_min_version,optional"`
	TLSMaxVersion      string `hcl:"tls_max_version,optional"`
	MaxRequestSize     *int64 `hcl:"max_request_size,optional"`
	MaxRequestDuration string `hcl:"max_request_duration,optional"`
}

func NewOpenBaoConfig(opts ...OpenBaoConfigOption) *OpenBaoConfig {
	config := &OpenBaoConfig{}

	for _, opt := range opts {
		opt(config)
	}

	return config
}

func EmptyOption() OpenBaoConfigOption {
	return func(config *OpenBaoConfig) {}
}

func WithDefaultOptions() OpenBaoConfigOption {
	return func(config *OpenBaoConfig) {
		config.UI = true
		config.LogLevel = "info"
		config.LogFormat = "json"
		config.PidFile = "/var/run/openbao.pid"
	}
}

func WithClusterName(clusterName string) OpenBaoConfigOption {
	return func(config *OpenBaoConfig) {
		config.ClusterName = clusterName
	}
}

func WithAPIAddr(addr string) OpenBaoConfigOption {
	return func(config *OpenBaoConfig) {
		config.APIAddr = addr
	}
}

func WithClusterAddr(addr string) OpenBaoConfigOption {
	return func(config *OpenBaoConfig) {
		config.ClusterAddr = addr
	}
}

func WithTLSListener(listener ListenerBlock) OpenBaoConfigOption {
	return func(config *OpenBaoConfig) {
		config.Listeners = append(config.Listeners, listener)
	}
}

func createHCLConfig(comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) error {
	serviceName := comp.GetName()
	ns := comp.GetInstanceNamespace()
	rsn := newOpenBaoResourceNames(serviceName)

	config := NewOpenBaoConfig(
		WithDefaultOptions(),
		WithAPIAddr(fmt.Sprintf("https://%s:8200", serviceName)),
		WithClusterAddr(fmt.Sprintf("https://%s:8201", serviceName)),
		WithTLSListener(ListenerBlock{
			Type:           "tcp",
			Address:        "[::]:8200",
			ClusterAddress: "[::]:8201",
			TLSDisable:     false,
			TLSCertFile:    fmt.Sprintf("%s/tls.crt", TlsCertsMountPath),
			TLSKeyFile:     fmt.Sprintf("%s/tls.key", TlsCertsMountPath),
		}),
		WithClusterName(serviceName),
	)

	hclBytes := EncodeHCL(config)

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rsn.HclConfigSecretName,
			Namespace: ns,
		},
		Data: map[string][]byte{
			HclConfigFileName: hclBytes,
		},
	}

	err := svc.SetDesiredKubeObject(secret, rsn.HclConfigSecretName)
	if err != nil {
		return fmt.Errorf("cannot add %s secret object: %w", rsn.HclConfigSecretName, err)
	}

	return nil
}

// EncodeHCL encodes an OpenBaoConfig struct into HCL format bytes.
// Returns the HCL-formatted byte slice.
func EncodeHCL(config *OpenBaoConfig) []byte {
	hclFile := hclwrite.NewEmptyFile()
	gohcl.EncodeIntoBody(config, hclFile.Body())
	return hclFile.Bytes()
}

// DecodeHCL decodes HCL format bytes into an OpenBaoConfig struct.
// Returns the decoded config and an error if parsing or decoding fails.
func DecodeHCL(hclBytes []byte, filename string) (*OpenBaoConfig, error) {
	parser := hclparse.NewParser()
	hclFile, diag := parser.ParseHCL(hclBytes, filename)
	if diag.HasErrors() {
		return nil, fmt.Errorf("failed to parse HCL: %w", diag)
	}

	var config OpenBaoConfig
	diag = gohcl.DecodeBody(hclFile.Body, nil, &config)
	if diag.HasErrors() {
		return nil, fmt.Errorf("failed to decode HCL: %w", diag)
	}

	return &config, nil
}
