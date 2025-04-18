package maintenance

import (
	"context"
	"github.com/vshn/appcat/v4/pkg/maintenance/release"
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
	release.VersionHandler
}

// NewForgejo returns a new Forgejo object
func NewForgejo(c client.Client, hc *http.Client, vh release.VersionHandler, logger logr.Logger) *Forgejo {
	return &Forgejo{
		k8sClient:      c,
		httpClient:     hc,
		log:            logger,
		VersionHandler: vh,
	}
}

// DoMaintenance will run forgejo's maintenance script.
func (f *Forgejo) DoMaintenance(ctx context.Context) error {
	patcher := helm.NewImagePatcher(f.k8sClient, f.httpClient, f.log)
	valuesPath := helm.NewValuePath("image", "tag")
	return patcher.DoMaintenance(ctx, forgejoURL, valuesPath, helm.SemVerPatchesOnly(true))
}
