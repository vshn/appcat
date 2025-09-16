package vshnpostgrescnpg

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/go-logr/logr"
	"github.com/sethvargo/go-password/password"
	controllerruntime "sigs.k8s.io/controller-runtime"

	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"

	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddPvcSecret adds a secret for the encrypted PVC for the PostgreSQL instance.
func AddPvcSecret(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) *xfnproto.Result {
	log := controllerruntime.LoggerFrom(ctx)

	err := svc.GetObservedComposite(comp)

	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}
	log.Info("Check if encrypted storage is enabled")

	log.V(1).Info("Transforming")

	if !comp.Spec.Parameters.Encryption.Enabled {
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
			return runtime.NewFatalResult(fmt.Errorf("cannot generate new luksKey: %w", err))
		}
	} else if err == nil {
		log.Info("retrieving existing secret key...")
		luksKey = string(secret.Data["luksKey"])
	} else {
		return runtime.NewFatalResult(fmt.Errorf("cannot get luks secret object: %w", err))
	}

	secret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-data-%s-%d-luks-key", comp.ObjectMeta.Labels["crossplane.io/composite"], comp.ObjectMeta.Labels["crossplane.io/composite"], i),
			Namespace: comp.GetInstanceNamespace(),
		},
		Data: map[string][]byte{
			"luksKey": []byte(luksKey),
		},
	}
	err = svc.SetDesiredKubeObject(secret, luksSecretResourceName)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot add luks secret object: %w", err))
	}
	return nil
}
