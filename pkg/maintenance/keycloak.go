package maintenance

import (
	"context"
	"fmt"
	"github.com/vshn/appcat/v4/pkg/maintenance/release"
	"net/http"

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
}

// NewKeycloak returns a new Keycloak object
func NewKeycloak(c client.Client, hc *http.Client) *Keycloak {
	return &Keycloak{
		k8sClient:  c,
		httpClient: hc,
	}
}

// DoMaintenance will run minios's maintenance script.
func (k *Keycloak) DoMaintenance(ctx context.Context) error {
	k.log = logr.FromContextOrDiscard(ctx).WithValues("type", "keycloak")
	patcher := helm.NewImagePatcher(k.k8sClient, k.httpClient, k.log)

	valuesPath := helm.NewValuePath("image", "tag")

	return patcher.DoMaintenance(ctx, keycloakURL, valuesPath, helm.SemVerPatchesOnly(true))
}

func (k *Keycloak) ReleaseLatestAppCatVersion(ctx context.Context) error {
	vh, err := release.NewDefaultVersionHandler(k.k8sClient)
	if err != nil {
		return fmt.Errorf("could not initialize default version handler: %w", err)
	}
	return vh.LatestVersion(ctx)
}
