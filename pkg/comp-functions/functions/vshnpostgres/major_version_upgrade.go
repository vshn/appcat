package vshnpostgres

import (
	"context"
	"errors"
	"fmt"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pointer "k8s.io/utils/ptr"
)

const (
	majorUpgradeSuffix = "-major-upgrade-dbops"
)

func MajorVersionUpgrade(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	comp, err := getVSHNPostgreSQL(ctx, svc)

	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get composite from function io: %w", err))
	}

	expectedV := comp.Spec.Parameters.Service.MajorVersion
	currentV := comp.Status.MajorVersion

	majorUpgradeDbOps := &stackgresv1.SGDbOps{}
	err = svc.GetObservedKubeObject(majorUpgradeDbOps, comp.GetName()+majorUpgradeSuffix)
	if err != nil && !errors.Is(err, runtime.ErrNotFound) {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get observed kube object major upgrade sgdbops: %w", err))
	}

	// If current and expected versions do not match then issue a major version upgrade via SGDBOps resource
	if currentV != "" && currentV != expectedV {
		// If SGDBOps resource does not exist create it otherwise cleanup if successful or keep the resource on fail
		if errors.Is(err, runtime.ErrNotFound) {
			return createMajorUpgradeSgDbOps(svc, comp, expectedV)
		} else if isSuccessful(majorUpgradeDbOps.Status.Conditions) {
			return cleanUp(svc, comp, expectedV)
		} else {
			return keepSgDbOpsResource(svc, comp, majorUpgradeDbOps)
		}
	}

	return runtime.NewNormalResult("No major upgrade issued")
}

func keepSgDbOpsResource(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL, majorUpgradeDbOps *stackgresv1.SGDbOps) *xfnproto.Result {
	err := svc.SetDesiredKubeObject(majorUpgradeDbOps, comp.GetName()+majorUpgradeSuffix)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot keep major upgrade kube object %s", comp.GetName()))
	}
	return runtime.NewWarningResult("Major upgrade is not completed or it failed")
}

func cleanUp(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL, expectedV string) *xfnproto.Result {
	comp.Status.MajorVersion = expectedV
	err := svc.SetDesiredCompositeStatus(comp)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot update status field with the newest major postgres version: %w", err))
	}
	return runtime.NewNormalResult("Major upgrade successfully finished, SGDBOps cleaned up")
}

func isSuccessful(conditions *[]stackgresv1.SGDbOpsStatusConditionsItem) bool {
	var successful, completed bool
	if conditions != nil {
		for _, c := range *conditions {
			if !(*c.Reason == "OperationFailed" && *c.Status == "True") {
				successful = true
			}
			if *c.Reason == "OperationCompleted" && *c.Status == "True" {
				completed = true
			}
		}
	}
	return successful && completed
}

func createMajorUpgradeSgDbOps(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL, expectedV string) *xfnproto.Result {
	cluster := &stackgresv1.SGCluster{}
	err := svc.GetObservedKubeObject(cluster, "cluster")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get observed kube object cluster: %w", err))
	}

	conf := &stackgresv1.SGPostgresConfig{}
	err = svc.GetObservedKubeObject(conf, comp.GetName()+"-"+configResourceName)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get observed kube object postgres config: %w", err))
	}
	conf.Spec.PostgresVersion = expectedV
	err = svc.SetDesiredKubeObject(conf, comp.GetName()+"-"+configResourceName)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot set observed kube object postgres config %s", comp.GetName()))
	}

	sgdbops := &stackgresv1.SGDbOps{
		ObjectMeta: v1.ObjectMeta{
			Name:      comp.GetName() + majorUpgradeSuffix,
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: stackgresv1.SGDbOpsSpec{
			MajorVersionUpgrade: &stackgresv1.SGDbOpsSpecMajorVersionUpgrade{
				Check:            pointer.To(true),
				Clone:            nil,
				Link:             pointer.To(true),
				PostgresVersion:  &expectedV,
				SgPostgresConfig: pointer.To(conf.GetName()),
			},
			Op:        "majorVersionUpgrade",
			SgCluster: cluster.GetName(),
		},
	}
	err = svc.SetDesiredKubeObject(sgdbops, comp.GetName()+majorUpgradeSuffix)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create major upgrade kube object %s", comp.GetName()))
	}
	return runtime.NewNormalResult("SGDBOps for major upgrade created")
}
