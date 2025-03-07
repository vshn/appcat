package maintenance

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/vshn/appcat/v4/pkg/maintenance/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	forgejoURL = "https://codeberg.org/v2/forgejo/forgejo/tags/list"
)

// Forgejo contains all necessary dependencies to successfully run a Forgejo maintenance
type Forgejo struct {
	k8sClient  client.Client
	httpClient *http.Client
	log        logr.Logger
}

// NewForgejo returns a new Forgejo object
func NewForgejo(c client.Client, hc *http.Client) *Forgejo {
	return &Forgejo{
		k8sClient:  c,
		httpClient: hc,
	}
}

// DoMaintenance will run forgejo's maintenance script.
func (m *Forgejo) DoMaintenance(ctx context.Context) error {
	m.log = logr.FromContextOrDiscard(ctx).WithValues("type", "forgejo")
	patcher := helm.NewImagePatcher(m.k8sClient, m.httpClient, m.log)

	valuesPath := helm.NewValuePath("image", "tag")

	return patcher.DoMaintenance(ctx, forgejoURL, valuesPath, helm.SemVerPatchesOnly(true))
}
