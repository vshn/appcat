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
		index := i + 1 // i+1 because we are dealing with an STS
		result := writeLuksSecret(svc, log, comp, index)
		if result != nil {
			return result
		}

		for _, luksKey := range getLuksKeyNamesArray(comp, index) {
			ready := svc.WaitForDesiredDependencies(comp.GetName(), luksKey)
			if !ready {
				return runtime.NewWarningResult(fmt.Sprintf("luks secret '%s' not yet ready", luksKey))
			}
		}
	}

	return nil
}

func writeLuksSecret(svc *runtime.ServiceRuntime, log logr.Logger, comp *vshnv1.VSHNPostgreSQL, i int) *xfnproto.Result {
	// luksSecretResourceName is the resource name defined in the composition
	// This name is different from metadata.name of the same resource
	// The value is hardcoded in the composition for each resource and due to crossplane limitation
	// it cannot be matched to the metadata.name
	for _, luksSecretResourceName := range getLuksKeyNamesArray(comp, i) {
		log.Info("Processing LUKS key...", "luksKey", luksSecretResourceName)

		secret := &v1.Secret{}
		err := svc.GetObservedKubeObject(secret, luksSecretResourceName)
		luksKey := ""
		switch err {
		case runtime.ErrNotFound:
			log.Info("Secret does not exist yet. Creating...")
			luksKey, err = password.Generate(64, 10, 1, false, true)
			if err != nil {
				return runtime.NewFatalResult(fmt.Errorf("cannot generate new luksKey: %w", err))
			}
		case nil:
			log.Info("retrieving existing secret key...")
			luksKey = string(secret.Data["luksKey"])
		default:
			return runtime.NewFatalResult(fmt.Errorf("cannot get luks secret object: %w", err))
		}

		secret = &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      luksSecretResourceName,
				Namespace: comp.GetInstanceNamespace(),
			},
			Data: map[string][]byte{
				"luksKey": []byte(luksKey),
			},
		}

		// Allow deletion required for scaling
		err = svc.SetDesiredKubeObject(secret, luksSecretResourceName, runtime.KubeOptionAllowDeletion)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("cannot add luks secret object: %w", err))
		}
	}

	return nil
}

func getLuksKeyNamesArray(comp *vshnv1.VSHNPostgreSQL, index int) []string {
	return []string{
		fmt.Sprintf("%s-cluster-%d-luks-key", comp.Name, index),
		fmt.Sprintf("%s-cluster-%d-wal-luks-key", comp.Name, index),
	}
}
