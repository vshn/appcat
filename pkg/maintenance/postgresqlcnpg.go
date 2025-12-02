package maintenance

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/spf13/viper"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshnpostgrescnpg"
	"github.com/vshn/appcat/v4/pkg/maintenance/release"
	"gopkg.in/yaml.v2"

	"github.com/go-logr/logr"
	cnpgv1 "github.com/vshn/appcat/v4/apis/cnpg/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	catalogURL        string
	// repack          unavailable
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
		catalogURL:        viper.GetString("MAINTENANCE_URL"),
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

	version, err := strconv.Atoi(claim.Spec.Parameters.Service.MajorVersion)
	if err != nil {
		return fmt.Errorf("cannot parse postgresql major version: %w", err)
	}

	latestCatalog, err := p.getLatestImageCatalog(ctx, p.catalogURL)
	if err != nil {
		return fmt.Errorf("cannot get the latest image catalog: %w", err)
	}

	err = p.applyImageCatalog(ctx, latestCatalog, instanceCluster)
	if err != nil {
		return fmt.Errorf("cannot apply new image catalog: %w", err)
	}

	p.log.Info(fmt.Sprintf("Current image: %s", instanceCluster.Status.Image))
	for _, i := range latestCatalog.Spec.Images {
		if i.Major == version {
			p.log.Info(fmt.Sprintf("Latest image: %s", i.Image))
		}
	}

	// EOL handling

	if isEol := p.isEOL(version, latestCatalog); isEol {
		p.log.Info("Setting EOL on claim")
		if err := p.setEOLStatus(ctx); err != nil {
			return fmt.Errorf("couldn't set EOL status on claim: %w", err)
		}
	}

	// Vacuum
	if p.vacuum == "true" {
		p.log.Info("Starting vacuum")

		if instanceCluster.Status.ReadyInstances > 0 {
			err = p.doVacuum(ctx)
			if err != nil {
				return fmt.Errorf("vacuum failed: %v", err)
			}
		} else {
			p.log.Error(fmt.Errorf("the cluster is reporting no ready instances"), "vacuuming skipped")
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

// getLatestImageCatalog fetches the imagecatalog from the given maintenanceURL
func (p *PostgreSQLCNPG) getLatestImageCatalog(ctx context.Context, catalogURL string) (*cnpgv1.ImageCatalog, error) {
	imageCatalog := &cnpgv1.ImageCatalog{}

	resp, err := p.httpClient.Get(catalogURL)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch latest cnpg catalog: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cannot fetch latest cnpg catalog, got status code: %s", resp.Status)
	}

	bodyDecoder := yaml.NewDecoder(resp.Body)
	err = bodyDecoder.Decode(imageCatalog)
	if err != nil {
		return nil, fmt.Errorf("cannot decode image catalog: %w", err)
	}

	return imageCatalog, nil
}

func (p *PostgreSQLCNPG) applyImageCatalog(ctx context.Context, newCatalog *cnpgv1.ImageCatalog, instanceCluster *cnpgv1.Cluster) error {
	currentCatalog := &cnpgv1.ImageCatalog{}

	err := p.k8sClient.Get(ctx, client.ObjectKey{Name: instanceCluster.Spec.ImageCatalogRef.Name, Namespace: p.instanceNamespace}, currentCatalog)
	if err != nil {
		return fmt.Errorf("cannot get current catalog: %w", err)
	}

	currentCatalog.Spec.Images = newCatalog.Spec.Images

	return p.k8sClient.Update(ctx, currentCatalog)
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

func (p *PostgreSQLCNPG) isEOL(currentVersion int, imageCatalog *cnpgv1.ImageCatalog) bool {
	for _, i := range imageCatalog.Spec.Images {
		if i.Major == currentVersion {
			return false
		}
	}
	return true
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
