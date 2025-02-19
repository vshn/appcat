package vshnforgejo

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
)

//go:embed script/backup.sh
var forgejoBackupScript string

func AddBackup(ctx context.Context, comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetDesiredComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
	}

	common.SetRandomSchedules(comp, comp)
	err = backup.AddK8upBackup(ctx, svc, comp)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot add k8s backup to the desired state: %v", err))
	}

	err = backup.AddBackupScriptCM(svc, comp, forgejoBackupScript)
	if err != nil {
		return runtime.NewFatalResult(err)
	}

	err = updateRelease(svc, comp)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot update release with backup configuration: %s", err))
	}

	return nil
}

func updateRelease(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo) error {
	release := &xhelmv1.Release{}

	err := svc.GetDesiredComposedResourceByName(release, comp.GetName()+"-release")
	if err != nil {
		return err
	}

	values, err := common.GetReleaseValues(release)
	if err != nil {
		return err
	}

	err = backup.AddPVCAnnotationToValues(values, "persistence", "annotations")
	if err != nil {
		return fmt.Errorf("cannot add pvc annotations to values: %w", err)
	}

	err = backup.AddPodAnnotationToValues(values, "/scripts/backup.sh", ".zip", "gitea", "podAnnotations")
	if err != nil {
		return fmt.Errorf("cannot add pod annotations to values: %w", err)
	}

	err = backup.AddBackupCMToValues(values, []string{"extraVolumes"}, []string{"extraContainerVolumeMounts"})
	if err != nil {
		return fmt.Errorf("cannot add backup cm to values: %w", err)
	}

	byteValues, err := json.Marshal(values)
	if err != nil {
		return err
	}
	release.Spec.ForProvider.Values.Raw = byteValues

	return svc.SetDesiredComposedResourceWithName(release, comp.GetName()+"-release")
}
