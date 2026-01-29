package vshnopenbao

const (
	HclConfigFileName  = "config.hcl"
	HclConfigMountPath = "/openbao/userconfig/openbao-hcl-config"
	HclVolumeName      = "userconfig-openbao-storage-config"
	TlsCertsMountPath  = "/openbao/userconfig/openbao-tls"
	TlsVolumeName      = "userconfig-openbao-tls-secret"
	RaftDataPath       = "/openbao/data"
)

type OpenBaoResourceNames struct {
	ServiceName          string
	SelfSignedIssuerName string
	RootCAIssuerName     string
	RootCAName           string
	ServerCertName       string
	RootCASecretName     string
	ServerCertSecretName string
	HclConfigSecretName  string
}

func newOpenBaoResourceNames(serviceName string) *OpenBaoResourceNames {
	return &OpenBaoResourceNames{
		ServiceName:          serviceName,
		SelfSignedIssuerName: serviceName + "-selfsigned-issuer",
		RootCAIssuerName:     serviceName + "-ca-issuer",
		RootCAName:           serviceName + "-ca",
		RootCASecretName:     serviceName + "-ca-tls",
		ServerCertName:       serviceName + "-server",
		ServerCertSecretName: serviceName + "-server-tls",
		HclConfigSecretName:  serviceName + "-hcl-config",
	}
}
