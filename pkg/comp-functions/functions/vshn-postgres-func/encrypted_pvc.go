package vshnpostgres

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/sethvargo/go-password/password"
	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"

	stackgresv1 "github.com/vshn/appcat/v4/apis/stackgres/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// AddPvcSecret adds a secret for the encrypted PVC for the  PostgreSQL instance.
func AddPvcSecret(ctx context.Context, iof *runtime.Runtime) runtime.Result {

	log := controllerruntime.LoggerFrom(ctx)

	comp := &vshnv1.VSHNPostgreSQL{}
	err := iof.Observed.GetComposite(ctx, comp)

	if err != nil {
		return runtime.NewFatalErr(ctx, "Cannot get composite", err)
	}
	log.Info("Check if encrypted storage is enabled")

	log.V(1).Info("Transforming")

	encryptionSpec := comp.Spec.Parameters.Encryption

	if !encryptionSpec.Enabled {
		log.Info("Encryption not enabled")
		return runtime.NewNormal()
	}

	log.Info("Adding secret to composite")

	cluster := &stackgresv1.SGCluster{}
	err = iof.Desired.GetFromObject(ctx, cluster, "cluster")
	if err != nil {
		return runtime.NewFatalErr(ctx, "Cannot get SgCluster object", err)
	}

	cluster.Spec.Pods.PersistentVolume.StorageClass = pointer.String("ssd-encrypted")
	err = iof.Desired.PutIntoObject(ctx, cluster, "cluster")

	if err != nil {
		return runtime.NewFatalErr(ctx, "Cannot edit SGCluster object", err)
	}

	pods := cluster.Spec.Instances

	for i := 0; i < pods; i++ {
		result := writeLuksSecret(ctx, iof, log, comp, i)
		if result != nil {
			return result
		}
	}

	return runtime.NewNormal()
}

func writeLuksSecret(ctx context.Context, iof *runtime.Runtime, log logr.Logger, comp *vshnv1.VSHNPostgreSQL, i int) runtime.Result {
	// luksSecretResourceName is the resource name defined in the composition
	// This name is different from metadata.name of the same resource
	// The value is hardcoded in the composition for each resource and due to crossplane limitation
	// it cannot be matched to the metadata.name
	luksSecretResourceName := fmt.Sprintf("%s-luks-key-%d", comp.Name, i)

	secret := &v1.Secret{}
	err := iof.Observed.GetFromObject(ctx, secret, luksSecretResourceName)
	luksKey := ""
	if err == runtime.ErrNotFound {
		log.Info("Secret does not exist yet. Creating...")
		luksKey, err = password.Generate(64, 10, 1, false, true)
		if err != nil {
			return runtime.NewFatalErr(ctx, "Cannot generate new luksKey", err)
		}
	} else if err == nil {
		log.Info("retreiviing existing secret key...")
		luksKey = string(secret.Data["luksKey"])
	} else {
		return runtime.NewFatalErr(ctx, "Cannot get luks secret object", err)
	}

	secret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-data-%s-%d-luks-key", comp.ObjectMeta.Labels["crossplane.io/composite"], comp.ObjectMeta.Labels["crossplane.io/composite"], i),
			Namespace: getInstanceNamespace(comp),
		},
		Data: map[string][]byte{
			"luksKey": []byte(luksKey),
		},
	}
	err = iof.Desired.PutIntoObject(ctx, secret, luksSecretResourceName)
	if err != nil {
		return runtime.NewFatalErr(ctx, "Cannot add luks secret object", err)
	}
	return nil
}
