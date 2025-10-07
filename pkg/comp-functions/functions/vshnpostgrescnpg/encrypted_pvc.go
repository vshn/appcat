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

	log.Info("Adding secrets to composite")
	// Whenever CNPG scales, new instances won't reuse a previous index.
	// Therefore, it's important to check what instances the Cluster CR is currently reporting.
	// We are proxying this information through the releases connection details in order to save on provider-kubernetes objects.
	clusterInstances, err := getClusterInstancesReportedByCd(svc, comp)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot set up LUKS: %v", err))
	}

	luksKeyNames := getLuksKeyNames(clusterInstances)
	result := writeLuksSecret(svc, log, comp, luksKeyNames)
	if result != nil {
		return result
	}

	for _, luksKey := range luksKeyNames {
		ready := svc.WaitForDesiredDependencies(comp.GetName(), luksKey)
		if !ready {
			return runtime.NewWarningResult(fmt.Sprintf("luks secret '%s' not yet ready", luksKey))
		}
	}

	// As we proxy that info through the connection details, we'd have to wait up to an hour for crossplane to reconcile the CD.
	// By comparing the amount of instances vs what is reported in the CD, we can force an earlier reconcilation by setting the release unready.
	// Why not just use an observer? Because that would take more time and require us to use an evil object.
	svc.Log.Info("Checking for instances discrepancy",
		"comp", comp.Spec.Parameters.Instances,
		"reported", len(clusterInstances),
		"verdict", comp.Spec.Parameters.Instances != len(clusterInstances),
	)

	if comp.Spec.Parameters.Instances != len(clusterInstances) {
		svc.SetDesiredResourceReadiness(comp.GetName(), runtime.ResourceUnReady)
		svc.Log.Info("discrepancy in comp instances vs reported instances in connection details found, encouraging early reconcilation...")
	}

	return nil
}

func writeLuksSecret(svc *runtime.ServiceRuntime, log logr.Logger, comp *vshnv1.VSHNPostgreSQL, luksKeys []string) *xfnproto.Result {
	for _, luksKeyName := range luksKeys {
		log.Info("Processing LUKS key...", "luksKey", luksKeyName)

		secret := &v1.Secret{}
		err := svc.GetObservedKubeObject(secret, luksKeyName)
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
				Name:      luksKeyName,
				Namespace: comp.GetInstanceNamespace(),
			},
			Data: map[string][]byte{
				"luksKey": []byte(luksKey),
			},
		}

		// Allow deletion is required for scaling
		err = svc.SetDesiredKubeObject(secret, luksKeyName, runtime.KubeOptionAllowDeletion)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("cannot add luks secret object: %w", err))
		}
	}

	return nil
}

// Generate LUKS key secret names based on an array of cluster instances
func getLuksKeyNames(clusterInstances []string) []string {
	var output []string
	for _, instance := range clusterInstances {
		output = append(output, []string{
			instance + "-luks-key",
			instance + "-wal-luks-key",
		}...)
	}

	return output
}
