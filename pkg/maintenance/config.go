package maintenance

import (
	"context"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MaintenanceConfig holds the per-service mainteancne configuration
type MaintenanceConfig struct {
	DisableServiceRelease bool
	DisableAppcatRelease  bool
}

func GetMaintenanceConfig(ctx context.Context, c client.Reader, cmName, namespace, service string) (MaintenanceConfig, error) {
	conf := MaintenanceConfig{}

	cm := &corev1.ConfigMap{}

	err := c.Get(ctx, types.NamespacedName{Name: cmName, Namespace: namespace}, cm)
	if err != nil {
		return conf, err
	}

	conf.DisableServiceRelease = parseBool(cm.Data[service+".disableServiceRelease"])
	conf.DisableAppcatRelease = parseBool(cm.Data[service+".disableAppcatRelease"])

	return conf, nil
}

// parseBool will parse the string to a boolean
// Invalid and empty strings are treated as false
func parseBool(s string) bool {
	v, _ := strconv.ParseBool(s)
	return v
}
