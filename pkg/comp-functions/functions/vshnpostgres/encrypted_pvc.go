package vshnpostgres

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
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
func AddPvcSecret(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {

	log := controllerruntime.LoggerFrom(ctx)

	err := svc.GetObservedComposite(comp)

	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot get composite: %w", err))
	}
	log.Info("Check if encrypted storage is enabled")

	log.V(1).Info("Transforming")

	encryptionSpec := comp.Spec.Parameters.Encryption

	if !encryptionSpec.Enabled {
		log.Info("Encryption not enabled")
		return nil
	}

	log.Info("Adding secret to composite")

	pods := comp.GetInstances()

	for i := 0; i < pods; i++ {
		result := writeLuksSecret(svc, log, comp, i)
		if result != nil {
			return result
		}
		ready := svc.WaitForDesiredDependencies(comp.GetName(), fmt.Sprintf("%s-luks-key-%d", comp.Name, i))
		if !ready {
			return runtime.NewWarningResult("luks secret not yet ready")
		}
	}

	cluster := &stackgresv1.SGCluster{}
	err = svc.GetDesiredKubeObject(cluster, "cluster")
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot get SgCluster object: %w", err))
	}

	cluster.Spec.Pods.PersistentVolume.StorageClass = pointer.String("ssd-encrypted")
	err = svc.SetDesiredKubeObjectWithName(cluster, comp.GetName()+"-cluster", "cluster")

	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot edit SGCluster object: %w", err))
	}

	return nil
}

func writeLuksSecret(svc *runtime.ServiceRuntime, log logr.Logger, comp *vshnv1.VSHNPostgreSQL, i int) *xfnproto.Result {
	// luksSecretResourceName is the resource name defined in the composition
	// This name is different from metadata.name of the same resource
	// The value is hardcoded in the composition for each resource and due to crossplane limitation
	// it cannot be matched to the metadata.name
	luksSecretResourceName := fmt.Sprintf("%s-luks-key-%d", comp.Name, i)

	secret := &v1.Secret{}
	err := svc.GetObservedKubeObject(secret, luksSecretResourceName)
	luksKey := ""
	if err == runtime.ErrNotFound {
		log.Info("Secret does not exist yet. Creating...")
		luksKey, err = password.Generate(64, 10, 1, false, true)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("Cannot generate new luksKey: %w", err))
		}
	} else if err == nil {
		log.Info("retreiviing existing secret key...")
		luksKey = string(secret.Data["luksKey"])
	} else {
		return runtime.NewFatalResult(fmt.Errorf("Cannot get luks secret object: %w", err))
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
	err = svc.SetDesiredKubeObject(secret, luksSecretResourceName)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("Cannot add luks secret object: %w", err))
	}
	return nil
}
