package nextcloud

import (
	k8upv1 "github.com/k8up-io/k8up/v2/api/v1"
	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/apiserver"
	"github.com/vshn/appcat/v4/pkg/apiserver/noop"
	"github.com/vshn/appcat/v4/pkg/apiserver/vshn/k8up"
	"github.com/vshn/appcat/v4/pkg/apiserver/vshn/postgres"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	genericregistry "k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/dynamic"
	restbuilder "sigs.k8s.io/apiserver-runtime/pkg/builder/rest"
	"sigs.k8s.io/apiserver-runtime/pkg/util/loopback"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ rest.Scoper = &vshnNextcloudBackupStorage{}
var _ rest.Storage = &vshnNextcloudBackupStorage{}

type vshnNextcloudBackupStorage struct {
	snapshothandler k8up.Snapshothandler
	vshnNextcloud   vshnNextcloudProvider
	backup          postgres.KubeBackupProvider
	noop.Noop
}

// New returns a new resthandler for nextcloud backups.
func New() restbuilder.ResourceHandlerProvider {
	return func(s *runtime.Scheme, gasdf genericregistry.RESTOptionsGetter) (rest.Storage, error) {
		c, err := client.NewWithWatch(loopback.GetLoopbackMasterClientConfig(), client.Options{})
		if err != nil {
			return nil, err
		}

		_ = k8upv1.AddToScheme(c.Scheme())

		noopImplementation := noop.New(s, &appcatv1.VSHNNextcloudBackup{}, &appcatv1.VSHNNextcloudBackupList{})

		if !utils.IsKindAvailable(vshnv1.GroupVersion, "XVSHNNextcloud", loopback.GetLoopbackMasterClientConfig()) {
			return noopImplementation, nil
		}

		dc, err := dynamic.NewForConfig(loopback.GetLoopbackMasterClientConfig())
		if err != nil {
			return nil, err
		}

		return &vshnNextcloudBackupStorage{
			snapshothandler: k8up.New(c),
			vshnNextcloud: &concreteNextcloudProvider{
				ClientConfigurator: apiserver.New(c),
			},
			Noop: *noopImplementation,
			backup: postgres.KubeBackupProvider{
				DynamicClient: dc.Resource(postgres.SGbackupGroupVersionResource),
			},
		}, nil
	}
}

func (v vshnNextcloudBackupStorage) New() runtime.Object {
	return &appcatv1.VSHNNextcloudBackup{}
}

// GetSingularName is needed for the OpenAPI Registartion
func (*vshnNextcloudBackupStorage) GetSingularName() string {
	return "vshnnextcloudbackup"
}

func (v vshnNextcloudBackupStorage) Destroy() {}

func (v *vshnNextcloudBackupStorage) NamespaceScoped() bool {
	return true
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

func trimStringLength(in string) string {
	length := len(in)
	if length > 8 {
		length = 8
	}
	return in[:length]
}
