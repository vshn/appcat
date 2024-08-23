package vshnkeycloak

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func init() {
	runtime.RegisterService[*vshnv1.VSHNKeycloak]("keycloak", runtime.Service[*vshnv1.VSHNKeycloak]{
		Steps: []runtime.Step[*vshnv1.VSHNKeycloak]{

			{
				Name:    "deploy",
				Execute: DeployKeycloak,
			},
		},
	})
}

func getKeycloakFromObject(obj client.Object) *vshnv1.VSHNKeycloak {
	comp, ok := obj.(*vshnv1.VSHNKeycloak)
	if !ok {
		panic("shit")
	}
	return comp
}
