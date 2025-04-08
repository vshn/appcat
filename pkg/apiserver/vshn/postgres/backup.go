package postgres

import (
	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/apiserver"
	"github.com/vshn/appcat/v4/pkg/apiserver/noop"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"k8s.io/apimachinery/pkg/runtime"
	genericregistry "k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/dynamic"
	restbuilder "sigs.k8s.io/apiserver-runtime/pkg/builder/rest"
	"sigs.k8s.io/apiserver-runtime/pkg/util/loopback"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// New returns a new storage provider for VSHNPostgresBackup
func New() restbuilder.ResourceHandlerProvider {
	return func(s *runtime.Scheme, gasdf genericregistry.RESTOptionsGetter) (rest.Storage, error) {
		c, err := client.NewWithWatch(loopback.GetLoopbackMasterClientConfig(), client.Options{})
		if err != nil {
			return nil, err
		}

		err = vshnv1.AddToScheme(c.Scheme())
		if err != nil {
			return nil, err
		}

		noopImplementation := noop.New(s, &appcatv1.VSHNPostgresBackup{}, &appcatv1.VSHNPostgresBackupList{})

		if !utils.IsKindAvailable(vshnv1.GroupVersion, "VSHNPostgreSQL", loopback.GetLoopbackMasterClientConfig()) {
			return noopImplementation, nil
		}

		dc, err := dynamic.NewForConfig(loopback.GetLoopbackMasterClientConfig())
		if err != nil {
			return nil, err
		}
		return &vshnPostgresBackupStorage{
			sgbackups: &KubeSGBackupProvider{
				DynamicClient: dc.Resource(SGbackupGroupVersionResource),
			},
			vshnpostgresql: &kubeVSHNPostgresqlProvider{
				ClientConfigurator: apiserver.New(c),
			},
			Noop: *noopImplementation,
		}, nil
	}
}

type vshnPostgresBackupStorage struct {
	sgbackups      sgbackupProvider
	vshnpostgresql vshnPostgresqlProvider
	noop.Noop
}

// GetSingularName is needed for the OpenAPI Registartion
func (in *vshnPostgresBackupStorage) GetSingularName() string {
	return "vshnpostgresbackup"
}

func (v vshnPostgresBackupStorage) New() runtime.Object {
	return &appcatv1.VSHNPostgresBackup{}
}

func (v vshnPostgresBackupStorage) Destroy() {}

var _ rest.Scoper = &vshnPostgresBackupStorage{}
var _ rest.Storage = &vshnPostgresBackupStorage{}

func (v *vshnPostgresBackupStorage) NamespaceScoped() bool {
	return true
}
