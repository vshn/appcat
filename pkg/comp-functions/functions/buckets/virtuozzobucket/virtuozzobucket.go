package virtuozzobucket

import (
	"context"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/buckets/util"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	virtuozzov1 "github.com/vshn/provider-virtuozzo/apis/virtuozzo/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	bucketResName = "virtuozzo-bucket"
)

// ProvisionVirtuozzobucket will create a bucket in Virtuozzo.
func ProvisionVirtuozzobucket(_ context.Context, bucket *appcatv1.ObjectBucket, svc *runtime.ServiceRuntime) *xfnproto.Result {

	err := svc.GetObservedComposite(bucket)
	if err != nil {
		return runtime.NewFatalResult(err)
	}

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

	cd, err := svc.GetObservedComposedResourceConnectionDetails(bucketResName)
	if err != nil && err != runtime.ErrNotFound {
		return runtime.NewFatalResult(err)
	}

	for v, k := range cd {
		svc.SetConnectionDetail(v, k)
	}

	return nil
}

func addBucket(svc *runtime.ServiceRuntime, bucket *appcatv1.ObjectBucket, config string) error {

	mb := &virtuozzov1.Bucket{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				runtime.IgnoreConnectionDetailsAnnotation: "true",
			},
		},
		Spec: virtuozzov1.BucketSpec{
			ForProvider: virtuozzov1.BucketParameters{
				BucketDeletionPolicy: virtuozzov1.BucketDeletionPolicy(bucket.Spec.Parameters.BucketDeletionPolicy),
				BucketName:           bucket.GetBucketName(),
			},
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: config,
				},
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Namespace: svc.Config.Data["providerSecretNamespace"],
				},
			},
		},
	}

	objName := util.GetBucketObjectName(svc, bucket, bucketResName, mb.DeepCopy())

	mb.ObjectMeta.Name = objName
	mb.Spec.WriteConnectionSecretToReference.Name = objName

	return svc.SetDesiredComposedResourceWithName(mb, bucketResName)
}

func populateEndpointConnectionDetails(svc *runtime.ServiceRuntime) error {

	bucket := &virtuozzov1.Bucket{}

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
