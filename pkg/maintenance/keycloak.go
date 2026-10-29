package maintenance

import (
	"context"
	"net/http"

	"github.com/vshn/appcat/v4/pkg/maintenance/release"

	"github.com/go-logr/logr"
	"github.com/vshn/appcat/v4/pkg/maintenance/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Keycloak contains all necessary dependencies to successfully run a Keycloak maintenance
type Keycloak struct {
	k8sClient  client.Client
	httpClient *http.Client
	log        logr.Logger
	release.VersionHandler
}

// NewKeycloak returns a new Keycloak object
func NewKeycloak(c client.Client, hc *http.Client, vh release.VersionHandler, logger logr.Logger) *Keycloak {
	return &Keycloak{
		k8sClient:      c,
		httpClient:     hc,
		log:            logger,
		VersionHandler: vh,
	}
}

// Keycloak has no backups of its own, the PostgreSQL instance waits for its backups in its own maintenance
func (k *Keycloak) WaitForRunningBackups(ctx context.Context) error {
	k.log.Info("Keycloak has no backups, not waiting for running backups")
	return nil
}

// DoMaintenance will run Keycloak's maintenance script.
func (k *Keycloak) DoMaintenance(ctx context.Context) error {
	maintenanceURL, err := getMaintenanceURL()
	if err != nil {
		return err
	}

	patcher := helm.NewImagePatcher(k.k8sClient, k.httpClient, k.log)
	valuesPath := helm.NewValuePath("image", "tag")
	return patcher.DoMaintenance(ctx, maintenanceURL, valuesPath, helm.SemVerMinorAndPatches(true))
}
