package maintenance

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/vshn/appcat/v4/pkg/maintenance/helm"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	redisURL = "https://hub.docker.com/v2/repositories/bitnami/redis/tags/?page_size=100"
)

// Redis holds all the necessary objects to do a Redis maintenance
type Redis struct {
	k8sClient  client.Client
	httpClient *http.Client
}

// NewRedis returns a new Redis maintenance job runner
func NewRedis(c client.Client, hc *http.Client) *Redis {
	return &Redis{
		k8sClient:  c,
		httpClient: hc,
	}
}

// DoMaintenance will run redis' maintenance script.
func (r *Redis) DoMaintenance(ctx context.Context) error {
	patcher := helm.NewImagePatcher(r.k8sClient, r.httpClient, logr.FromContextOrDiscard(ctx).WithValues("type", "redis"))

	return patcher.DoMaintenance(ctx, redisURL, helm.NewValuePath("image", "tag"), helm.SemVerPatchesOnly(false))
}
