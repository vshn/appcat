package maintenance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/jackc/pgx/v5"
	"github.com/spf13/viper"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshnpostgrescnpg"
	"github.com/vshn/appcat/v4/pkg/maintenance/helm"
	"github.com/vshn/appcat/v4/pkg/maintenance/release"

	"github.com/go-logr/logr"
	cnpgv1 "github.com/vshn/appcat/v4/apis/cnpg/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	apiUrl = "https://ghcr.io/v2/cloudnative-pg/postgresql/tags/list"
)

// PostgreSQLCNPG handles the maintenance of postgresql services
type PostgreSQLCNPG struct {
	k8sClient  client.WithWatch
	httpClient *http.Client
	log        logr.Logger
	timeout    time.Duration
	release.VersionHandler
	instanceNamespace string
	claimNamespace    string
	claimName         string
	vacuum            string
	psqlUri           string
	//repack          unavailable
}

// NewPostgreSQL returns a new PostgreSQL maintenance job runner
func NewPostgreSQLCNPG(c client.WithWatch, hc *http.Client, versionHandler release.VersionHandler, log logr.Logger) *PostgreSQLCNPG {
	return &PostgreSQLCNPG{
		k8sClient:         c,
		httpClient:        hc,
		timeout:           time.Hour,
		log:               log,
		VersionHandler:    versionHandler,
		instanceNamespace: viper.GetString("INSTANCE_NAMESPACE"),
		claimNamespace:    viper.GetString("CLAIM_NAMESPACE"),
		claimName:         viper.GetString("CLAIM_NAME"),
		vacuum:            viper.GetString("VACUUM_ENABLED"),
		psqlUri:           viper.GetString("PSQL_URI"),
	}
}

// DoMaintenance will run postgresql's maintenance script.
func (p *PostgreSQLCNPG) DoMaintenance(ctx context.Context) error {
	p.log.Info("Starting maintenance on postgresql instance")

	imageCatalogs, err := p.listImageCatalogsInNamespace(ctx)
	if err != nil {
		return err
	}
	if len(imageCatalogs.Items) == 0 {
		p.log.Info("No image catalogs found in namespace, skipping maintenance")
		return nil
	}

	clusters, err := p.listClustersInNamespace(ctx)
	if err != nil {
		return err
	}
	if len(clusters.Items) == 0 {
		p.log.Info("No clusters found in namespace, skipping maintenance")
		return nil
	}

	claim := &vshnv1.VSHNPostgreSQL{}
	if err = p.k8sClient.Get(ctx, client.ObjectKey{Name: p.claimName, Namespace: p.claimNamespace}, claim); err != nil {
		return fmt.Errorf("couldn't get claim: %w", err)
	}

	// Get specifically our cluster
	claimCluster := cnpgv1.Cluster{}
	for _, cluster := range clusters.Items {
		verdict := cluster.Name == claim.GetName()+"-cluster"
		p.log.Info("processing cluster", "clusterName", cluster.Name, "verdict", verdict)
		if verdict {
			claimCluster = cluster
			break
		}
	}

	// Uppdate image catalogs, but only the one in use by our cluster
	var versionList []string
	p.log.Info("Updating image catalogs...", "amount", len(imageCatalogs.Items))
	for _, imageCatalog := range imageCatalogs.Items {
		if claimCluster.Spec.ImageCatalogRef.Name != imageCatalog.Name {
			p.log.Info("Skipping this ImageCatalog (not in use by claim cluster)", "name", imageCatalog.Name)
			continue
		}

		p.log.Info("Upgrading ImageCatalog...", "imageCatalog", imageCatalog.Name)
		changed, err := p.setLatestVersionInCatalog(&imageCatalog)
		if err != nil {
			return fmt.Errorf("couldn't upgrade catalog: %w", err)
		}

		for _, image := range imageCatalog.Spec.Images {
			versionList = append(versionList, fmt.Sprintf("%d", image.Major))
		}

		if changed {
			if err := p.k8sClient.Update(ctx, &imageCatalog); err != nil {
				return fmt.Errorf("could't update ImageCatalog in cluster: %w", err)
			}
		}
	}

	p.log.Info("Determining EOL...")
	if isEol := p.isEOL(claim.Spec.Parameters.Service.MajorVersion, versionList); isEol {
		if err := p.setEOLStatus(ctx); err != nil {
			return fmt.Errorf("couldn't set EOL status on claim: %w", err)
		}
	}

	if p.vacuum == "true" {
		p.log.Info("Vacuuming databases...")
		err = p.doVacuum(ctx)
		if err != nil {
			return fmt.Errorf("vacuum failed: %v", err)
		}
	}

	return nil
}

func (p *PostgreSQLCNPG) doVacuum(ctx context.Context) error {
	if p.psqlUri == "" {
		return fmt.Errorf("no 'PSQL_URI' set")
	}

	conn, err := pgx.Connect(context.Background(), p.psqlUri)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, "SELECT datname FROM pg_database WHERE datistemplate = false AND datallowconn = true")
	if err != nil {
		return fmt.Errorf("failed to list databases: %w", err)
	}
	defer rows.Close()

	var dbs []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("failed to scan database name: %w", err)
		}
		dbs = append(dbs, name)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating database rows: %w", err)
	}

	for _, db := range dbs {
		p.log.Info("Vacuuming database...", "database", db)

		cfg, err := pgx.ParseConfig(p.psqlUri)
		if err != nil {
			return fmt.Errorf("failed to parse psql uri: %w", err)
		}
		cfg.Database = db

		dbConn, err := pgx.ConnectConfig(context.Background(), cfg)
		if err != nil {
			return fmt.Errorf("failed to connect to database %s: %w", db, err)
		}

		if _, err := dbConn.Exec(ctx, "VACUUM"); err != nil {
			_ = dbConn.Close(ctx)
			return fmt.Errorf("failed to execute VACUUM on %s: %w", db, err)
		}

		if err := dbConn.Close(ctx); err != nil {
			p.log.Info("warning: failed to close connection", "database", db, "err", err)
		}
	}

	return nil
}

// Upgrade every minor version in an image catalog. Return true if anything was updated.
func (p *PostgreSQLCNPG) setLatestVersionInCatalog(ic *cnpgv1.ImageCatalog) (bool, error) {
	v, err := p.getRegistryVersions(apiUrl)
	if err != nil {
		return false, fmt.Errorf("couldn't get latest versions from registry: %w", err)
	}

	var haveMadeChanges bool
	versions := v.GetVersions()
	for i, image := range ic.Spec.Images {
		thisVersion := strings.Split(image.Image, ":")[1]
		version, err := getLatestMinorVersion(thisVersion, versions)
		if err != nil {
			return false, fmt.Errorf("couldn't get latest minor version: %w", err)
		}

		if thisVersion != version {
			haveMadeChanges = true
			result := fmt.Sprintf("%s:%s", vshnpostgrescnpg.PsqlContainerRegistry, version)
			p.log.Info("Setting new version", "newVersion", version, "origImage", image.Image, "newImage", result, "major", image.Major)
			ic.Spec.Images[i].Image = result
		}
	}

	return haveMadeChanges, nil
}

func (p *PostgreSQLCNPG) listImageCatalogsInNamespace(ctx context.Context) (*cnpgv1.ImageCatalogList, error) {
	imageCatalogs := &cnpgv1.ImageCatalogList{}
	err := p.k8sClient.List(ctx, imageCatalogs, &client.ListOptions{Namespace: p.instanceNamespace})
	if err != nil {
		return nil, err
	}

	return imageCatalogs, nil
}

func (p *PostgreSQLCNPG) listClustersInNamespace(ctx context.Context) (*cnpgv1.ClusterList, error) {
	clusters := &cnpgv1.ClusterList{}
	err := p.k8sClient.List(ctx, clusters, &client.ListOptions{Namespace: p.instanceNamespace})
	if err != nil {
		return nil, err
	}

	return clusters, nil
}

func (p *PostgreSQLCNPG) isEOL(currentVersion string, versionList []string) bool {
	return !slices.Contains(versionList, currentVersion)
}

func (p *PostgreSQLCNPG) setEOLStatus(ctx context.Context) error {
	claim := &vshnv1.VSHNPostgreSQL{}

	err := p.k8sClient.Get(ctx, client.ObjectKey{Name: p.claimName, Namespace: p.claimNamespace}, claim)
	if err != nil {
		return err
	}

	claim.Status.IsEOL = true

	return p.k8sClient.Update(ctx, claim)
}

// getLatestMinorVersion determines the most current minor version
func getLatestMinorVersion(vers string, versionList []string) (string, error) {
	if len(versionList) == 0 {
		return vers, nil
	}

	current, err := version.NewVersion(vers)
	if err != nil {
		return "", err
	}

	validVersions := make([]*version.Version, 0)
	for _, newVersion := range versionList {
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

// Temp
func (p *PostgreSQLCNPG) getRegistryVersions(imageURL string) (helm.VersionLister, error) {
	results := &helm.RegistryResult{}

	token, err := getGhcrToken()
	if err != nil {
		return nil, err
	}

	req, _ := http.NewRequest("GET", imageURL, nil)
	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot access registry: %w", err)
	}

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry returned bad status code (%d): %s", resp.StatusCode, string(b))
	}

	err = json.NewDecoder(resp.Body).Decode(results)
	if err != nil {
		return nil, fmt.Errorf("cannot decode registry results: %w", err)
	}

	return results, nil
}

// Temp slop func
func getGhcrToken() (string, error) {
	registry := vshnpostgrescnpg.PsqlContainerRegistry
	parts := strings.SplitN(registry, "/", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid registry format: %s", registry)
	}

	host := parts[0]
	repo := parts[1]
	tokenURL := fmt.Sprintf("https://%s/token?scope=repository:%s:pull", host, repo)

	req, err := http.NewRequest("GET", tokenURL, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("cannot access token endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token endpoint returned status %d: %s", resp.StatusCode, string(b))
	}

	var tr struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", fmt.Errorf("cannot decode token response: %w", err)
	}
	if tr.Token == "" {
		return "", fmt.Errorf("empty token in response")
	}

	return tr.Token, nil
}
