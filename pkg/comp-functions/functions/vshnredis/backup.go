package vshnredis

import (
	"context"
	_ "embed"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/backup"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

const (
	backupScriptCMName = "backup-script"
)

//go:embed script/backup.sh
var redisBackupScript string

// AddBackup creates an object bucket and a K8up schedule to do the actual backup.
func AddBackup(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {

	l := controllerruntime.LoggerFrom(ctx)

	comp := &vshnv1.VSHNRedis{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("failed to parse composite: %w", err))
	}

	common.SetRandomSchedules(comp, comp)

	err = svc.SetDesiredCompositeStatus(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("failed to set composite: %w", err))
	}

	err = backup.AddK8upBackup(ctx, svc, comp)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot add k8up backup: %s", err.Error()))
	}

	l.Info("Creating backup config map")
	err = createScriptCM(ctx, comp, svc)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create backup config map: %w", err))
	}

	return nil
}

func createScriptCM(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) error {

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupScriptCMName,
			Namespace: getInstanceNamespace(comp),
		},
		Data: map[string]string{
			"backup.sh": redisBackupScript,
		},
	}

	return svc.SetDesiredKubeObject(cm, comp.Name+"-backup-cm")
}
