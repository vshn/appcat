package vshnpostgres

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	sgv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

var sgDbOpsRestartRetention time.Duration = 30 * 24 * time.Hour

// TransformRestart triggers a restart of the postgreqsql instance if there is a pending restart and the composite is configured to restart on update.
func TransformRestart(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	return transformRestart(ctx, svc, time.Now)
}

func transformRestart(ctx context.Context, svc *runtime.ServiceRuntime, now func() time.Time) *xfnproto.Result {
	comp := &vshnv1.VSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(err)
	}
	err = keepRecentRestartOps(ctx, svc, comp.GetName(), now)
	if err != nil {
		return runtime.NewFatalResult(err)
	}

	if comp.Spec.Parameters.UpdateStrategy.Type == vshnv1.VSHNPostgreSQLUpdateStrategyTypeOnRestart {
		return nil
	}

	restartTime, err := getPendingRestart(ctx, svc)
	if err != nil {
		return runtime.NewWarningResult(err.Error())
	}

	if restartTime.IsZero() {
		return nil
	}

	err = scheduleRestart(ctx, svc, comp.GetName(), restartTime)
	if err != nil {
		return runtime.NewFatalResult(err)
	}

	return nil
}

func getPendingRestart(ctx context.Context, svc *runtime.ServiceRuntime) (time.Time, error) {
	cluster := sgv1.SGCluster{}

	err := svc.GetObservedKubeObject(&cluster, "cluster")
	if err != nil {
		return time.Time{}, err
	}

	log := controllerruntime.LoggerFrom(ctx).WithValues("cluster", cluster.Name)
	if cluster.Status.Conditions == nil {
		return time.Time{}, nil
	}

	if hasPendingUpgrade(*cluster.Status.Conditions) {
		return time.Time{}, nil
	}

	for _, cond := range *cluster.Status.Conditions {
		if cond.Type == nil || *cond.Type != sgv1.SGClusterConditionTypePendingRestart || cond.Status == nil || cond.LastTransitionTime == nil {
			continue
		}

		status, err := strconv.ParseBool(*cond.Status)
		if err != nil || !status {
			continue
		}

		log.WithValues("at", cond.LastTransitionTime).Info("PendingRestart")

		restartTime, err := time.Parse(time.RFC3339, *cond.LastTransitionTime)
		if err != nil {
			continue
		}
		return restartTime, nil
	}

	return time.Time{}, nil
}

// In case the operator was updated and the PostgreSQL clusters require a security maintenance, we ensure that it only happens during the maintenance window
func hasPendingUpgrade(items []sgv1.SGClusterStatusConditionsItem) bool {
	for _, i := range items {
		if *i.Type == sgv1.SGClusterConditionTypePendingUpgrade && *i.Status == "True" {
			return true
		}
	}
	return false
}

func scheduleRestart(ctx context.Context, svc *runtime.ServiceRuntime, compName string, restartTime time.Time) error {
	name := fmt.Sprintf("pg-restart-%d", restartTime.Unix())
	ops := &sgv1.SGDbOps{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: fmt.Sprintf("vshn-postgresql-%s", compName),
		},
		Spec: sgv1.SGDbOpsSpec{
			Op:        sgv1.SGDbOpsOpRestart,
			SgCluster: compName,
			Restart: &sgv1.SGDbOpsSpecRestart{
				Method:             ptr.To(sgv1.SGDbOpsRestartMethodInPlace),
				OnlyPendingRestart: ptr.To(true),
			},
			RunAt: ptr.To(restartTime.Format(time.RFC3339)),
		},
	}
	return svc.SetDesiredKubeObject(ops, fmt.Sprintf("%s-%s", compName, name))
}

func keepRecentRestartOps(ctx context.Context, svc *runtime.ServiceRuntime, compName string, now func() time.Time) error {

	objects, err := svc.GetAllObserved()
	if err != nil {
		return err
	}

	for _, r := range objects {
		if !strings.HasPrefix(r.Resource.GetName(), fmt.Sprintf("%s-pg-restart", compName)) {
			continue
		}

		op := sgv1.SGDbOps{}
		err := svc.GetObservedKubeObject(&op, r.Resource.GetName())
		if err != nil {
			continue
		}

		if op.Spec.Op != "restart" || op.Spec.RunAt == nil {
			continue
		}

		runAt, err := time.Parse(time.RFC3339, *op.Spec.RunAt)
		if err != nil {
			continue
		}

		if runAt.Before(now().Add(-1 * sgDbOpsRestartRetention)) {
			continue
		}

		err = svc.SetDesiredKubeObject(&op, r.Resource.GetName())
		if err != nil {
			return err
		}
	}
	return nil
}
