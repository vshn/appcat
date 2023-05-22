package vshnpostgres

import (
	"context"
	"fmt"
	runtime2 "github.com/vshn/appcat/pkg/comp-functions/runtime"

	"github.com/sethvargo/go-password/password"
	controllerruntime "sigs.k8s.io/controller-runtime"

	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AddPvcSecret adds a secret for the encrypted PVC for the  PostgreSQL instance.
func AddPvcSecret(ctx context.Context, iof *runtime2.Runtime) runtime2.Result {

	log := controllerruntime.LoggerFrom(ctx)

	comp := &vshnv1.VSHNPostgreSQL{}
	err := iof.Observed.GetComposite(ctx, comp)

	if err != nil {
		return runtime2.NewFatalErr(ctx, "Cannot get composite", err)
	}
	log.Info("Check if encrypted storage is enabled")

	log.V(1).Info("Transforming", "obj", iof)

	encryptionSpec := comp.Spec.Parameters.Encryption

	if !encryptionSpec.Enabled {
		log.Info("Encryption not enabled")
		return runtime2.NewNormal()
	}

	if comp.Status.InstanceNamespace == "" {
		return runtime2.NewWarning(ctx, "Composite is missing instance namespace, skipping transformation")
	}

	log.Info("Adding secret to composite")

	secret := &v1.Secret{}

	// luksSecretResourceName is the resource name defined in the composition
	// This name is different from metadata.name of the same resource
	// The value is hardcoded in the composition for each resource and due to crossplane limitation
	// it cannot be matched to the metadata.name
	luksSecretResourceName := comp.Name + "-luks-key"

	err = iof.Observed.GetFromObject(ctx, secret, luksSecretResourceName)
	luksKey := ""
	if err == runtime2.ErrNotFound {
		log.Info("Secret does not exist yet. Creating...")
		luksKey, err = password.Generate(64, 10, 1, false, true)
		if err != nil {
			return runtime2.NewFatalErr(ctx, "Cannot generate new luksKey", err)
		}
	} else if err == nil {
		log.Info("retreiviing existing secret key...")
		luksKey = string(secret.Data["luksKey"])
	} else {
		return runtime2.NewFatalErr(ctx, "Cannot get luks secret object", err)
	}

	secret = &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-data-%s-0-luks-key", comp.ObjectMeta.Labels["crossplane.io/composite"], comp.ObjectMeta.Labels["crossplane.io/composite"]),
			Namespace: comp.Status.InstanceNamespace,
		},
		Data: map[string][]byte{
			"luksKey": []byte(luksKey),
		},
	}
	err = iof.Desired.PutIntoObject(ctx, secret, luksSecretResourceName)
	if err != nil {
		return runtime2.NewFatalErr(ctx, "Cannot add luks secret object", err)
	}

	return runtime2.NewNormal()
}
