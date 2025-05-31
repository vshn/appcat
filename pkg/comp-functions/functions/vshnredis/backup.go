package vshnredis

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/backup"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

//go:embed script/backup.sh
var redisBackupScript string

// AddBackup creates an object bucket and a K8up schedule to do the actual backup.
func AddBackup(ctx context.Context, comp *vshnv1.VSHNRedis, svc *runtime.ServiceRuntime) *xfnproto.Result {

	l := controllerruntime.LoggerFrom(ctx)

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

	l.Info("Adding backup script config map")
	err = backup.AddBackupScriptCM(svc, comp, redisBackupScript)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create backup script configMap: %s", err.Error()))
	}

	l.Info("Updating the release object")
	err = updateRelease(ctx, svc)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot update release: %s", err.Error()))
	}

	return nil
}

func updateRelease(ctx context.Context, svc *runtime.ServiceRuntime) error {
	l := controllerruntime.LoggerFrom(ctx)

	release := &xhelmv1.Release{}

	err := svc.GetDesiredComposedResourceByName(release, redisRelease)
	if err != nil {
		return err
	}

	values, err := common.GetReleaseValues(release)
	if err != nil {
		return err
	}

	l.Info("Adding the PVC k8up annotations")
	if err := backup.AddPVCAnnotationToValues(values, "master", "persistence", "annotations"); err != nil {
		return err
	}

	l.Info("Adding the Pod k8up annotations")
	if err := backup.AddPodAnnotationToValues(values, "/scripts/backup.sh", ".tar", "master", "podAnnotations"); err != nil {
		return err
	}

	l.Info("Mounting CM into pod")
	if err := backup.AddBackupCMToValues(values, []string{"master", "extraVolumes"}, []string{"master", "extraVolumeMounts"}); err != nil {
		return err
	}

	byteValues, err := json.Marshal(values)
	if err != nil {
		return err
	}
	release.Spec.ForProvider.Values.Raw = byteValues

	return svc.SetDesiredComposedResourceWithName(release, redisRelease)
}
