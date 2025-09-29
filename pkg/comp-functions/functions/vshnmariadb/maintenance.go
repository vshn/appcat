package vshnmariadb

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/maintenance"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// AddMaintenanceJob will add a job to do the maintenance for the instance
func AddMaintenanceJob(ctx context.Context, comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime) *xfnproto.Result {
	if err := svc.GetObservedComposite(comp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("can't get composite: %w", err))
	}

	maintTime := common.SetRandomMaintenanceSchedule(comp)
	common.SetRandomBackupSchedule(comp, &maintTime)

	if err := svc.SetDesiredCompositeStatus(comp); err != nil {
		svc.Log.Error(err, "cannot set schedules in the composite status")
	}

	instanceNamespace := comp.GetInstanceNamespace()
	schedule := comp.GetFullMaintenanceSchedule()

	result := maintenance.New(comp, svc, schedule, instanceNamespace, comp.GetServiceName()).
		WithHelmBasedService().
		Run(ctx)

	if result != nil {
		return result
	}

	return AddInitialMaintenanceJob(ctx, comp, svc)
}

// AddInitialMaintenanceJob creates a one-time job from the CronJob template to run maintenance immediately after provisioning
func AddInitialMaintenanceJob(ctx context.Context, comp *vshnv1.VSHNMariaDB, svc *runtime.ServiceRuntime) *xfnproto.Result {
	cronJobName := comp.GetName() + "-maintenancejob"
	initialJobName := comp.GetName() + "-initial-maintenance"

	observedJob := &batchv1.Job{}
	err := svc.GetObservedKubeObject(observedJob, initialJobName)
	if err == nil {
		// Keep it in the desired state to avoid deletion
		errSet := svc.SetDesiredKubeObject(observedJob, initialJobName, runtime.KubeOptionAllowDeletion)
		if errSet != nil {
			return runtime.NewFatalResult(fmt.Errorf("failed to set desired kube object for existing job: %w", errSet))
		}
		return nil
	}

	observedCronJob := &batchv1.CronJob{}
	err = svc.GetObservedKubeObject(observedCronJob, cronJobName)
	if err != nil {
		return nil
	}

	svc.Log.Info("Creating initial maintenance job from CronJob template")

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      initialJobName,
			Namespace: observedCronJob.Namespace,
			Labels:    observedCronJob.Labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            ptr.To(int32(2)),
			TTLSecondsAfterFinished: ptr.To(int32(3600)),
			Template:                observedCronJob.Spec.JobTemplate.Spec.Template,
		},
	}

	errSet := svc.SetDesiredKubeObject(job, initialJobName, runtime.KubeOptionAllowDeletion)
	if errSet != nil {
		return runtime.NewFatalResult(fmt.Errorf("failed to set desired kube object: %w", errSet))
	}
	return nil
}
