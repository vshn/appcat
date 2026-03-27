package garagebucket

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
)

// ProvisionGarageBucket will create a bucket for the garage operator.
func ProvisionGarageBucket(_ context.Context, bucket *appcatv1.ObjectBucket, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(bucket)
	if err != nil {
		return runtime.NewFatalResult(err)
	}

	// Set the effective bucket name in status
	bucket.Status.BucketName = bucket.GetBucketName()
	err = svc.SetDesiredCompositeStatus(bucket)
	if err != nil {
		return runtime.NewFatalResult(err)
	}

	values := map[string]any{
		"bucketName":     bucket.GetBucketName(),
		"clusterRef":     svc.Config.Data["garageClaim"],
		"claimNamespace": svc.Config.Data["garageClaimNamespace"],
	}

	connectionDetails := getConnectionDetailDefinitions(svc, bucket)

	release, err := newBucketRelease(svc, bucket, values, connectionDetails...)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create release: %w", err))
	}

	err = svc.SetDesiredComposedResource(release)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot set desired release: %w", err))
	}

	err = svc.AddObservedConnectionDetails(bucket.GetName())
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot add connection details to composite: %s", err))
	}

	svc.SetConnectionDetail("BUCKET_NAME", []byte(bucket.GetBucketName()))

	return nil
}

func newBucketRelease(svc *runtime.ServiceRuntime, comp *appcatv1.ObjectBucket, values map[string]any, cd ...xhelmv1.ConnectionDetail) (*xhelmv1.Release, error) {
	vb, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	release := &xhelmv1.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name: comp.GetName(),
			Annotations: map[string]string{
				runtime.IgnoreConnectionDetailsAnnotation: "true",
			},
		},
		Spec: xhelmv1.ReleaseSpec{
			ForProvider: xhelmv1.ReleaseParameters{
				Chart: xhelmv1.ChartSpec{
					Repository: svc.Config.Data["chartRepository"],
					Version:    svc.Config.Data["chartVersion"],
					Name:       svc.Config.Data["chartName"],
				},
				Namespace: svc.Config.Data["garageClaimNamespace"],
				ValuesSpec: xhelmv1.ValuesSpec{
					Values: k8sruntime.RawExtension{
						Raw: vb,
					},
				},
			},
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: "helm",
				},
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name:      comp.GetName() + "-bucket",
					Namespace: svc.GetCrossplaneNamespace(),
				},
			},
			ConnectionDetails: cd,
		},
	}

	if strings.HasPrefix(svc.Config.Data["chartRepository"], "oci://") {
		release.Spec.ForProvider.Chart.URL = fmt.Sprintf("%s:%s", svc.Config.Data["chartRepository"], svc.Config.Data["chartVersion"])
	}

	return release, nil
}

func getConnectionDetailDefinitions(svc *runtime.ServiceRuntime, bucket *appcatv1.ObjectBucket) []xhelmv1.ConnectionDetail {
	return []xhelmv1.ConnectionDetail{
		{
			ObjectReference: corev1.ObjectReference{
				Kind:       "Secret",
				APIVersion: "v1",
				Namespace:  svc.Config.Data["garageClaimNamespace"],
				Name:       bucket.GetBucketName(),
				FieldPath:  "data.endpoint",
			},
			SkipPartOfReleaseCheck: true,
			ToConnectionSecretKey:  "ENDPOINT_URL",
		},
		{
			ObjectReference: corev1.ObjectReference{
				Kind:       "Secret",
				APIVersion: "v1",
				Namespace:  svc.Config.Data["garageClaimNamespace"],
				Name:       bucket.GetBucketName(),
				FieldPath:  "data.host",
			},
			SkipPartOfReleaseCheck: true,
			ToConnectionSecretKey:  "ENDPOINT",
		},
		{
			ObjectReference: corev1.ObjectReference{
				Kind:       "Secret",
				APIVersion: "v1",
				Namespace:  svc.Config.Data["garageClaimNamespace"],
				Name:       bucket.GetBucketName(),
				FieldPath:  "data.access-key-id",
			},
			SkipPartOfReleaseCheck: true,
			ToConnectionSecretKey:  "AWS_ACCESS_KEY_ID",
		},
		{
			ObjectReference: corev1.ObjectReference{
				Kind:       "Secret",
				APIVersion: "v1",
				Namespace:  svc.Config.Data["garageClaimNamespace"],
				Name:       bucket.GetBucketName(),
				FieldPath:  "data.secret-access-key",
			},
			SkipPartOfReleaseCheck: true,
			ToConnectionSecretKey:  "AWS_SECRET_ACCESS_KEY",
		},
		{
			ObjectReference: corev1.ObjectReference{
				Kind:       "Secret",
				APIVersion: "v1",
				Namespace:  svc.Config.Data["garageClaimNamespace"],
				Name:       bucket.GetBucketName(),
				FieldPath:  "data.region",
			},
			SkipPartOfReleaseCheck: true,
			ToConnectionSecretKey:  "AWS_REGION",
		},
	}
}
