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
	corev1 "k8s.io/api/core/v1"
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
	compName          string
	vacuum            string
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
		compName:          viper.GetString("COMPOSITE_NAME"),
		vacuum:            viper.GetString("VACUUM_ENABLED"),
	}
}

// DoMaintenance will run postgresql's maintenance script.
func (p *PostgreSQLCNPG) DoMaintenance(ctx context.Context) error {
	p.log.Info("Starting maintenance on postgresql instance")

	claim := &vshnv1.VSHNPostgreSQL{}
	if err := p.k8sClient.Get(ctx, client.ObjectKey{Name: p.claimName, Namespace: p.claimNamespace}, claim); err != nil {
		return fmt.Errorf("couldn't get claim: %w", err)
	}

	instanceCluster, err := p.getCompositeCluster(ctx)
	if err != nil {
		return fmt.Errorf("couldn't get instance cluster: %w", err)
	}

	icName := instanceCluster.Spec.ImageCatalogRef.Name
	imageCatalog, err := p.getImageCatalog(ctx, icName)
	if err != nil {
		return fmt.Errorf("couldn't get image catalog '%s': %w", icName, err)
	}

	var versionList []string
	for _, image := range imageCatalog.Spec.Images {
		versionList = append(versionList, fmt.Sprintf("%d", image.Major))
	}

	// Update image catalog
	p.log.Info("Updating image catalog...", "imageCatalog", icName)
	if changed, err := p.setLatestVersionInCatalog(imageCatalog); err != nil {
		return fmt.Errorf("couldn't upgrade catalog: %w", err)
	} else if changed {
		if err := p.k8sClient.Update(ctx, imageCatalog); err != nil {
			return fmt.Errorf("could't update ImageCatalog in cluster: %w", err)
		}
	}

	// EOL
	if isEol := p.isEOL(claim.Spec.Parameters.Service.MajorVersion, versionList); isEol {
		p.log.Info("Setting EOL on calim")
		if err := p.setEOLStatus(ctx); err != nil {
			return fmt.Errorf("couldn't set EOL status on claim: %w", err)
		}
	}

	// Vacuum
	if p.vacuum == "true" {
		p.log.Info("Vacuuming databases...")

		if instanceCluster.Status.ReadyInstances == 0 {
			return fmt.Errorf("instance cluster reports no ready instances")
		}

		err = p.doVacuum(ctx)
		if err != nil {
			return fmt.Errorf("vacuum failed: %v", err)
		}
	}

	p.log.Info("Instance maintenance is done")
	return nil
}

func (p *PostgreSQLCNPG) doVacuum(ctx context.Context) error {
	uri, err := p.getConnectionUri(ctx)
	if err != nil {
		return fmt.Errorf("couldn't get psql connection uri: %w", err)
	}

	conn, err := pgx.Connect(context.Background(), uri)
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

		cfg, err := pgx.ParseConfig(uri)
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
		version, err := p.getLatestMinorVersion(thisVersion, versions)
		if err != nil {
			return false, fmt.Errorf("couldn't get latest minor version: %w", err)
		}

		p.log.Info("Comparing latest version vs this version", "thisVersion", thisVersion, "newVersion", version)
		if thisVersion != version {
			haveMadeChanges = true
			result := fmt.Sprintf("%s:%s", vshnpostgrescnpg.PsqlContainerRegistry, version)
			p.log.Info("Setting new version", "newVersion", version, "origImage", image.Image, "newImage", result, "major", image.Major)
			ic.Spec.Images[i].Image = result
		}
	}

	return haveMadeChanges, nil
}

// Get PSQL connection URI directly from the connection secret
func (p *PostgreSQLCNPG) getConnectionUri(ctx context.Context) (string, error) {
	secret := &corev1.Secret{}
	secretName := p.compName + "-connection"
	if err := p.k8sClient.Get(ctx, client.ObjectKey{
		Namespace: p.instanceNamespace,
		Name:      secretName,
	}, secret); err != nil {
		return "", fmt.Errorf("couldn't get secret '%s': %w", secretName, err)
	}

	uri := string(secret.Data[vshnpostgrescnpg.PostgresqlURL])
	if uri == "" {
		return "", fmt.Errorf("connection secret has empty '%s", vshnpostgrescnpg.PostgresqlURL)
	}

	if strings.HasSuffix(uri, "*") {
		db := string(secret.Data[vshnpostgrescnpg.PostgresqlDb])
		if db == "" {
			return "", fmt.Errorf("connection secret has empty '%s'", vshnpostgrescnpg.PostgresqlDb)
		}
		uri = fmt.Sprintf("%s%s", strings.TrimSuffix(uri, "*"), db)
	}

	return uri, nil
}

func (p *PostgreSQLCNPG) getImageCatalog(ctx context.Context, name string) (*cnpgv1.ImageCatalog, error) {
	imageCatalog := &cnpgv1.ImageCatalog{}
	err := p.k8sClient.Get(ctx, client.ObjectKey{Namespace: p.instanceNamespace, Name: name}, imageCatalog)
	if err != nil {
		return nil, err
	}

	return imageCatalog, nil
}

func (p *PostgreSQLCNPG) getCompositeCluster(ctx context.Context) (*cnpgv1.Cluster, error) {
	cluster := &cnpgv1.Cluster{}
	err := p.k8sClient.Get(ctx, client.ObjectKey{
		Namespace: p.instanceNamespace,
		Name:      p.compName + "-cluster",
	}, cluster)
	if err != nil {
		return nil, err
	}

	return cluster, nil
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
func (p *PostgreSQLCNPG) getLatestMinorVersion(vers string, versionList []string) (string, error) {
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
			continue
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
