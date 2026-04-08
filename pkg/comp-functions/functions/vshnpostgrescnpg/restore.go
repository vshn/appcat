package vshnpostgrescnpg

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	barmancloudv1 "github.com/vshn/appcat/v4/apis/barmancloud/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed scripts/copy-cnpg-backup-creds.sh
var cnpgCopyJobScript string

const (
	recoverySecretName      = "cnpg-recovery-credentials"
	recoveryObjectStoreName = "postgresql-recovery-object-store"
)

// handleRestore checks for restore parameters and orchestrates the CNPG restore flow.
// Returns (result, skipHelmRelease). When skipHelmRelease is true, the caller must NOT
// create the Helm release this reconcile cycle.
func handleRestore(ctx context.Context, comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime, values map[string]any) (*xfnproto.Result, bool) {
	restore := comp.Spec.Parameters.Restore
	if restore == nil || restore.ClaimName == "" {
		return nil, false
	}

	l := svc.Log
	l.Info("Restore requested", "claimName", restore.ClaimName)

	// Always create the copy job (idempotent)
	if err := createCnpgCopyJob(comp, svc); err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create CNPG copy job: %v", err)), false
	}

	// Create an observe-only KubeObject for the recovery credentials secret
	observerName := comp.GetName() + "-cnpg-recovery-creds"
	recoverySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      recoverySecretName,
			Namespace: comp.GetInstanceNamespace(),
		},
	}
	err := svc.SetDesiredKubeObject(recoverySecret, observerName, runtime.KubeOptionObserve, runtime.KubeOptionAllowDeletion)
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create recovery credentials observer: %v", err)), false
	}

	// Try to read the observed recovery credentials secret
	observedSecret := &corev1.Secret{}
	err = svc.GetObservedKubeObject(observedSecret, observerName)
	if err != nil || len(observedSecret.Data) == 0 {
		l.Info("Recovery credentials not yet available, waiting for copy job to complete")
		return runtime.NewWarningResult("Waiting for recovery credentials to be copied from source instance"), true
	}

	// Check if recovery is already complete (Helm release exists and is ready)
	releaseName := comp.GetName() + "-cnpg"
	ready, readyErr := svc.IsResourceReady(releaseName)
	if readyErr == nil && ready {
		l.Info("Recovery complete, switching to standalone mode")
		// Don't set recovery values — normal standalone + backup flow takes over
		return nil, false
	}

	l.Info("Setting up recovery mode")

	// Create the recovery ObjectStore and set recovery Helm values
	if err := createRecoveryResources(comp, svc, observedSecret.Data); err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot create recovery resources: %v", err)), false
	}

	setRecoveryValues(values, observedSecret.Data, comp)
	return nil, false
}

// createCnpgCopyJob creates a Kubernetes Job that resolves the source instance's
// backup bucket credentials and copies them to the target namespace.
func createCnpgCopyJob(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime) error {
	copyJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName() + "-cnpg-copyjob",
			Namespace: svc.Config.Data["controlNamespace"],
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      "Never",
					ServiceAccountName: "copyserviceaccount",
					Containers: []corev1.Container{
						{
							Name:    "copyjob",
							Image:   svc.Config.Data["kubectl_image"],
							Command: []string{"sh", "-c"},
							Args:    []string{cnpgCopyJobScript},
							Env: []corev1.EnvVar{
								{
									Name:  "CLAIM_NAMESPACE",
									Value: comp.GetClaimNamespace(),
								},
								{
									Name:  "CLAIM_NAME",
									Value: comp.Spec.Parameters.Restore.ClaimName,
								},
								{
									Name:  "CLAIM_TYPE",
									Value: comp.Spec.Parameters.Restore.ClaimType,
								},
								{
									Name:  "TARGET_NAMESPACE",
									Value: comp.GetInstanceNamespace(),
								},
								{
									Name:  "CROSSPLANE_NAMESPACE",
									Value: svc.GetCrossplaneNamespace(),
								},
							},
						},
					},
				},
			},
		},
	}

	return svc.SetDesiredKubeObjectWithName(copyJob, comp.GetName()+"-cnpg-copyjob", "cnpg-copy-job", runtime.KubeOptionAllowDeletion)
}

// createRecoveryResources creates the recovery ObjectStore CR in the target namespace.
// This ObjectStore points to the source instance's backup bucket and is referenced by
// the Helm chart's external cluster for recovery bootstrap.
func createRecoveryResources(comp *vshnv1.VSHNPostgreSQL, svc *runtime.ServiceRuntime, secretData map[string][]byte) error {
	endpoint := strings.TrimSuffix(string(secretData["ENDPOINT_URL"]), "/")
	bucket := string(secretData["BUCKET_NAME"])

	objectStore := &barmancloudv1.ObjectStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      recoveryObjectStoreName,
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: barmancloudv1.ObjectStoreSpec{
			Configuration: barmancloudv1.ObjectStoreConfiguration{
				EndpointURL:     endpoint,
				DestinationPath: fmt.Sprintf("s3://%s/", bucket),
				S3Credentials: &barmancloudv1.S3Credentials{
					AccessKeyId: barmancloudv1.SecretKeySelector{
						Name: recoverySecretName,
						Key:  "AWS_ACCESS_KEY_ID",
					},
					SecretAccessKey: barmancloudv1.SecretKeySelector{
						Name: recoverySecretName,
						Key:  "AWS_SECRET_ACCESS_KEY",
					},
					Region: barmancloudv1.SecretKeySelector{
						Name: recoverySecretName,
						Key:  "AWS_REGION",
					},
				},
			},
		},
	}

	return svc.SetDesiredKubeObjectWithName(objectStore, comp.GetName()+"-recovery-object-store", "recovery-object-store", runtime.KubeOptionAllowDeletion)
}

// setRecoveryValues configures the Helm values for recovery mode.
// The chart will use the external recovery ObjectStore for bootstrap
// while keeping its own ObjectStore for ongoing backups.
// The clusterName is derived from the restore instance's own major version,
// which determines which archive path barman-cloud searches in the object store.
// This allows rollback after a failed major version upgrade: the user creates
// a restore instance on the previous major version, and it finds the matching backups.
func setRecoveryValues(values map[string]any, secretData map[string][]byte, comp *vshnv1.VSHNPostgreSQL) {
	clusterName := "postgresql-" + comp.Spec.Parameters.Service.MajorVersion

	recovery := map[string]any{
		"method":          "object_store",
		"objectStoreName": recoveryObjectStoreName,
		"clusterName":     clusterName,
	}

	if comp.Spec.Parameters.Restore.RecoveryTimeStamp != "" {
		recovery["pitrTarget"] = map[string]any{
			"time": comp.Spec.Parameters.Restore.RecoveryTimeStamp,
		}
	}

	values["mode"] = "recovery"
	values["recovery"] = recovery
}
