package maintenance

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/vshn/appcat/v4/pkg/maintenance/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	nextcloudURL = "https://hub.docker.com/v2/namespaces/library/repositories/nextcloud/tags?page_size=100"
)

// Nextcloud contains all necessary dependencies to successfully run a Nextcloud maintenance
type Nextcloud struct {
	k8sClient  client.Client
	httpClient *http.Client
	log        logr.Logger
}

// NewNextcloud returns a new Nextcloud object
func NewNextcloud(c client.Client, hc *http.Client) *Nextcloud {
	return &Nextcloud{
		k8sClient:  c,
		httpClient: hc,
	}
}

// DoMaintenance will run minios's maintenance script.
func (m *Nextcloud) DoMaintenance(ctx context.Context) error {
	m.log = logr.FromContextOrDiscard(ctx).WithValues("type", "nextcloud")
	patcher := helm.NewImagePatcher(m.k8sClient, m.httpClient, m.log)

	valuesPath := helm.NewValuePath("image", "tag")

	return patcher.DoMaintenance(ctx, nextcloudURL, valuesPath, helm.SemVerPatchesOnly(true))
}
