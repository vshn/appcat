package maintenance

import (
	"context"
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
func (m *Keycloak) DoMaintenance(ctx context.Context) error {
	m.log = logr.FromContextOrDiscard(ctx).WithValues("type", "keycloak")
	patcher := helm.NewImagePatcher(m.k8sClient, m.httpClient, m.log)

	valuesPath := helm.NewValuePath("image", "tag")

	return patcher.DoMaintenance(ctx, keycloakURL, valuesPath, helm.SemVerPatchesOnly(true))
}
