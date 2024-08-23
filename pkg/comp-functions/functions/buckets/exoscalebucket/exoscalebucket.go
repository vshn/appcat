package exoscalebucket

import (
	"context"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/buckets/util"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	exoscalev1 "github.com/vshn/provider-exoscale/apis/exoscale/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	bucketResName = "exoscale-bucket"
	iamResName    = "exoscale-iam"
)

// ProvisionExoscalebucket will create a bucket in cloudscale.
// This function will leverage provider-cloudscale to deploy proper users
// alongside the bucket.
func ProvisionExoscalebucket(_ context.Context, bucket *appcatv1.ObjectBucket, svc *runtime.ServiceRuntime) *xfnproto.Result {

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

	mb := &exoscalev1.Bucket{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: exoscalev1.BucketSpec{
			ForProvider: exoscalev1.BucketParameters{
				BucketDeletionPolicy: exoscalev1.BucketDeletionPolicy(bucket.Spec.Parameters.BucketDeletionPolicy),
				Zone:                 bucket.Spec.Parameters.Region,
				BucketName:           bucket.Spec.Parameters.BucketName,
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

	return svc.SetDesiredComposedResourceWithName(mb, bucketResName)
}

func addUser(svc *runtime.ServiceRuntime, bucket *appcatv1.ObjectBucket, config string) error {

	user := &exoscalev1.IAMKey{
		ObjectMeta: metav1.ObjectMeta{},
		Spec: exoscalev1.IAMKeySpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: config,
				},
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Namespace: svc.Config.Data["providerSecretNamespace"],
				},
			},
			ForProvider: exoscalev1.IAMKeyParameters{
				Zone:    bucket.Spec.Parameters.Region,
				KeyName: fmt.Sprintf("%s.%s", bucket.GetLabels()["crossplane.io/claim-namespace"], bucket.GetLabels()["crossplane.io/claim-name"]),
				Services: exoscalev1.ServicesSpec{
					SOS: exoscalev1.SOSSpec{
						Buckets: []string{
							bucket.Spec.Parameters.BucketName,
						},
					},
				},
			},
		},
	}

	objName := util.GetBucketObjectName(svc, bucket, iamResName, user.DeepCopy())

	user.ObjectMeta.Name = objName
	user.Spec.WriteConnectionSecretToReference.Name = objName

	cd, err := svc.GetObservedComposedResourceConnectionDetails(iamResName)
	if err != nil && err != runtime.ErrNotFound {
		return err
	}

	for v, k := range cd {
		svc.SetConnectionDetail(v, k)
	}

	return svc.SetDesiredComposedResourceWithName(user, iamResName)
}

func populateEndpointConnectionDetails(svc *runtime.ServiceRuntime) error {

	bucket := &exoscalev1.Bucket{}

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
