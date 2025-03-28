package maintenance

import (
	"context"
	"fmt"
	"github.com/spf13/viper"
	"github.com/vshn/appcat/v4/pkg/auth/stackgres"
	"github.com/vshn/appcat/v4/pkg/maintenance/release"
	"time"

	"k8s.io/apimachinery/pkg/watch"

	"github.com/go-logr/logr"
	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OpName is the type of operation for SGDbOps resource
type OpName string

var (
	su  OpName = "securityUpgrade"
	mvu OpName = "minorVersionUpgrade"
	r   OpName = "repack"
	v   OpName = "vacuum"
)

// PostgreSQL handles the maintenance of postgresql services
type PostgreSQL struct {
	k8sClient client.WithWatch
	sClient   *stackgres.StackgresClient
	log       logr.Logger
	timeout   time.Duration
	release.VersionHandler
	instanceNamespace string
	apiUserName       string
	apiPassword       string
	sgNamespace       string
	claimNamespace    string
	claimName         string
	repack            string
	vacuum            string
}

// NewPostgreSQL returns a new PostgreSQL maintenance job runner
func NewPostgreSQL(c client.WithWatch, sClient *stackgres.StackgresClient, versionHandler release.VersionHandler, log logr.Logger) *PostgreSQL {
	return &PostgreSQL{
		k8sClient:         c,
		sClient:           sClient,
		timeout:           time.Hour,
		log:               log,
		VersionHandler:    versionHandler,
		instanceNamespace: viper.GetString("INSTANCE_NAMESPACE"),
		apiUserName:       viper.GetString("API_USERNAME"),
		apiPassword:       viper.GetString("API_PASSWORD"),
		sgNamespace:       viper.GetString("SG_NAMESPACE"),
		claimNamespace:    viper.GetString("CLAIM_NAMESPACE"),
		claimName:         viper.GetString("CLAIM_NAME"),
		repack:            viper.GetString("REPACK_ENABLED"),
		vacuum:            viper.GetString("VACUUM_ENABLED"),
	}
}

// DoMaintenance will run postgresql's maintenance script.
func (p *PostgreSQL) DoMaintenance(ctx context.Context) error {

	p.log.Info("Starting maintenance on postgresql instance")

	sgclusters, err := p.listClustersInNamespace(ctx)
	if err != nil {
		return err
	}

	if len(sgclusters.Items) == 0 {
		p.log.Info("No sgcluster found in namespace, skipping maintenance")
	}

	sgCluster := sgclusters.Items[0]
	currentVersion := sgclusters.Items[0].Spec.Postgres.Version

	p.log.Info("Checking for pending upgrades...")
	upgradeRequired := p.checkRequiredUpgrade(sgCluster)

	if upgradeRequired {
		p.log.Info("Doing a security maintenance")
		err := p.createSecurityUpgrade(ctx, sgCluster.GetName())
		if err != nil {
			return err
		}
		p.log.Info("Waiting for security maintenance to finish before checking for minor upgrades")
		err = p.waitForUpgrade(ctx, su)
		if apierrors.IsTimeout(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("cannot watch for maintenance sgdbops resources: %v", err)
		}
	}

	p.log.Info("Checking for upgrades...")
	op, err := p.upgradeVersion(ctx, currentVersion, sgCluster.GetName())
	if err != nil {
		return err
	}

	p.log.Info("Waiting for upgrades to finish before doing repack on databases")
	err = p.waitForUpgrade(ctx, op)
	if apierrors.IsTimeout(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot watch for maintenance sgdbops resources: %v", err)
	}

	if p.vacuum == "true" {
		p.log.Info("Vacuuming databases...")
		err = p.createVacuum(ctx, sgCluster.GetName())
		if err != nil {
			return fmt.Errorf("cannot create vacuum: %v", err)
		}
	}

	if p.repack == "true" {
		p.log.Info("Repacking databases...")
		err = p.createRepack(ctx, sgCluster.GetName())
		if err != nil {
			return fmt.Errorf("cannot create repack: %v", err)
		}
	}
	// default to repack if for some reason it was possible to disable both vacuum and repack
	// this should be catched by webhook
	if p.vacuum != "true" && p.repack != "true" {
		err = p.createRepack(ctx, sgCluster.GetName())
		if err != nil {
			return fmt.Errorf("cannot create repack: %v", err)
		}
	}

	return nil
}

func (p *PostgreSQL) checkRequiredUpgrade(sgCluster stackgresv1.SGCluster) bool {
	if sgCluster.Status.Conditions == nil {
		return false
	}
	sgClusterConditions := *sgCluster.Status.Conditions
	for _, condition := range sgClusterConditions {
		if *condition.Reason == "ClusterRequiresUpgrade" && *condition.Status == "True" {
			p.log.Info("Restart required ...")
			return true
		}
	}
	return false
}

func (p *PostgreSQL) upgradeVersion(ctx context.Context, currentVersion string, sgClusterName string) (OpName, error) {
	versionList, err := p.fetchVersionList()
	if err != nil {
		p.log.Error(err, "StackGres API error")
		p.log.Info("Can't get latest minor version, proceeding with security maintenance")
	}

	// if there are any errors here, we fall back to a security upgrade
	latestMinor, err := stackgres.GetLatestMinorVersion(currentVersion, versionList)
	if err != nil {
		p.log.Error(err, "Could not get latest minor version from list, continuing with security upgrade")
		currentVersion = latestMinor
	}

	p.log.Info("Found versions", "current", currentVersion, "latest", latestMinor)
	if currentVersion != latestMinor {
		p.log.Info("Doing a minor upgrade")
		return mvu, p.createMinorUpgrade(ctx, sgClusterName, latestMinor)
	}

	p.log.Info("Checking for EOL")
	if versionList != nil && p.isEOL(currentVersion, versionList) {
		err = p.setEOLStatus(ctx)
		if err != nil {
			return "", fmt.Errorf("cannot set EOL status on claim: %w", err)
		}
	}

	p.log.Info("Doing a security maintenance")
	return su, p.createSecurityUpgrade(ctx, sgClusterName)
}

func (p *PostgreSQL) waitForUpgrade(ctx context.Context, op OpName) error {
	ls := &stackgresv1.SGDbOpsList{}
	watcher, err := p.k8sClient.Watch(ctx, ls, client.InNamespace(p.instanceNamespace))
	if err != nil {
		return fmt.Errorf("watch error:%v for sgdbops resources in %s", err, p.instanceNamespace)
	}
	defer watcher.Stop()
	rc := watcher.ResultChan()
	timer := time.NewTimer(p.timeout)
	for {
		select {

		// Timout in case the job runs too long
		case <-timer.C:
			p.log.Info("Timeout waiting for upgrade to finish")
			return apierrors.NewTimeoutError("job timeout", -1)

		// Read updates for SGDbOps resources
		case event, ok := <-rc:
			if !ok {
				return fmt.Errorf("sgdbops resource watch channel had been closed")
			}

			switch event.Type {
			case watch.Modified:
				ops, _ := event.Object.(*stackgresv1.SGDbOps)
				if ops.Status.Conditions == nil || ops.Spec.Op != string(op) {
					continue
				}
				for _, c := range *ops.Status.Conditions {
					// When operation is completed then return, regardless if it failed or not
					if isUpgradeFinished(c) {
						return nil
					}
				}
			case watch.Error:
				return fmt.Errorf("wait for maintenance received watch error: %v", event)
			}
		}
	}
}

func isUpgradeFinished(v stackgresv1.SGDbOpsStatusConditionsItem) bool {
	return *v.Reason == "OperationCompleted" && *v.Status == "True"
}

func (p *PostgreSQL) listClustersInNamespace(ctx context.Context) (*stackgresv1.SGClusterList, error) {
	sgClusters := &stackgresv1.SGClusterList{}

	err := p.k8sClient.List(ctx, sgClusters, &client.ListOptions{Namespace: p.instanceNamespace})
	if err != nil {
		return nil, err
	}

	return sgClusters, nil

}

func (p *PostgreSQL) fetchVersionList() (*stackgres.PgVersions, error) {
	return p.sClient.GetAvailableVersions()
}

func (p *PostgreSQL) createRepack(ctx context.Context, clusterName string) error {
	repack := p.getDbOpsObject(clusterName, "databasesrepack", r)

	return p.applyDbOps(ctx, repack)
}

func (p *PostgreSQL) createVacuum(ctx context.Context, clusterName string) error {
	vacuum := p.getDbOpsObject(clusterName, "vacuum", v)

	return p.applyDbOps(ctx, vacuum)
}

func (p *PostgreSQL) createMinorUpgrade(ctx context.Context, clusterName, minorVersion string) error {
	minorMaint := p.getDbOpsObject(clusterName, "minorupgrade", mvu)
	minorMaint.Spec.MinorVersionUpgrade = &stackgresv1.SGDbOpsSpecMinorVersionUpgrade{
		Method:          pointer.String("InPlace"),
		PostgresVersion: &minorVersion,
	}
	return p.applyDbOps(ctx, minorMaint)
}

func (p *PostgreSQL) createSecurityUpgrade(ctx context.Context, clusterName string) error {
	secMaint := p.getDbOpsObject(clusterName, "securitymaintenance", su)
	secMaint.Spec.SecurityUpgrade = &stackgresv1.SGDbOpsSpecSecurityUpgrade{
		Method: pointer.String("InPlace"),
	}
	return p.applyDbOps(ctx, secMaint)
}

func (p *PostgreSQL) getDbOpsObject(clusterName, objectName string, op OpName) *stackgresv1.SGDbOps {
	return &stackgresv1.SGDbOps{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectName,
			Namespace: p.instanceNamespace,
		},
		Spec: stackgresv1.SGDbOpsSpec{
			SgCluster:  clusterName,
			Op:         string(op),
			MaxRetries: pointer.Int(1),
		},
	}

}

func (p *PostgreSQL) applyDbOps(ctx context.Context, obj *stackgresv1.SGDbOps) error {
	err := p.k8sClient.Delete(ctx, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return p.k8sClient.Create(ctx, obj)
}

func (p *PostgreSQL) isEOL(currentVersion string, versionList *stackgres.PgVersions) bool {
	return !slices.Contains(versionList.Postgresql, currentVersion)
}

func (p *PostgreSQL) setEOLStatus(ctx context.Context) error {
	claim := &vshnv1.VSHNPostgreSQL{}

	err := p.k8sClient.Get(ctx, client.ObjectKey{Name: p.claimName, Namespace: p.claimNamespace}, claim)
	if err != nil {
		return err
	}

	claim.Status.IsEOL = true

	return p.k8sClient.Update(ctx, claim)
}
