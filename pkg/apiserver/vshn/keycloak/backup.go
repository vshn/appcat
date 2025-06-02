package keycloak

import (
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/apiserver"
	"github.com/vshn/appcat/v4/pkg/apiserver/noop"
	"github.com/vshn/appcat/v4/pkg/apiserver/vshn/k8up"
	"github.com/vshn/appcat/v4/pkg/apiserver/vshn/postgres"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"k8s.io/apimachinery/pkg/runtime"
	genericregistry "k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/dynamic"
	restbuilder "sigs.k8s.io/apiserver-runtime/pkg/builder/rest"
	"sigs.k8s.io/apiserver-runtime/pkg/util/loopback"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ rest.Scoper = &vshnKeycloakBackupStorage{}
var _ rest.Storage = &vshnKeycloakBackupStorage{}

type vshnKeycloakBackupStorage struct {
	snapshothandler k8up.Snapshothandler
	vshnKeycloak    vshnKeycloakProvider
	sgBackup        postgres.KubeSGBackupProvider
	noop.Noop
}

// New returns a new resthandler for keycloak backups.
func New() restbuilder.ResourceHandlerProvider {
	return func(s *runtime.Scheme, gasdf genericregistry.RESTOptionsGetter) (rest.Storage, error) {
		c, err := client.NewWithWatch(loopback.GetLoopbackMasterClientConfig(), client.Options{})
		if err != nil {
			return nil, err
		}

		_ = k8upv1.AddToScheme(c.Scheme())

		noopImplementation := noop.New(s, &appcatv1.VSHNKeycloakBackup{}, &appcatv1.VSHNKeycloakBackupList{})

		if !utils.IsKindAvailable(vshnv1.GroupVersion, "XVSHNKeycloak", loopback.GetLoopbackMasterClientConfig()) {
			return noopImplementation, nil
		}

		dc, err := dynamic.NewForConfig(loopback.GetLoopbackMasterClientConfig())
		if err != nil {
			return nil, err
		}

		return &vshnKeycloakBackupStorage{
			snapshothandler: k8up.New(c),
			vshnKeycloak: &concreteKeycloakProvider{
				ClientConfigurator: apiserver.New(c),
			},
			Noop: *noopImplementation,
			sgBackup: postgres.KubeSGBackupProvider{
				DynamicClient: dc.Resource(postgres.SGbackupGroupVersionResource),
			},
		}, nil
	}
}

func (v vshnKeycloakBackupStorage) New() runtime.Object {
	return &appcatv1.VSHNKeycloakBackup{}
}

// GetSingularName is needed for the OpenAPI Registartion
func (*vshnKeycloakBackupStorage) GetSingularName() string {
	return "vshnkeycloakbackup"
}

func (v vshnKeycloakBackupStorage) Destroy() {}

func (v *vshnKeycloakBackupStorage) NamespaceScoped() bool {
	return true
}

func trimStringLength(in string) string {
	length := len(in)
	if length > 8 {
		length = 8
	}
	return in[:length]
}
