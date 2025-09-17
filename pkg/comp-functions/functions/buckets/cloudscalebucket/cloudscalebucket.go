package cloudscalebucket

import (
	"context"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/buckets/util"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	cloudscalev1 "github.com/vshn/provider-cloudscale/apis/cloudscale/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	bucketResName = "cloudscale-bucket"
	userResName   = "cloudscale-user"
)

// ProvisionCloudscalebucket will create a bucket in cloudscale.
// This function will leverage provider-cloudscale to deploy proper users
// alongside the bucket.
func ProvisionCloudscalebucket(_ context.Context, bucket *appcatv1.ObjectBucket, svc *runtime.ServiceRuntime) *xfnproto.Result {

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

	svc.SetConnectionDetail("BUCKET_NAME", []byte(bucket.GetBucketName()))
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
				BucketName:           bucket.GetBucketName(),
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

	objName := util.GetBucketObjectName(svc, bucket, bucketResName, mb.DeepCopy())

	mb.ObjectMeta.Name = objName
	mb.Spec.ForProvider.CredentialsSecretRef.Name = objName

	return svc.SetDesiredComposedResourceWithName(mb, bucketResName)
}

func addUser(svc *runtime.ServiceRuntime, bucket *appcatv1.ObjectBucket, config string) error {

	user := &cloudscalev1.ObjectsUser{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				runtime.IgnoreConnectionDetailsAnnotation: "true",
			},
		},
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

	objName := util.GetBucketObjectName(svc, bucket, userResName, user.DeepCopy())

	user.ObjectMeta.Name = objName
	user.Spec.WriteConnectionSecretToReference.Name = objName

	cd, err := svc.GetObservedComposedResourceConnectionDetails(userResName)
	if err != nil && err != runtime.ErrNotFound {
		return err
	}

	for v, k := range cd {
		svc.SetConnectionDetail(v, k)
	}

	return svc.SetDesiredComposedResourceWithName(user, userResName)
}

func populateEndpointConnectionDetails(svc *runtime.ServiceRuntime) error {

	bucket := &cloudscalev1.Bucket{}

	err := svc.GetObservedComposedResource(bucket, bucketResName)
	if err != nil && err == runtime.ErrNotFound {
		return nil
	} else if err != nil {
		return err
	}

	svc.SetConnectionDetail("ENDPOINT", []byte(bucket.Status.Endpoint))
	svc.SetConnectionDetail("ENDPOINT_URL", []byte(bucket.Status.EndpointURL))

	return nil

}
