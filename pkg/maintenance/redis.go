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
	backupWaiter *backup.Waiter
	k8sClient    client.WithWatch
	httpClient   *http.Client
	log          logr.Logger
	release.VersionHandler
}

// NewRedis returns a new Redis maintenance job runner
func NewRedis(c client.WithWatch, hc *http.Client, vh release.VersionHandler, logger logr.Logger) *Redis {
	return &Redis{
		backupWaiter:   backup.NewK8upWaiter(c, logger),
		k8sClient:      c,
		httpClient:     hc,
		log:            logger,
		VersionHandler: vh,
	}
}

// WaitForRunningBackups waits until no backup is running anymore
func (r *Redis) WaitForRunningBackups(ctx context.Context) error {
	return r.backupWaiter.WaitForRunningBackups(ctx)
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
