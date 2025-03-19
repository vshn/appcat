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
	mariaDBURL = "https://hub.docker.com/v2/repositories/bitnami/mariadb-galera/tags/?page_size=100"
)

// MariaDB holds all the necessary objects to do a MariaDB maintenance
type MariaDB struct {
	k8sClient  client.Client
	httpClient *http.Client
	log        logr.Logger
	release.VersionHandler
}

// NewMariaDB returns a new Redis maintenance job runner
func NewMariaDB(c client.Client, hc *http.Client, vh release.VersionHandler, logger logr.Logger) *MariaDB {
	return &MariaDB{
		k8sClient:      c,
		httpClient:     hc,
		log:            logger,
		VersionHandler: vh,
	}
}

// DoMaintenance will run redis' maintenance script.
func (m *MariaDB) DoMaintenance(ctx context.Context) error {
	patcher := helm.NewImagePatcher(m.k8sClient, m.httpClient, m.log)
	return patcher.DoMaintenance(ctx, mariaDBURL, helm.NewValuePath("image", "tag"), helm.SemVerPatchesOnly(false))
}
