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

// Forgejo contains all necessary dependencies to successfully run a Forgejo maintenance
type Forgejo struct {
	backupWaiter *backup.Waiter
	k8sClient    client.WithWatch
	httpClient   *http.Client
	log          logr.Logger
	release.VersionHandler
}

// NewForgejo returns a new Forgejo object
func NewForgejo(c client.WithWatch, hc *http.Client, vh release.VersionHandler, logger logr.Logger) *Forgejo {
	return &Forgejo{
		backupWaiter:   backup.NewK8upWaiter(c, logger),
		k8sClient:      c,
		httpClient:     hc,
		log:            logger,
		VersionHandler: vh,
	}
}

// WaitForRunningBackups waits until no backup is running anymore
func (f *Forgejo) WaitForRunningBackups(ctx context.Context) error {
	return f.backupWaiter.WaitForRunningBackups(ctx)
}

// DoMaintenance will run Forgejo's maintenance script.
func (f *Forgejo) DoMaintenance(ctx context.Context) error {
	maintenanceURL, err := getMaintenanceURL()
	if err != nil {
		return err
	}

	patcher := helm.NewImagePatcher(f.k8sClient, f.httpClient, f.log)
	valuesPath := helm.NewValuePath("image", "tag")
	return patcher.DoMaintenance(ctx, maintenanceURL, valuesPath, helm.SemVerPatchesOnly(true))
}
