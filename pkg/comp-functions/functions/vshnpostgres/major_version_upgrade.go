package vshnpostgres

import (
	"context"
	"errors"
	"fmt"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/auth/stackgres"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v2 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pointer "k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

const majorUpgradeSuffix = "major-upgrade-dbops"

// MajorVersionUpgrade upgrades next major postgres version. The latest minor version is applied
func MajorVersionUpgrade(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	log := ctrl.LoggerFrom(ctx)
	comp, err := getVSHNPostgreSQL(ctx, svc)

	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get composite from function io: %v", err))
	}

	expectedV := comp.Spec.Parameters.Service.MajorVersion
	currentV := comp.Status.CurrentVersion

	// If current and expected versions do not match then issue a major version upgrade via SGDBOps resource
	if currentV != "" && currentV != expectedV {
		log.Info("detected major version upgrade")
		err = upgradePGSettings(comp, svc)
		if err != nil {
			log.Error(err, "failed to upgrade PG settings during major version upgrade")
			return runtime.NewWarningResult(fmt.Sprintf("cannot upgrade PG settings: %v", err))
		}
		majorUpgradeDbOps, err := getSgDbOpsIfExists(err, svc, comp)
		if err != nil {
			log.Error(err, "failed to get major upgrade sgdbops resource")
			return runtime.NewWarningResult(fmt.Sprintf("cannot get major upgrade SgDbOps resource: %v", err))
		}

		// If SGDBOps resource does not exist create it and keep during upgrade otherwise cleanup if successful
		if majorUpgradeDbOps == nil {
			log.Info("Creating SgDbOps major version upgrade resource")
			return createMajorUpgradeSgDbOps(ctx, svc, comp, expectedV)
		} else if isSuccessful(majorUpgradeDbOps.Status.Conditions) {
			log.Info("Major version upgrade successfully completed")
			updateStatusVersion(svc, comp, expectedV)
		}
		return keepSgDbOpsResource(svc, comp, majorUpgradeDbOps)
	}
	log.Info("No major version upgrade detected")
	return nil
}

// upgradePGSettings creates a new SGPostgresConfig resource with the new expected major
func upgradePGSettings(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {
	conf := &stackgresv1.SGPostgresConfig{}
	err := svc.GetObservedKubeObject(conf, fmt.Sprintf("%s-%s-%s", comp.GetName(), configResourceName, comp.Status.CurrentVersion))
	if err != nil {
		// For compatibility purposes the old PG settings has to be considered
		err = svc.GetObservedKubeObject(conf, fmt.Sprintf("%s-%s", comp.GetName(), configResourceName))
		if err != nil {
			return fmt.Errorf("cannot get observed kube object postgres config: %v", err)
		}
	}

	expectedV := comp.Spec.Parameters.Service.MajorVersion
	upgradeVersionConfig := &stackgresv1.SGPostgresConfig{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("%s-postgres-config-%s", comp.GetName(), expectedV),
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: stackgresv1.SGPostgresConfigSpec{
			PostgresVersion: expectedV,
			PostgresqlConf:  conf.Spec.PostgresqlConf,
		},
	}
	err = svc.SetDesiredKubeObject(upgradeVersionConfig, fmt.Sprintf("%s-%s-%s", comp.GetName(), configResourceName, expectedV))
	if err != nil {
		return fmt.Errorf("cannot create current version postgres config:  %v", err)
	}
	return nil
}

// getSgDbOpsIfExists returns the existing major upgrade SgDbOps resource if upgrade is in process
func getSgDbOpsIfExists(err error, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL) (*stackgresv1.SGDbOps, error) {
	majorUpgradeDbOps := &stackgresv1.SGDbOps{}
	err = svc.GetObservedKubeObject(majorUpgradeDbOps, fmt.Sprintf("%s-%s", comp.GetName(), majorUpgradeSuffix))
	if err != nil && !errors.Is(err, runtime.ErrNotFound) {
		return nil, fmt.Errorf("cannot get observed kube object major upgrade sgdbops: %v", err)
	}
	if errors.Is(err, runtime.ErrNotFound) {
		return nil, nil
	}
	return majorUpgradeDbOps, nil
}

// keepSgDbOpsResource saves the major upgrade SgDbOps for the duration of the major upgrade
func keepSgDbOpsResource(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL, majorUpgradeDbOps *stackgresv1.SGDbOps) *xfnproto.Result {
	err := svc.SetDesiredKubeObject(majorUpgradeDbOps, fmt.Sprintf("%s-%s", comp.GetName(), majorUpgradeSuffix), runtime.KubeOptionAllowDeletion)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot keep major upgrade kube object %s", comp.GetName()))
	}
	return runtime.NewWarningResult("Major upgrade is not completed or it failed")
}

// updateStatusVersion rotates the status versions in the composite
func updateStatusVersion(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL, v string) *xfnproto.Result {
	comp.Status.PreviousVersion = comp.Status.CurrentVersion
	comp.Status.CurrentVersion = v
	err := svc.SetDesiredCompositeStatus(comp)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot update status field with the newest major postgres version: %v", err))
	}
	return runtime.NewNormalResult("Major upgrade successfully finished, SGDBOps cleaned up")
}

// isSuccessful checks whether the major version upgrade was successful
func isSuccessful(conditions *[]stackgresv1.SGDbOpsStatusConditionsItem) bool {
	if conditions == nil {
		return false
	}

	var hasCompleted, hasFailed bool
	for _, c := range *conditions {
		switch {
		case *c.Reason == "OperationFailed" && *c.Status == "True":
			hasFailed = true
		case *c.Reason == "OperationCompleted" && *c.Status == "True":
			hasCompleted = true
		}
		if hasFailed {
			return false
		}
	}

	return hasCompleted
}

// createMajorUpgradeSgDbOps create a major upgrade SgDbOps resource to start the upgrade process
func createMajorUpgradeSgDbOps(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL, expectedV string) *xfnproto.Result {
	log := ctrl.LoggerFrom(ctx)
	cluster := &stackgresv1.SGCluster{}
	err := svc.GetObservedKubeObject(cluster, "cluster")
	if err != nil {
		log.Error(err, "cannot get observed sgcluster object")
		return runtime.NewWarningResult(fmt.Sprintf("cannot get observed kube object cluster: %v", err))
	}

	sgNamespace := svc.Config.Data["sgNamespace"]
	stacgresRestApi := &v2.Secret{}
	err = svc.GetObservedKubeObject(stacgresRestApi, fmt.Sprintf("%s-%s", comp.GetName(), stackgresCredObserver))
	if err != nil {
		log.Error(err, "cannot get username and password")
		return runtime.NewWarningResult(fmt.Sprintf("cannot get observed stackgres-restapi-admin secret: %v", err))
	}

	stackgresClient, err := stackgres.New(string(stacgresRestApi.Data["k8sUsername"]), string(stacgresRestApi.Data["clearPassword"]), sgNamespace)
	if err != nil {
		log.Error(err, "cannot create stackgres client")
		return runtime.NewWarningResult(fmt.Sprintf("cannot initialize stackgres client: %v", err))
	}

	vList, err := stackgresClient.GetAvailableVersions()
	if err != nil {
		log.Error(err, "cannot get postgres available versions")
		return runtime.NewWarningResult(fmt.Sprintf("cannot get postgres version list: %v", err))
	}

	majorMinorVersion, err := stackgres.GetLatestMinorVersion(expectedV, vList)
	if err != nil {
		log.Error(err, "cannot get the latest minor postgres version")
		return runtime.NewWarningResult(fmt.Sprintf("cannot get latest minor version: %v", err))
	}

	sgdbops := &stackgresv1.SGDbOps{
		ObjectMeta: v1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", comp.GetName(), majorUpgradeSuffix),
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: stackgresv1.SGDbOpsSpec{
			MajorVersionUpgrade: &stackgresv1.SGDbOpsSpecMajorVersionUpgrade{
				Link:             pointer.To(true),
				PostgresVersion:  &majorMinorVersion,
				SgPostgresConfig: pointer.To(fmt.Sprintf("%s-postgres-config-%s", comp.GetName(), expectedV)),
			},
			Op:        "majorVersionUpgrade",
			SgCluster: cluster.GetName(),
		},
	}

	err = svc.SetDesiredKubeObject(sgdbops, fmt.Sprintf("%s-%s", comp.GetName(), majorUpgradeSuffix), runtime.KubeOptionAllowDeletion)
	if err != nil {
		log.Error(err, "cannot set desired major postgres version sgdbops resource")
		return runtime.NewWarningResult(fmt.Sprintf("cannot create major upgrade kube object %s", comp.GetName()))
	}

	return runtime.NewNormalResult("SGDBOps for major upgrade created")
}
