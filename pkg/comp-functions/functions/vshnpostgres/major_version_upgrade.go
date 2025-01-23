package vshnpostgres

import (
	"context"
	"errors"
	"fmt"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/auth/stackgres"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	v2 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	pointer "k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	majorUpgradeSuffix = "major-upgrade-dbops"
)

func MajorVersionUpgrade(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	comp, err := getVSHNPostgreSQL(ctx, svc)

	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get composite from function io: %w", err))
	}

	expectedV := comp.Spec.Parameters.Service.MajorVersion
	currentV := comp.Status.CurrentVersion

	// If current and expected versions do not match then issue a major version upgrade via SGDBOps resource
	if currentV != "" && currentV != expectedV {
		err = upgradePGSettings(comp, svc)
		if err != nil {
			return runtime.NewWarningResult(fmt.Sprintf("cannot upgrade PG settings: %w", err))
		}
		majorUpgradeDbOps, err := getSgDbOpsIfExists(err, svc, comp)
		if err != nil {
			return runtime.NewWarningResult(fmt.Sprintf("cannot get major upgrade SgDbOps resource: %w", err))
		}

		// If SGDBOps resource does not exist create it and keep during upgrade otherwise cleanup if successful
		if majorUpgradeDbOps == nil {
			return createMajorUpgradeSgDbOps(ctx, svc, comp, expectedV)
		} else if isSuccessful(majorUpgradeDbOps.Status.Conditions) {
			updateStatusVersion(svc, comp, expectedV)
		}
		return keepSgDbOpsResource(svc, comp, majorUpgradeDbOps)
	}
	removePreviousVersionConfig(svc, comp)
	return nil
}

// getSgDbOpsIfExists returns a major upgrade SgDbOps resource if upgrade is in process
func getSgDbOpsIfExists(err error, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL) (*stackgresv1.SGDbOps, error) {
	majorUpgradeDbOps := &stackgresv1.SGDbOps{}
	err = svc.GetObservedKubeObject(majorUpgradeDbOps, fmt.Sprintf("%s-%s", comp.GetName(), majorUpgradeSuffix))
	if err != nil && !errors.Is(err, runtime.ErrNotFound) {
		return nil, fmt.Errorf("cannot get observed kube object major upgrade sgdbops: %w", err)
	}
	if errors.Is(err, runtime.ErrNotFound) {
		return nil, nil
	}
	return majorUpgradeDbOps, nil
}

// removePreviousVersionConfig removes previous version pg settings resource.
func removePreviousVersionConfig(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL) {
	svc.DeleteDesiredCompososedResource(fmt.Sprintf("%s-postgres-config-%s", comp.GetName(), comp.Status.PreviousVersion))
}

// upgradePGSettings creates a new SGPostgresConfig resource with the new expected major
func upgradePGSettings(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {
	conf := &stackgresv1.SGPostgresConfig{}
	err := svc.GetObservedKubeObject(conf, fmt.Sprintf("%s-%s-%s", comp.GetName(), configResourceName, comp.Status.CurrentVersion))
	if err != nil {
		return fmt.Errorf("cannot get observed kube object postgres config: %w", err)
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
		return fmt.Errorf("cannot create current version postgres config:  %w", err)
	}
	return nil
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

// createMajorUpgradeSgDbOps create a major upgrade SgDbOps resource to start the upgrade process
func createMajorUpgradeSgDbOps(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNPostgreSQL, expectedV string) *xfnproto.Result {
	cluster := &stackgresv1.SGCluster{}
	err := svc.GetObservedKubeObject(cluster, "cluster")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get observed kube object cluster: %w", err))
	}

	sgNamespace := svc.Config.Data["sgNamespace"]
	username, password, err := getCredentials(ctx, sgNamespace)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get stackgres rest api credentials: %w", err))
	}
	stackgresClient, err := stackgres.New(username, password, sgNamespace)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot initialize stackgres client: %w", err))
	}

	vList, err := stackgresClient.GetAvailableVersions()
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get postgres version list: %w", err))
	}

	majorMinorVersion, err := stackgres.GetLatestMinorVersion(expectedV, vList)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get latest minor version: %w", err))
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
		return runtime.NewWarningResult(fmt.Sprintf("cannot create major upgrade kube object %s", comp.GetName()))
	}

	return runtime.NewNormalResult("SGDBOps for major upgrade created")
}

func getCredentials(ctx context.Context, sgNamespace string) (username, password string, err error) {
	kubeClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: pkg.SetupScheme(),
	})
	if err != nil {
		return "", "", err
	}

	c := &v2.Secret{}
	err = kubeClient.Get(ctx, types.NamespacedName{
		Namespace: sgNamespace,
		Name:      "stackgres-restapi-admin",
	}, c)
	if err != nil {
		return "", "", err
	}

	return string(c.Data["k8sUsername"]), string(c.Data["clearPassword"]), nil
}
