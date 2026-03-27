package maintenance

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/vshn/appcat/v4/pkg/maintenance/backup"
	"github.com/vshn/appcat/v4/pkg/maintenance/helm"
	"github.com/vshn/appcat/v4/pkg/maintenance/release"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MariaDB holds all the necessary objects to do a MariaDB maintenance
type MariaDB struct {
	backupHelper *backup.Helper
	k8sClient    client.WithWatch
	httpClient   *http.Client
	log          logr.Logger
	release.VersionHandler
}

// NewMariaDB returns a new MariaDB maintenance job runner
func NewMariaDB(c client.WithWatch, hc *http.Client, vh release.VersionHandler, logger logr.Logger) *MariaDB {
	runner := backup.NewK8upBackupRunner(c, logger)
	return &MariaDB{
		backupHelper:   backup.NewHelper(runner, logger),
		k8sClient:      c,
		httpClient:     hc,
		log:            logger,
		VersionHandler: vh,
	}
}

// RunBackup executes a pre-maintenance backup
func (m *MariaDB) RunBackup(ctx context.Context) error {
	return m.backupHelper.RunBackup(ctx)
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
