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

// Redis holds all the necessary objects to do a Redis maintenance
type Redis struct {
	backupHelper *backup.Helper
	k8sClient    client.WithWatch
	httpClient   *http.Client
	log          logr.Logger
	release.VersionHandler
}

// NewRedis returns a new Redis maintenance job runner
func NewRedis(c client.WithWatch, hc *http.Client, vh release.VersionHandler, logger logr.Logger) *Redis {
	runner := backup.NewK8upBackupRunner(c, logger)
	return &Redis{
		backupHelper:   backup.NewHelper(runner, logger),
		k8sClient:      c,
		httpClient:     hc,
		log:            logger,
		VersionHandler: vh,
	}
}

// RunBackup executes a pre-maintenance backup
func (r *Redis) RunBackup(ctx context.Context) error {
	return r.backupHelper.RunBackup(ctx)
}

// DoMaintenance will run redis' maintenance script.
func (r *Redis) DoMaintenance(ctx context.Context) error {
	maintenanceURL, err := getMaintenanceURL()
	if err != nil {
		return err
	}

	patcher := helm.NewImagePatcher(r.k8sClient, r.httpClient, r.log)
	return patcher.DoMaintenance(ctx, maintenanceURL, helm.NewValuePath("image", "tag"), helm.SemVerPatchesOnly(false))
}
