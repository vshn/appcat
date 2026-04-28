package genericbucket

import (
	"context"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	s3v1 "github.com/vshn/appcat/v4/apis/s3/v1"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/buckets/util"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	bucketResName = "generic-bucket"
)

func ProvisionGenericBucket(_ context.Context, bucket *appcatv1.ObjectBucket, svc *runtime.ServiceRuntime) *xfnproto.Result {
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

	svc.SetConnectionDetail("BUCKET_NAME", []byte(bucket.GetBucketName()))
	svc.SetConnectionDetail("AWS_REGION", []byte(bucket.Spec.Parameters.Region))

	err = populateEndpointConnectionDetails(svc)
	if err != nil {
		return runtime.NewFatalResult(err)
	}

	return nil
}

func addBucket(svc *runtime.ServiceRuntime, bucket *appcatv1.ObjectBucket, config string) error {

	if svc.Config.Data["providerSecretNamespace"] == "" {
		return fmt.Errorf("input providerSecretNamespace not set, aborting bucket provisioning")
	}

	mb := &s3v1.Bucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: bucket.GetBucketName(),
			Annotations: map[string]string{
				runtime.IgnoreConnectionDetailsAnnotation: "true",
			},
		},
		Spec: s3v1.BucketSpec{
			ForProvider: s3v1.BucketParameters{
				BucketDeletionPolicy: s3v1.BucketDeletionPolicy(bucket.Spec.Parameters.BucketDeletionPolicy),
				Region:               bucket.Spec.Parameters.Region,
			},
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: config,
				},
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Namespace: svc.Config.Data["providerSecretNamespace"],
					Name:      bucket.GetBucketName(),
				},
			},
		},
	}

	objName := util.GetBucketObjectName(svc, bucket, bucketResName, mb.DeepCopy())

	mb.Name = objName

	cd, err := svc.GetObservedComposedResourceConnectionDetails(bucketResName)
	if err != nil && err != runtime.ErrNotFound {
		return err
	}

	for v, k := range cd {
		svc.SetConnectionDetail(v, k)
	}

	return svc.SetDesiredComposedResourceWithName(mb, bucketResName)
}

func populateEndpointConnectionDetails(svc *runtime.ServiceRuntime) error {

	bucket := &s3v1.Bucket{}

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
