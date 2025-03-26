package mariadb

import (
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/apiserver"
	"github.com/vshn/appcat/v4/pkg/apiserver/noop"
	"github.com/vshn/appcat/v4/pkg/apiserver/vshn/k8up"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	genericregistry "k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	restbuilder "sigs.k8s.io/apiserver-runtime/pkg/builder/rest"
	"sigs.k8s.io/apiserver-runtime/pkg/util/loopback"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ rest.Scoper = &vshnMariaDBBackupStorage{}
var _ rest.Storage = &vshnMariaDBBackupStorage{}

type vshnMariaDBBackupStorage struct {
	snapshothandler k8up.Snapshothandler
	vshnMariaDB     vshnMariaDBProvider
	noop.Noop
}

// New returns a new resthandler for MariaDB backups.
func New() restbuilder.ResourceHandlerProvider {
	return func(s *runtime.Scheme, gasdf genericregistry.RESTOptionsGetter) (rest.Storage, error) {
		c, err := client.NewWithWatch(loopback.GetLoopbackMasterClientConfig(), client.Options{})
		if err != nil {
			return nil, err
		}

		_ = k8upv1.AddToScheme(c.Scheme())

		noopImplementation := noop.New(s, &appcatv1.VSHNMariaDBBackup{}, &appcatv1.VSHNMariaDBBackupList{})

		if !utils.IsKindAvailable(vshnv1.GroupVersion, "XVSHNMariaDB", loopback.GetLoopbackMasterClientConfig()) {
			return noopImplementation, nil
		}

		return &vshnMariaDBBackupStorage{
			snapshothandler: k8up.New(c),
			vshnMariaDB: &concreteMariaDBProvider{
				ClientConfigurator: apiserver.New(c),
			},
			Noop: *noopImplementation,
		}, nil
	}
}

func (v vshnMariaDBBackupStorage) New() runtime.Object {
	return &appcatv1.VSHNMariaDBBackup{}
}

// GetSingularName is needed for the OpenAPI Registartion
func (in *vshnMariaDBBackupStorage) GetSingularName() string {
	return "vshnmariadbbackup"
}

func (v vshnMariaDBBackupStorage) Destroy() {}

func (v *vshnMariaDBBackupStorage) NamespaceScoped() bool {
	return true
}

func trimStringLength(in string) string {
	length := len(in)
	if length > 8 {
		length = 8
	}
	return in[:length]
}

func deRefString(in *string) string {
	if in == nil {
		return ""
	}
	return *in
}

func deRefMetaTime(in *metav1.Time) metav1.Time {
	if in == nil {
		return metav1.Now()
	}
	return *in
}
