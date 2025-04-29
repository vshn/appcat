package maintenance

import (
	"context"
	"net/http"

	"github.com/vshn/appcat/v4/pkg/maintenance/release"

	"github.com/go-logr/logr"
	"github.com/vshn/appcat/v4/pkg/maintenance/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	keycloakURL = "https://docker-registry.inventage.com:10121/v2/keycloak-competence-center/keycloak-managed/tags/list"
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

// DoMaintenance will run minios's maintenance script.
func (k *Keycloak) DoMaintenance(ctx context.Context) error {
	patcher := helm.NewImagePatcher(k.k8sClient, k.httpClient, k.log)
	valuesPath := helm.NewValuePath("image", "tag")
	return patcher.DoMaintenance(ctx, keycloakURL, valuesPath, helm.SemVerMinorAndPatches(true))
}
