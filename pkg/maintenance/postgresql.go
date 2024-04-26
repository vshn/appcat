package maintenance

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/watch"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-version"
	"github.com/spf13/viper"
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
	Client            client.WithWatch
	log               logr.Logger
	MaintTimeout      time.Duration
	instanceNamespace string
	ctx               context.Context
	apiUserName       string
	apiPassword       string
	claimNamespace    string
	claimName         string
	SgURL             string
	Repack, Vacuum    bool
}

type loginRequest struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type authToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type pgVersions struct {
	Postgresql []string `json:"postgresql"`
}

// DoMaintenance will run postgresql's maintenance script.
func (p *PostgreSQL) DoMaintenance(ctx context.Context) error {

	if err := p.configure(); err != nil {
		return err
	}
	p.ctx = ctx

	p.log = logr.FromContextOrDiscard(p.ctx).WithValues("instanceNamespace", p.instanceNamespace)

	p.log.Info("Starting maintenance on postgresql instance")

	sgclusters, err := p.listClustersInNamespace()
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
		err := p.createSecurityUpgrade(sgCluster.GetName())
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
	op, err := p.upgradeVersion(currentVersion, sgCluster.GetName())
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

	p.log.Info("Setting vacuum and repack variables")
	err = p.setVacuumRepack()
	if err != nil {
		return fmt.Errorf("cannot set vacuum and repack variables: %v", err)
	}

	if p.Vacuum {
		p.log.Info("Vacuuming databases...")
		err = p.createVacuum(sgCluster.GetName())
		if err != nil {
			return fmt.Errorf("cannot create vacuum: %v", err)
		}
	}

	if p.Repack {
		p.log.Info("Repacking databases...")
		err = p.createRepack(sgCluster.GetName())
		if err != nil {
			return fmt.Errorf("cannot create repack: %v", err)
		}
	}
	// default to repack if for some reason it was possible to disable both vacuum and repack
	if !p.Vacuum && !p.Repack {
		err = p.createRepack(sgCluster.GetName())
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

func (p *PostgreSQL) upgradeVersion(currentVersion string, sgClusterName string) (OpName, error) {
	versionList, err := p.fetchVersionList(p.SgURL)
	if err != nil {
		p.log.Error(err, "StackGres API error")
		p.log.Info("Can't get latest minor version, proceeding with security maintenance")
	}

	// if there are any errors here, we fall back to a security upgrade
	latestMinor, err := p.getLatestMinorversion(currentVersion, versionList)
	if err != nil {
		p.log.Error(err, "Could not get latest minor version from list, continuing with security upgrade")
		currentVersion = latestMinor
	}

	p.log.Info("Found versions", "current", currentVersion, "latest", latestMinor)
	if currentVersion != latestMinor {
		p.log.Info("Doing a minor upgrade")
		return mvu, p.createMinorUpgrade(sgClusterName, latestMinor)
	}

	p.log.Info("Checking for EOL")
	if versionList != nil && p.isEOL(currentVersion, *versionList) {
		err = p.setEOLStatus()
		if err != nil {
			return "", fmt.Errorf("cannot set EOL status on claim: %w", err)
		}
	}

	p.log.Info("Doing a security maintenance")
	return su, p.createSecurityUpgrade(sgClusterName)
}

func (p *PostgreSQL) waitForUpgrade(ctx context.Context, op OpName) error {
	ls := &stackgresv1.SGDbOpsList{}
	watcher, err := p.Client.Watch(ctx, ls, client.InNamespace(p.instanceNamespace))
	if err != nil {
		return fmt.Errorf("watch error:%v for sgdbops resources in %s", err, p.instanceNamespace)
	}
	defer watcher.Stop()
	rc := watcher.ResultChan()
	timer := time.NewTimer(p.MaintTimeout)
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

func (p *PostgreSQL) listClustersInNamespace() (*stackgresv1.SGClusterList, error) {
	sgClusters := &stackgresv1.SGClusterList{}

	err := p.Client.List(p.ctx, sgClusters, &client.ListOptions{Namespace: p.instanceNamespace})
	if err != nil {
		return nil, err
	}

	return sgClusters, nil

}

func (p *PostgreSQL) getLatestMinorversion(vers string, versionList *pgVersions) (string, error) {

	if versionList == nil {
		return vers, nil
	}

	p.log.Info("Searching most current minor version")
	current, err := version.NewVersion(vers)
	if err != nil {
		return "", err
	}

	validVersions := make([]*version.Version, 0)
	for _, newVersion := range versionList.Postgresql {
		tmpVersion, err := version.NewVersion(newVersion)
		if err != nil {
			return "", err
		}
		if tmpVersion.Segments()[0] == current.Segments()[0] {
			validVersions = append(validVersions, tmpVersion)
		}
	}

	sort.Sort(sort.Reverse(version.Collection(validVersions)))

	if len(validVersions) != 0 && current.LessThan(validVersions[0]) {
		return validVersions[0].Original(), nil
	}

	return current.Original(), nil
}

func (p *PostgreSQL) fetchVersionList(url string) (*pgVersions, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	httpClient := &http.Client{Transport: transport}

	auth := loginRequest{Username: p.apiUserName, Password: p.apiPassword}

	p.log.V(1).Info("Building login json")
	byteAuth, err := json.Marshal(auth)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal login json: %w", err)
	}

	p.log.V(1).Info("Logging into stackgres")
	resp, err := httpClient.Post(url+"/stackgres/auth/login", "application/json", bytes.NewBuffer(byteAuth))
	if err != nil {
		return nil, fmt.Errorf("cannot login: %w", err)
	}

	token := &authToken{}
	err = json.NewDecoder(resp.Body).Decode(token)
	if err != nil {
		return nil, fmt.Errorf("cannot decode login token: %w", err)
	}

	p.log.V(1).Info("Getting list of versions")
	req, err := http.NewRequest("GET", url+"/stackgres/version/postgresql", nil)
	if err != nil {
		return nil, fmt.Errorf("cannot get list of versions: %w", err)
	}
	req.Header.Add("Authorization", "Bearer "+token.AccessToken)

	resp, err = httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error during http request: %w", err)
	}
	versionList := &pgVersions{}

	err = json.NewDecoder(resp.Body).Decode(versionList)
	if err != nil {
		return nil, fmt.Errorf("error during json decoding: %w", err)
	}
	return versionList, nil
}

func (p *PostgreSQL) createRepack(clusterName string) error {
	repack := p.getDbOpsObject(clusterName, "databasesrepack", r)

	return p.applyDbOps(repack)
}

func (p *PostgreSQL) createVacuum(clusterName string) error {
	vacuum := p.getDbOpsObject(clusterName, "vacuum", v)

	return p.applyDbOps(vacuum)
}

func (p *PostgreSQL) createMinorUpgrade(clusterName, minorVersion string) error {
	minorMaint := p.getDbOpsObject(clusterName, "minorupgrade", mvu)
	minorMaint.Spec.MinorVersionUpgrade = &stackgresv1.SGDbOpsSpecMinorVersionUpgrade{
		Method:          pointer.String("InPlace"),
		PostgresVersion: &minorVersion,
	}
	return p.applyDbOps(minorMaint)
}

func (p *PostgreSQL) createSecurityUpgrade(clusterName string) error {
	secMaint := p.getDbOpsObject(clusterName, "securitymaintenance", su)
	secMaint.Spec.SecurityUpgrade = &stackgresv1.SGDbOpsSpecSecurityUpgrade{
		Method: pointer.String("InPlace"),
	}
	return p.applyDbOps(secMaint)
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

func (p *PostgreSQL) applyDbOps(obj *stackgresv1.SGDbOps) error {
	err := p.Client.Delete(p.ctx, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return p.Client.Create(p.ctx, obj)
}

func (p *PostgreSQL) configure() error {

	errString := "missing environment variable: %s"

	p.instanceNamespace = viper.GetString("INSTANCE_NAMESPACE")
	if p.instanceNamespace == "" {
		return fmt.Errorf(errString, "INSTANCE_NAMESPACE")
	}

	p.apiPassword = viper.GetString("API_PASSWORD")
	if p.apiPassword == "" {
		return fmt.Errorf(errString, "API_PASSWORD")
	}

	p.apiUserName = viper.GetString("API_USERNAME")
	if p.apiUserName == "" {
		return fmt.Errorf(errString, "API_USERNAME")
	}

	p.claimName = viper.GetString("CLAIM_NAME")
	if p.claimName == "" {
		return fmt.Errorf(errString, "CLAIM_NAME")
	}

	p.claimNamespace = viper.GetString("CLAIM_NAMESPACE")
	if p.claimNamespace == "" {
		return fmt.Errorf(errString, "CLAIM_NAMESPACE")
	}

	return nil
}

func (p *PostgreSQL) isEOL(currentVersion string, versionList pgVersions) bool {
	return !slices.Contains(versionList.Postgresql, currentVersion)
}

func (p *PostgreSQL) setEOLStatus() error {
	claim := &vshnv1.VSHNPostgreSQL{}

	err := p.Client.Get(p.ctx, client.ObjectKey{Name: p.claimName, Namespace: p.claimNamespace}, claim)
	if err != nil {
		return err
	}

	claim.Status.IsEOL = true

	return p.Client.Update(p.ctx, claim)
}

func (p *PostgreSQL) setVacuumRepack() error {
	claim := &vshnv1.VSHNPostgreSQL{}

	err := p.Client.Get(p.ctx, client.ObjectKey{Name: p.claimName, Namespace: p.claimNamespace}, claim)
	if err != nil {
		return err
	}

	p.Vacuum = claim.Spec.Parameters.Service.VacuumEnabled
	p.Repack = claim.Spec.Parameters.Service.RepackEnabled

	return nil
}
