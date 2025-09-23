package miniobucket

import (
	"context"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	miniov1 "github.com/vshn/provider-minio/apis/minio/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProvisionMiniobucket will create a bucket in a pre-deployed minio instance.
// This function will leverage provider-minio to deploy proper policies and users
// alongside the bucket.
func ProvisionMiniobucket(_ context.Context, bucket *appcatv1.ObjectBucket, svc *runtime.ServiceRuntime) *xfnproto.Result {

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

	err = addPolicy(svc, bucket, config)
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

	mb := &miniov1.Bucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: bucket.GetBucketName(),
		},
		Spec: miniov1.BucketSpec{
			ForProvider: miniov1.BucketParameters{
				BucketDeletionPolicy: miniov1.BucketDeletionPolicy(bucket.Spec.Parameters.BucketDeletionPolicy),
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

	user := &miniov1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: bucket.GetBucketName(),
			Annotations: map[string]string{
				runtime.IgnoreConnectionDetailsAnnotation: "true",
			},
		},
		Spec: miniov1.UserSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: config,
				},
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name:      bucket.GetName(),
					Namespace: svc.GetCrossplaneNamespace(),
				},
			},
			ForProvider: miniov1.UserParameters{
				Policies: []string{
					bucket.GetBucketName(),
				},
			},
		},
	}

	cd, err := svc.GetObservedComposedResourceConnectionDetails("minio-user")
	if err != nil && err != runtime.ErrNotFound {
		return err
	}

	for v, k := range cd {
		svc.SetConnectionDetail(v, k)
	}

	return svc.SetDesiredComposedResourceWithName(user, "minio-user")
}

func addPolicy(svc *runtime.ServiceRuntime, bucket *appcatv1.ObjectBucket, config string) error {

	policy := &miniov1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name: bucket.GetBucketName(),
		},
		Spec: miniov1.PolicySpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: config,
				},
			},
			ForProvider: miniov1.PolicyParameters{
				AllowBucket: bucket.GetBucketName(),
			},
		},
	}

	return svc.SetDesiredComposedResourceWithName(policy, "minio-policy")
}

func populateEndpointConnectionDetails(svc *runtime.ServiceRuntime) error {

	bucket := &miniov1.Bucket{}

	err := svc.GetObservedComposedResource(bucket, "minio-bucket")
	if err != nil && err == runtime.ErrNotFound {
		return nil
	} else if err != nil {
		return err
	}

	svc.SetConnectionDetail("ENDPOINT", []byte(bucket.Status.Endpoint))
	svc.SetConnectionDetail("ENDPOINT_URL", []byte(bucket.Status.EndpointURL))

	return nil

}
