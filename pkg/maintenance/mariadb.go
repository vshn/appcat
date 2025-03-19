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
	mariaDBURL = "https://hub.docker.com/v2/repositories/bitnami/mariadb-galera/tags/?page_size=100"
)

// MariaDB holds all the necessary objects to do a MariaDB maintenance
type MariaDB struct {
	k8sClient  client.Client
	httpClient *http.Client
}

// NewMariaDB returns a new Redis maintenance job runner
func NewMariaDB(c client.Client, hc *http.Client) *MariaDB {
	return &MariaDB{
		k8sClient:  c,
		httpClient: hc,
	}
}

// DoMaintenance will run redis' maintenance script.
func (m *MariaDB) DoMaintenance(ctx context.Context) error {
	patcher := helm.NewImagePatcher(m.k8sClient, m.httpClient, logr.FromContextOrDiscard(ctx).WithValues("type", "mariadb"))

	return patcher.DoMaintenance(ctx, mariaDBURL, helm.NewValuePath("image", "tag"), helm.SemVerPatchesOnly(false))
}

func (m *MariaDB) ReleaseLatestAppCatVersion(ctx context.Context) error {
	vh, err := release.NewDefaultVersionHandler(m.k8sClient)
	if err != nil {
		return fmt.Errorf("could not initialize default version handler: %w", err)
	}
	return vh.LatestVersion(ctx)
}
