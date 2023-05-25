package vshnpostgres

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	sgv1 "github.com/vshn/appcat/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

var sgDbOpsRestartRetention time.Duration = 30 * 24 * time.Hour

// TransformRestart triggers a restart of the postgreqsql instance if there is a pending restart and the composite is configured to restart on update.
func TransformRestart(ctx context.Context, iof *runtime.Runtime) runtime.Result {
	return transformRestart(ctx, iof, time.Now)
}

func transformRestart(ctx context.Context, iof *runtime.Runtime, now func() time.Time) runtime.Result {
	comp := vshnv1.VSHNPostgreSQL{}
	err := iof.Desired.GetComposite(ctx, &comp)
	if err != nil {
		return runtime.NewFatal(ctx, err.Error())
	}
	err = keepRecentRestartOps(ctx, iof, comp.GetName(), now)
	if err != nil {
		return runtime.NewFatal(ctx, err.Error())
	}

	if comp.Spec.Parameters.UpdateStrategy.Type == vshnv1.VSHNPostgreSQLUpdateStrategyTypeOnRestart {
		return runtime.NewNormal()
	}

	restartTime, err := getPendingRestart(ctx, iof)
	if err != nil {
		return runtime.NewWarning(ctx, err.Error())
	}

	if restartTime.IsZero() {
		return runtime.NewNormal()
	}

	err = scheduleRestart(ctx, iof, comp.GetName(), restartTime)
	if err != nil {
		return runtime.NewFatal(ctx, err.Error())
	}

	return runtime.NewNormal()
}

func getPendingRestart(ctx context.Context, iof *runtime.Runtime) (time.Time, error) {
	cluster := sgv1.SGCluster{}

	err := iof.Observed.GetFromObject(ctx, &cluster, "cluster")
	if err != nil {
		return time.Time{}, err
	}

	log := controllerruntime.LoggerFrom(ctx).WithValues("cluster", cluster.Name)
	if cluster.Status.Conditions == nil {
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

func scheduleRestart(ctx context.Context, iof *runtime.Runtime, compName string, restartTime time.Time) error {
	name := fmt.Sprintf("pg-restart-%d", restartTime.Unix())
	ops := sgv1.SGDbOps{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: fmt.Sprintf("vshn-postgresql-%s", compName),
		},
		Spec: sgv1.SGDbOpsSpec{
			Op:        sgv1.SGDbOpsOpRestart,
			SgCluster: compName,
			Restart: &sgv1.SGDbOpsSpecRestart{
				Method:             pointer.String(sgv1.SGDbOpsRestartMethodInPlace),
				OnlyPendingRestart: pointer.Bool(true),
			},
			RunAt: pointer.String(restartTime.Format(time.RFC3339)),
		},
	}
	return iof.Desired.PutIntoObject(ctx, &ops, fmt.Sprintf("%s-%s", compName, name))
}

func keepRecentRestartOps(ctx context.Context, iof *runtime.Runtime, compName string, now func() time.Time) error {

	for _, r := range iof.Observed.List(ctx) {
		if !strings.HasPrefix(r.GetName(), fmt.Sprintf("%s-pg-restart", compName)) {
			continue
		}

		op := sgv1.SGDbOps{}
		err := iof.Observed.GetFromObject(ctx, &op, r.GetName())
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

		err = iof.Desired.PutIntoObject(ctx, &op, r.GetName())
		if err != nil {
			return err
		}
	}
	return nil
}
