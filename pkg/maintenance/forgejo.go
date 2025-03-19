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
func (f *Forgejo) DoMaintenance(ctx context.Context) error {
	f.log = logr.FromContextOrDiscard(ctx).WithValues("type", "forgejo")
	patcher := helm.NewImagePatcher(f.k8sClient, f.httpClient, f.log)

	valuesPath := helm.NewValuePath("image", "tag")

	return patcher.DoMaintenance(ctx, forgejoURL, valuesPath, helm.SemVerPatchesOnly(true))
}

func (f *Forgejo) ReleaseLatestAppCatVersion(ctx context.Context) error {
	vh, err := release.NewDefaultVersionHandler(f.k8sClient)
	if err != nil {
		return fmt.Errorf("could not initialize default version handler: %w", err)
	}
	return vh.LatestVersion(ctx)
}
