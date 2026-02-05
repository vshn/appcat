package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	xpapi "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/thediveo/enumflag/v2"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/auth"
	"github.com/vshn/appcat/v4/pkg/auth/stackgres"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/maintenance"
	"github.com/vshn/appcat/v4/pkg/maintenance/release"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MaintenanceCMD specifies the cobra command for triggering the maintenance.
var (
	MaintenanceCMD = newMaintenanceCMD()
)

var (
	requiredEnvVars = []string{
		"INSTANCE_NAMESPACE",
		"CLAIM_NAME",
		"CLAIM_NAMESPACE",
		"OWNER_GROUP",
		"OWNER_KIND",
		"OWNER_VERSION",
		"SERVICE_ID",
	}
	requireAdditionalPosgresEnv = []string{
		"SG_NAMESPACE",
		"API_PASSWORD",
		"API_USERNAME",
		"VACUUM_ENABLED",
		"REPACK_ENABLED",
	}
)

type Maintenance interface {
	DoMaintenance(ctx context.Context) error
	ReleaseLatest(ctx context.Context, enabled bool, kubeClient client.Client, minAge time.Duration) error
	// RunBackup runs a pre-maintenance backup if supported
	RunBackup(ctx context.Context) error
}

type service enumflag.Flag

const (
	noDefault service = iota
	postgresql
	postgresqlCnpg
	redis
	minio
	mariadb
	keycloak
	nextcloud
	forgejo
)

var maintenanceServices = map[service][]string{
	postgresql:     {"postgresql"},
	postgresqlCnpg: {"postgresql_cnpg"},
	redis:          {"redis"},
	minio:          {"minio"},
	mariadb:        {"mariadb"},
	keycloak:       {"keycloak"},
	nextcloud:      {"nextcloud"},
	forgejo:        {"forgejo"},
}

var serviceName service

func newMaintenanceCMD() *cobra.Command {

	command := &cobra.Command{
		Use:   "maintenance",
		Short: "Maintenance runner",
		Long:  "Run maintenance for various services",
		RunE:  c.runMaintenance,
	}

	command.Flags().Var(
		enumflag.NewWithoutDefault(&serviceName, "service", maintenanceServices, enumflag.EnumCaseInsensitive),
		"service",
		"Specify the name of the service that should be maintained")
	err := command.MarkFlagRequired("service")
	if err != nil {
		panic(err)
	}

	return command
}

func (c *controller) runMaintenance(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	kubeClient, err := client.NewWithWatch(ctrl.GetConfigOrDie(), client.Options{
		Scheme: pkg.SetupScheme(),
	})
	if err != nil {
		return fmt.Errorf("failed to initialize kube client: %w", err)
	}

	maintClient, err := getMaintClient(ctx, kubeClient)
	if err != nil {
		return fmt.Errorf("cannot initialize control plane client: %w", err)
	}

	if err = validateMandatoryEnvs(serviceName); err != nil {
		return fmt.Errorf("missing required environment variables: %w", err)
	}

	log := logr.FromContextOrDiscard(ctx).WithValues(
		"instanceNamespace", viper.GetString("INSTANCE_NAMESPACE"),
		"claimName", viper.GetString("CLAIM_NAME"),
		"claimNamespace", viper.GetString("CLAIM_NAMESPACE"),
		"serviceId", viper.GetString("SERVICE_ID"),
	)

	vh := getVersionHandler(kubeClient, log)

	var m Maintenance
	switch serviceName {
	case postgresql:
		if m, err = initPostgreSQL(kubeClient, vh, log); err != nil {
			return fmt.Errorf("failed to initialize postgresql: %w", err)
		}
	case postgresqlCnpg:
		m = maintenance.NewPostgreSQLCNPG(kubeClient, getHTTPClient(), vh, log)

	case redis:
		m = maintenance.NewRedis(kubeClient, getHTTPClient(), vh, log)

	case minio:
		m = maintenance.NewMinio(kubeClient, getHTTPClient(), vh, log)

	case mariadb:
		m = maintenance.NewMariaDB(kubeClient, getHTTPClient(), vh, log)

	case keycloak:
		m = maintenance.NewKeycloak(kubeClient, getHTTPClient(), vh, log)

	case nextcloud:
		m = maintenance.NewNextcloud(kubeClient, getHTTPClient(), vh, log)

	case forgejo:
		m = maintenance.NewForgejo(kubeClient, getHTTPClient(), vh, log)
	default:

		panic("service name is mandatory")
	}

	pinImageTag := viper.GetString("PIN_IMAGE_TAG")
	disableAppcatRelease, err := strconv.ParseBool(viper.GetString("DISABLE_APPCAT_RELEASE"))
	if err != nil {
		return fmt.Errorf("cannot parse env variable DISABLE_APPCAT_RELEASE to bool: %w", err)
	}

	if disableAppcatRelease && pinImageTag != "" {
		log.Info("AppCat release disabled and image tag pinned, skipping...")
		return nil
	}

	if err = errors.Join(
		// Run backup before any changes, then release, then maintenance
		func() error {
			log.Info("Running pre-maintenance backup")
			if err := m.RunBackup(ctx); err != nil {
				log.Error(err, "Pre-maintenance backup failed")
				return fmt.Errorf("pre-maintenance backup failed: %w", err)
			}
			log.Info("Pre-maintenance backup completed successfully")
			return nil
		}(),
		func() error {
			if disableAppcatRelease {
				log.Info("AppCat release updates disabled by user configuration")
				return nil
			}

			enabled, err := strconv.ParseBool(viper.GetString("RELEASE_MANAGEMENT_ENABLED"))
			if err != nil {
				return fmt.Errorf("cannot determine if release management is enabled: %w", err)
			}
			if !enabled {
				log.Info("release management disabled, skipping rollout of latest revisions")
			}

			// Parse minimum revision age from component, default to 7 days
			minAge := 7 * 24 * time.Hour
			if ageStr := viper.GetString("MINIMUM_REVISION_AGE"); ageStr != "" {
				parsedAge, err := time.ParseDuration(ageStr)
				if err != nil {
					return fmt.Errorf("cannot parse MINIMUM_REVISION_AGE: %w", err)
				}
				minAge = parsedAge
			}

			return m.ReleaseLatest(ctx, enabled, maintClient, minAge)
		}(),
		func() error {
			if pinImageTag != "" {
				log.Info("Image tag pinned by user configuration, skipping service maintenance", "pinnedTag", pinImageTag)
				return nil
			}
			return m.DoMaintenance(ctx)
		}(),
	); err != nil {
		return fmt.Errorf("maintenance failed: %w", err)
	}

	return nil
}

func initPostgreSQL(kubeClient client.WithWatch, vh release.VersionHandler, log logr.Logger) (Maintenance, error) {
	sClient, err := getStackgresClient()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize stackgres client: %w", err)
	}
	return maintenance.NewPostgreSQL(kubeClient, sClient, vh, log), nil
}

func getHTTPClient() *http.Client {
	if viper.GetString("REGISTRY_USERNAME") == "" || viper.GetString("REGISTRY_PASSWORD") == "" {
		return &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	return auth.GetAuthHTTPClient(viper.GetString("REGISTRY_USERNAME"), viper.GetString("REGISTRY_PASSWORD"))
}

func validateMandatoryEnvs(s service) error {
	for _, envVar := range requiredEnvVars {
		if !viper.IsSet(envVar) {
			return fmt.Errorf("required environment variable %s not set", envVar)
		}
	}
	if s == postgresql {
		for _, envVar := range requireAdditionalPosgresEnv {
			if !viper.IsSet(envVar) {
				return fmt.Errorf("required environment variable %s not set", envVar)
			}
		}
	}
	return nil
}

func getVersionHandler(k8sCLient client.Client, log logr.Logger) release.VersionHandler {

	opts := release.ReleaserOpts{
		ClaimName:      viper.GetString("CLAIM_NAME"),
		ClaimNamespace: viper.GetString("CLAIM_NAMESPACE"),
		Composite:      viper.GetString("COMPOSITE_NAME"),
		Group:          viper.GetString("OWNER_GROUP"),
		Kind:           viper.GetString("OWNER_KIND"),
		Version:        viper.GetString("OWNER_VERSION"),
		ServiceID:      viper.GetString("SERVICE_ID"),
	}

	return release.NewDefaultVersionHandler(
		log,
		opts,
	)
}

func getStackgresClient() (*stackgres.StackgresClient, error) {
	sClient, err := stackgres.New(viper.GetString("API_USERNAME"), viper.GetString("API_PASSWORD"), viper.GetString("SG_NAMESPACE"))
	if err != nil {
		return nil, fmt.Errorf("cannot initilize stackgres http client: %w", err)
	}
	return sClient, nil
}

func getControlPlaneKubeConfig(ctx context.Context, kubeClient client.Client) (client.WithWatch, error) {

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "controlclustercredentials",
			Namespace: "syn-appcat",
		},
	}

	err := kubeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)
	if err != nil {
		return nil, err
	}

	config, err := clientcmd.RESTConfigFromKubeConfig(secret.Data["config"])
	if err != nil {
		return nil, err
	}

	maintClient, err := client.NewWithWatch(config, client.Options{
		Scheme: pkg.SetupScheme(),
	})

	return maintClient, err
}

// This function checks, if compositions are available on the current cluster.
// If not it will attempt to create a client pointing to the control plane.
func getMaintClient(ctx context.Context, kubeClient client.Client) (client.Client, error) {
	maintClient := kubeClient
	var err error
	if !utils.IsKindAvailable(xpapi.SchemeBuilder.GroupVersion, "Composition", ctrl.GetConfigOrDie()) {
		maintClient, err = getControlPlaneKubeConfig(ctx, kubeClient)
		if err != nil {
			return nil, fmt.Errorf("cannot get config for the control plane api: %w", err)
		}
	}
	return maintClient, nil
}
