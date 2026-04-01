package vshnopenbao

import (
	"sync"
)

type ServiceDetails struct {
	ServiceName          string
	SelfSignedIssuerName string
	RootCAIssuerName     string
	RootCAName           string
	ServerCertName       string
	RootCASecretName     string
	ServerCertSecretName string
	HclConfigSecretName  string
	HclConfigFileName    string
	HclConfigMountPath   string
	HclVolumeName        string
	TlsCertsMountPath    string
	TlsVolumeName        string
	RaftDataPath         string
	ApiPort              string
	ApiAddress           string
	ClusterPort          string
	ClusterAddress       string
}

var (
	serviceDetailsCache = map[string]*ServiceDetails{}
	serviceDetailsMu    sync.Mutex
)

func getServiceDetails(serviceName string) *ServiceDetails {
	serviceDetailsMu.Lock()
	defer serviceDetailsMu.Unlock()

	if existingDetails, ok := serviceDetailsCache[serviceName]; ok {
		return existingDetails
	}

	newDetails := &ServiceDetails{
		ServiceName:          serviceName,
		SelfSignedIssuerName: serviceName + "-selfsigned-issuer",
		RootCAIssuerName:     serviceName + "-ca-issuer",
		RootCAName:           serviceName + "-ca",
		RootCASecretName:     serviceName + "-ca-tls",
		ServerCertName:       serviceName + "-server",
		ServerCertSecretName: serviceName + "-server-tls",
		HclConfigSecretName:  serviceName + "-hcl-config",
		HclConfigFileName:    "config.hcl",
		HclConfigMountPath:   "/openbao/userconfig/openbao-hcl-config",
		HclVolumeName:        "userconfig-openbao-storage-config",
		TlsCertsMountPath:    "/openbao/userconfig/openbao-tls",
		TlsVolumeName:        "userconfig-openbao-tls-secret",
		RaftDataPath:         "/openbao/data",
		ApiPort:              "8200",
		ClusterPort:          "8201",
	}
	serviceDetailsCache[serviceName] = newDetails
	return newDetails
}
