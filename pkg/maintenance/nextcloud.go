package maintenance

import (
	"context"
	"net/http"

	"github.com/vshn/appcat/v4/pkg/maintenance/release"

	"github.com/go-logr/logr"
	"github.com/vshn/appcat/v4/pkg/maintenance/backup"
	"github.com/vshn/appcat/v4/pkg/maintenance/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Nextcloud contains all necessary dependencies to successfully run a Nextcloud maintenance
type Nextcloud struct {
	backupHelper *backup.Helper
	k8sClient    client.WithWatch
	httpClient   *http.Client
	log          logr.Logger
	release.VersionHandler
}

// NewNextcloud returns a new Nextcloud object
func NewNextcloud(c client.WithWatch, hc *http.Client, vh release.VersionHandler, logger logr.Logger) *Nextcloud {
	runner := backup.NewK8upBackupRunner(c, logger)
	return &Nextcloud{
		backupHelper:   backup.NewHelper(runner, logger),
		k8sClient:      c,
		httpClient:     hc,
		log:            logger,
		VersionHandler: vh,
	}
}

// RunBackup executes a pre-maintenance backup
func (n *Nextcloud) RunBackup(ctx context.Context) error {
	return n.backupHelper.RunBackup(ctx)
}

// DoMaintenance will run Nextcloud's maintenance script.
func (n *Nextcloud) DoMaintenance(ctx context.Context) error {
	maintenanceURL, err := getMaintenanceURL()
	if err != nil {
		return err
	}

	patcher := helm.NewImagePatcher(n.k8sClient, n.httpClient, n.log)
	valuesPath := helm.NewValuePath("image", "tag")
	return patcher.DoMaintenance(ctx, maintenanceURL, valuesPath, helm.SemVerPatchesOnly(true))
}
