package maintenance

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	stackgresv1 "github.com/vshn/appcat/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PostgreSQL handles the maintenance of postgresql services
type PostgreSQL struct {
	Client            client.Client
	log               logr.Logger
	instanceNamespace string
	ctx               context.Context
	apiUserName       string
	apiPassword       string
	sgNamespace       string
	claimNamespace    string
	claimName         string
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
func (p *PostgreSQL) DoMaintenance(cmd *cobra.Command) error {

	if err := p.configure(); err != nil {
		return err
	}

	p.log = logr.FromContextOrDiscard(cmd.Context()).WithValues("instanceNamespace", p.instanceNamespace)

	p.log.Info("Starting maintenance on postgresql instance")

	p.ctx = cmd.Context()

	sgclusters, err := p.listClustersInNamespace()
	if err != nil {
		return err
	}

	if len(sgclusters.Items) == 0 {
		p.log.Info("No sgcluster found in namespace, skipping maintenance")
	}

	clusterName := sgclusters.Items[0].GetName()
	currentVersion := sgclusters.Items[0].Spec.Postgres.Version

	versionList, err := p.fetchVersionList()
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
		return p.createMinorUpgrade(clusterName, latestMinor)
	}

	p.log.Info("Checking for EOL")
	if versionList != nil && p.isEOL(currentVersion, *versionList) {
		err := p.setEOLStatus()
		if err != nil {
			return fmt.Errorf("cannot set EOL status on claim: %w", err)
		}
	}

	p.log.Info("Doing a security maintenance")
	return p.createSecurityUpgrade(clusterName)
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

	if current.LessThan(validVersions[0]) {
		return validVersions[0].Original(), nil
	}

	return current.Original(), nil
}

func (p *PostgreSQL) fetchVersionList() (*pgVersions, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	httpClient := &http.Client{Transport: transport}

	auth := loginRequest{Username: p.apiUserName, Password: p.apiPassword}

	p.log.V(1).Info("Building login json")
	byteAuth, err := json.Marshal(auth)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal login json: %w", err)
	}

	sgURL := "https://stackgres-restapi." + p.sgNamespace + ".svc"

	p.log.V(1).Info("Logging into stackgres")
	resp, err := httpClient.Post(sgURL+"/stackgres/auth/login", "application/json", bytes.NewBuffer(byteAuth))
	if err != nil {
		return nil, fmt.Errorf("cannot login: %w", err)
	}

	token := &authToken{}
	err = json.NewDecoder(resp.Body).Decode(token)
	if err != nil {
		return nil, fmt.Errorf("cannot decode login token: %w", err)
	}

	p.log.V(1).Info("Getting list of versions")
	req, err := http.NewRequest("GET", sgURL+"/stackgres/version/postgresql", nil)
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

func (p *PostgreSQL) createMinorUpgrade(clusterName, minorVersion string) error {
	minorMaint := p.getDbOpsObject(clusterName, "minorupgrade", "minorVersionUpgrade")
	minorMaint.Spec.MinorVersionUpgrade = &stackgresv1.SGDbOpsSpecMinorVersionUpgrade{
		Method:          pointer.String("InPlace"),
		PostgresVersion: &minorVersion,
	}
	return p.applyDbOps(minorMaint)
}

func (p *PostgreSQL) createSecurityUpgrade(clusterName string) error {
	secMaint := p.getDbOpsObject(clusterName, "securitymaintenance", "securityUpgrade")
	secMaint.Spec.SecurityUpgrade = &stackgresv1.SGDbOpsSpecSecurityUpgrade{
		Method: pointer.String("InPlace"),
	}
	return p.applyDbOps(secMaint)
}

func (p *PostgreSQL) getDbOpsObject(clusterName, objectName, opName string) *stackgresv1.SGDbOps {
	return &stackgresv1.SGDbOps{
		ObjectMeta: metav1.ObjectMeta{
			Name:      objectName,
			Namespace: p.instanceNamespace,
		},
		Spec: stackgresv1.SGDbOpsSpec{
			SgCluster:  clusterName,
			Op:         opName,
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

	p.sgNamespace = viper.GetString("SG_NAMESPACE")
	if p.sgNamespace == "" {
		return fmt.Errorf(errString, "SG_NAMESPACE")
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
