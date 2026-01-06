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
	backupHelper *backup.Helper
	k8sClient    client.WithWatch
	httpClient   *http.Client
	log          logr.Logger
	release.VersionHandler
}

// NewForgejo returns a new Forgejo object
func NewForgejo(c client.WithWatch, hc *http.Client, vh release.VersionHandler, logger logr.Logger) *Forgejo {
	runner := backup.NewK8upBackupRunner(c, logger)
	return &Forgejo{
		backupHelper:   backup.NewHelper(runner, logger),
		k8sClient:      c,
		httpClient:     hc,
		log:            logger,
		VersionHandler: vh,
	}
}

// RunBackup executes a pre-maintenance backup
func (f *Forgejo) RunBackup(ctx context.Context) error {
	return f.backupHelper.RunBackup(ctx)
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
