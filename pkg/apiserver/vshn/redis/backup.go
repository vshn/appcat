package redis

import (
	appcatv1 "github.com/vshn/appcat/apis/appcat/v1"
	"github.com/vshn/appcat/pkg"
	"github.com/vshn/appcat/pkg/apiserver/vshn/k8up"
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
}

// New returns a new resthandler for Redis backups.
func New() restbuilder.ResourceHandlerProvider {
	return func(s *runtime.Scheme, gasdf genericregistry.RESTOptionsGetter) (rest.Storage, error) {
		c, err := client.NewWithWatch(loopback.GetLoopbackMasterClientConfig(), client.Options{})
		if err != nil {
			return nil, err
		}

		pkg.AddToScheme(c.Scheme())
		pkg.AddToScheme(s)

		return &vshnRedisBackupStorage{
			snapshothandler: k8up.New(c),
			vshnRedis: &concreteRedisProvider{
				client: c,
			},
		}, nil
	}
}

func (v vshnRedisBackupStorage) New() runtime.Object {
	return &appcatv1.VSHNRedisBackup{}
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
