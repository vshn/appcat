package redis

import (
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
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

var _ rest.Scoper = &vshnRedisBackupStorage{}
var _ rest.Storage = &vshnRedisBackupStorage{}

type vshnRedisBackupStorage struct {
	snapshothandler k8up.Snapshothandler
	vshnRedis       vshnRedisProvider
	noop.Noop
}

// New returns a new resthandler for Redis backups.
func New() restbuilder.ResourceHandlerProvider {
	return func(s *runtime.Scheme, gasdf genericregistry.RESTOptionsGetter) (rest.Storage, error) {
		c, err := client.NewWithWatch(loopback.GetLoopbackMasterClientConfig(), client.Options{})
		if err != nil {
			return nil, err
		}

		_ = k8upv1.AddToScheme(c.Scheme())

		noopImplementation := noop.New(s, &appcatv1.VSHNRedisBackup{}, &appcatv1.VSHNRedisBackupList{})

		if !utils.IsKindAvailable(vshnv1.GroupVersion, "XVSHNRedis", loopback.GetLoopbackMasterClientConfig()) {
			return noopImplementation, nil
		}

		return &vshnRedisBackupStorage{
			snapshothandler: k8up.New(c),
			vshnRedis: &concreteRedisProvider{
				client: c,
			},
			Noop: *noopImplementation,
		}, nil
	}
}

func (v vshnRedisBackupStorage) New() runtime.Object {
	return &appcatv1.VSHNRedisBackup{}
}

// GetSingularName is needed for the OpenAPI Registartion
func (in *vshnRedisBackupStorage) GetSingularName() string {
	return "vshnredisbackup"
}

func (v vshnRedisBackupStorage) Destroy() {}

func (v *vshnRedisBackupStorage) NamespaceScoped() bool {
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
