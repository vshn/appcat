package maintenance

import (
	"context"
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
func (r *MariaDB) DoMaintenance(ctx context.Context) error {
	patcher := helm.NewImagePatcher(r.k8sClient, r.httpClient, logr.FromContextOrDiscard(ctx).WithValues("type", "mariadb"))

	return patcher.DoMaintenance(ctx, mariaDBURL, helm.NewValuePath("image", "tag"), helm.SemVerPatchesOnly(false))
}
