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

// Skip pre-maintenance backup for keycloak
func (k *Keycloak) RunBackup(ctx context.Context) error {
	k.log.Info("Pre-maintenance backup disabled")
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
