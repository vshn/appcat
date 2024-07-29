package cloudscalebucket

import (
	"context"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	cloudscalev1 "github.com/vshn/provider-cloudscale/apis/cloudscale/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProvisionCloudscalebucket will create a bucket in a pre-deployed minio instance.
// This function will leverage provider-minio to deploy proper policies and users
// alongside the bucket.
func ProvisionCloudscalebucket(_ context.Context, svc *runtime.ServiceRuntime) *xfnproto.Result {

	bucket := &appcatv1.ObjectBucket{}

	err := svc.GetObservedComposite(bucket)
	if err != nil {
		return runtime.NewFatalResult(err)
	}

	config, ok := svc.Config.Data["providerConfig"]
	if !ok {
		return runtime.NewFatalResult(fmt.Errorf("no providerConfig specified"))
	}

	err = addBucket(svc, bucket, config)
	if err != nil {
		return runtime.NewFatalResult(err)
	}

	err = addUser(svc, bucket, config)
	if err != nil {
		return runtime.NewFatalResult(err)
	}

	svc.SetConnectionDetail("BUCKET_NAME", []byte(bucket.Spec.Parameters.BucketName))
	svc.SetConnectionDetail("AWS_REGION", []byte(bucket.Spec.Parameters.Region))

	err = populateEndpointConnectionDetails(svc)
	if err != nil {
		return runtime.NewFatalResult(err)
	}

	return nil
}

func addBucket(svc *runtime.ServiceRuntime, bucket *appcatv1.ObjectBucket, config string) error {

	mb := &cloudscalev1.Bucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: bucket.Spec.Parameters.BucketName,
		},
		Spec: cloudscalev1.BucketSpec{
			ForProvider: cloudscalev1.BucketParameters{
				BucketDeletionPolicy: cloudscalev1.BucketDeletionPolicy(bucket.Spec.Parameters.BucketDeletionPolicy),
			},
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: config,
				},
			},
		},
	}

	return svc.SetDesiredComposedResourceWithName(mb, "minio-bucket")
}

func addUser(svc *runtime.ServiceRuntime, bucket *appcatv1.ObjectBucket, config string) error {

	user := &cloudscalev1.ObjectsUser{
		ObjectMeta: metav1.ObjectMeta{
			Name: bucket.Spec.Parameters.BucketName,
		},
		Spec: cloudscalev1.ObjectsUserSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: config,
				},
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name:      bucket.GetName(),
					Namespace: "syn-crossplane",
				},
			},
			ForProvider: cloudscalev1.ObjectsUserParameters{
				DisplayName: fmt.Sprintf("%.%", bucket.Labels["crossplane.io/claim-namespace"], bucket.Labels["crossplane.io/claim-name"]),
			},
		},
	}

	cd, err := svc.GetObservedComposedResourceConnectionDetails("cloudscale-user")
	if err != nil && err != runtime.ErrNotFound {
		return err
	}

	for v, k := range cd {
		svc.SetConnectionDetail(v, k)
	}

	return svc.SetDesiredComposedResourceWithName(user, "cloudscale-user")
}

func populateEndpointConnectionDetails(svc *runtime.ServiceRuntime) error {

	bucket := &cloudscalev1.Bucket{}

	err := svc.GetObservedComposedResource(bucket, "cloudscale-bucket")
	if err != nil && err == runtime.ErrNotFound {
		return nil
	} else if err != nil {
		return err
	}

	svc.SetConnectionDetail("ENDPOINT", []byte(bucket.Status.Endpoint))
	svc.SetConnectionDetail("ENDPOINT_URL", []byte(bucket.Status.EndpointURL))

	return nil

}
