package vshnopenbao

type OpenBaoTLSCertificate struct {
	// CACert   string
	// Key      string
	// ClientCA string

	CACertFile   string
	KeyFile      string
	ClientCAFile string
}

func NewOpenBaoTLSCertificate() *OpenBaoTLSCertificate {
	return &OpenBaoTLSCertificate{
		CACertFile:   "/etc/ssl/certs/ca.crt",
		KeyFile:      "/etc/ssl/private/key.pem",
		ClientCAFile: "/etc/ssl/certs/client-ca.crt",
	}
}
