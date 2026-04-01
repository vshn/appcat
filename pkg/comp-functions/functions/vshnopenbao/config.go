package vshnopenbao

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
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
	Listeners   []ListenerBlock `hcl:"listener,block"`
	Storage     []StorageBlock  `hcl:"storage,block"`
}

type ListenerBlock struct {
	Type           string `hcl:"type,label"`
	Address        string `hcl:"address"`
	ClusterAddress string `hcl:"cluster_address,optional"`
	TLSDisable     bool   `hcl:"tls_disable"`
	TLSCertFile    string `hcl:"tls_cert_file,optional"`
	TLSKeyFile     string `hcl:"tls_key_file,optional"`
}

type StorageBlock struct {
	Type      string      `hcl:"type,label"`
	Path      string      `hcl:"path"`
	RetryJoin []RetryJoin `hcl:"retry_join,block"`
}

type RetryJoin struct {
	AutoJoin *string `hcl:"auto_join,optional"`
}

func CreateHCLConfigMap(ctx context.Context, comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) *xfnproto.Result {
	serviceName := comp.GetName()
	ns := comp.GetInstanceNamespace()
	details := getServiceDetails(serviceName)

	config := &OpenBaoConfig{
		UI:          true,
		LogLevel:    "info",
		LogFormat:   "json",
		ClusterName: serviceName,
		APIAddr:     fmt.Sprintf("https://%s:%s", serviceName, details.ApiPort),
		ClusterAddr: fmt.Sprintf("https://%s:%s", serviceName, details.ClusterPort),
		Listeners: []ListenerBlock{
			{
				Type:           "tcp",
				Address:        fmt.Sprintf("[::]:%s", details.ApiPort),
				ClusterAddress: fmt.Sprintf("[::]:%s", details.ClusterPort),
				TLSDisable:     false,
				TLSCertFile:    fmt.Sprintf("%s/tls.crt", details.TlsCertsMountPath),
				TLSKeyFile:     fmt.Sprintf("%s/tls.key", details.TlsCertsMountPath),
			},
		},
		Storage: []StorageBlock{
			{
				Type: "raft",
				Path: details.RaftDataPath,
			},
		},
	}

	hclBytes := EncodeHCL(config)

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      details.HclConfigSecretName,
			Namespace: ns,
		},
		Data: map[string][]byte{
			details.HclConfigFileName: hclBytes,
		},
	}

	err := svc.SetDesiredKubeObject(secret, details.HclConfigSecretName)
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot add %s secret object: %w", details.HclConfigSecretName, err).Error())
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
