package vshnmariadb

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/backup"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

//go:embed script/backup.sh
var mariadbBackupScript string

// AddBackupMariadb adds k8up backup to a MariaDB deployment.
func AddBackupMariadb(ctx context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {
	l := controllerruntime.LoggerFrom(ctx)

	comp := &vshnv1.VSHNMariaDB{}
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
		return runtime.NewWarningResult(fmt.Sprintf("cannot create backup: %s", err.Error()))
	}

	l.Info("Adding backup script config map")
	err = backup.AddBackupScriptCM(svc, comp, mariadbBackupScript)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create backup script configMap: %s", err.Error()))
	}

	l.Info("Updating the release object")
	err = updateRelease(ctx, svc, comp)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot update release: %s", err.Error()))
	}

	return nil
}

func updateRelease(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNMariaDB) error {
	l := controllerruntime.LoggerFrom(ctx)

	release := &xhelmv1.Release{}

	err := svc.GetDesiredComposedResourceByName(release, comp.GetName()+"-release")
	if err != nil {
		return err
	}

	values, err := common.GetReleaseValues(release)
	if err != nil {
		return err
	}

	l.Info("Adding the PVC k8up annotations")
	err = backup.AddPVCAnnotationToValues(values, "persistence", "annotations")
	if err != nil {
		return err
	}

	l.Info("Adding the Pod k8up annotations")
	err = backup.AddPodAnnotationToValues(values, "/scripts/backup.sh", ".xb", "podAnnotations")
	if err != nil {
		return err
	}

	l.Info("Mounting CM into pod")
	err = backup.AddBackupCMToValues(values, []string{"extraVolumes"}, []string{"extraVolumeMounts"})
	if err != nil {
		return err
	}

	byteValues, err := json.Marshal(values)
	if err != nil {
		return err
	}
	release.Spec.ForProvider.Values.Raw = byteValues

	return svc.SetDesiredComposedResourceWithName(release, comp.GetName()+"-release")
}
