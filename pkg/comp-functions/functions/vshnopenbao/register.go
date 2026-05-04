package vshnopenbao

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

func init() {
	runtime.RegisterService[*vshnv1.VSHNOpenBao]("openbao", runtime.Service[*vshnv1.VSHNOpenBao]{
		Steps: []runtime.Step[*vshnv1.VSHNOpenBao]{
			{
				Name:    "bootstrap-namespace",
				Execute: BootstrapNamespace,
			},
			{
				Name:    "create-hcl-cm",
				Execute: CreateHCLConfigMap,
			},
			{
				Name:    "setup-tls-certificates",
				Execute: SetupTLSCertificates,
			},
			{
				Name:    "create-discovery-rbac",
				Execute: ConfigureRBAC,
			},
			{
				Name:    "deploy-openbao",
				Execute: DeployOpenBao,
			},
		},
	})
}
