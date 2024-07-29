package cloudscalebucket

import (
	"context"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	cloudscalev1 "github.com/vshn/provider-cloudscale/apis/cloudscale/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProvisionCloudscalebucket will create a bucket in cloudscale.
// This function will leverage provider-cloudscale to deploy proper users
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
		ObjectMeta: metav1.ObjectMeta{},
		Spec: cloudscalev1.BucketSpec{
			ForProvider: cloudscalev1.BucketParameters{
				BucketDeletionPolicy: cloudscalev1.BucketDeletionPolicy(bucket.Spec.Parameters.BucketDeletionPolicy),
				Region:               bucket.Spec.Parameters.Region,
				BucketName:           bucket.Spec.Parameters.BucketName,
				CredentialsSecretRef: v1.SecretReference{
					Namespace: svc.Config.Data["providerSecretNamespace"],
				},
			},
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: config,
				},
			},
		},
	}

	objName := getBucketObjectName(svc, bucket, "cloudscale-bucket", mb.DeepCopy())

	mb.ObjectMeta.Name = objName
	mb.Spec.ForProvider.CredentialsSecretRef.Name = objName

	return svc.SetDesiredComposedResourceWithName(mb, "cloudscale-bucket")
}

func addUser(svc *runtime.ServiceRuntime, bucket *appcatv1.ObjectBucket, config string) error {

	user := &cloudscalev1.ObjectsUser{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: cloudscalev1.ObjectsUserSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: config,
				},
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Namespace: svc.Config.Data["providerSecretNamespace"],
				},
			},
			ForProvider: cloudscalev1.ObjectsUserParameters{
				DisplayName: fmt.Sprintf("%s.%s", bucket.Labels["crossplane.io/claim-namespace"], bucket.Labels["crossplane.io/claim-name"]),
			},
		},
	}

	objName := getBucketObjectName(svc, bucket, "cloudscale-user", user.DeepCopy())

	user.ObjectMeta.Name = objName
	user.Spec.WriteConnectionSecretToReference.Name = objName

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

// Legacy buckets where created with wrong object names that break nested
// services.
// This logic returns the new naming scheme, if there's no existing bucket CR.
// If there's already a CR it will simply return it's name.
// The `obj` parameter should always be a deepCopy of the original, otherwise
// the pointer in the calling function will have all the fields populated. Which
// can lead to unexpected side effects.
func getBucketObjectName(svc *runtime.ServiceRuntime, bucket *appcatv1.ObjectBucket, resName string, obj resource.Managed) string {

	err := svc.GetObservedComposedResource(obj, resName)
	if err != nil {
		return bucket.Spec.Parameters.BucketName
	}

	return obj.GetName()
}
