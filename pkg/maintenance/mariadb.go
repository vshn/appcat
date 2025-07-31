package maintenance

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/vshn/appcat/v4/pkg/maintenance/helm"
	"github.com/vshn/appcat/v4/pkg/maintenance/release"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MariaDB holds all the necessary objects to do a MariaDB maintenance
type MariaDB struct {
	k8sClient  client.Client
	httpClient *http.Client
	log        logr.Logger
	release.VersionHandler
}

// NewMariaDB returns a new MariaDB maintenance job runner
func NewMariaDB(c client.Client, hc *http.Client, vh release.VersionHandler, logger logr.Logger) *MariaDB {
	return &MariaDB{
		k8sClient:      c,
		httpClient:     hc,
		log:            logger,
		VersionHandler: vh,
	}
}

// DoMaintenance will run MariaDB's maintenance script.
func (m *MariaDB) DoMaintenance(ctx context.Context) error {
	maintenanceURL, err := getMaintenanceURL()
	if err != nil {
		return err
	}

	patcher := helm.NewImagePatcher(m.k8sClient, m.httpClient, m.log)
	return patcher.DoMaintenance(ctx, maintenanceURL, helm.NewValuePath("image", "tag"), helm.SemVerPatchesOnly(false))
}
