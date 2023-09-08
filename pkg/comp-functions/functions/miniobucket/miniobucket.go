package miniobucket

import (
	"context"
	"fmt"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	xfnv1alpha1 "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	miniov1 "github.com/vshn/provider-minio/apis/minio/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

const (
	accessKeyName = "AWS_ACCESS_KEY_ID"
	secretKeyName = "AWS_SECRET_ACCESS_KEY"
)

// ProvisionMiniobucket will create a bucket in a pre-deployed minio instance.
// This function will leverage provider-minio to deploy proper policies and users
// alongside the bucket.
func ProvisionMiniobucket(ctx context.Context, iof *runtime.Runtime) runtime.Result {

	bucket := &appcatv1.ObjectBucket{}

	err := iof.Desired.GetComposite(ctx, bucket)
	if err != nil {
		return runtime.NewFatalErr(ctx, "Could not get ObjectBucket claim", err)
	}

	config, ok := iof.Config.Data["providerConfig"]
	if !ok {
		return runtime.NewFatalErr(ctx, "no providerConfig specified in composition config", fmt.Errorf("no providerConfig specified"))
	}

	err = addBucket(ctx, iof, bucket, config)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot apply bucket", err)
	}

	err = addPolicy(ctx, iof, bucket, config)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot apply policy", err)
	}

	err = addUser(ctx, iof, bucket, config)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot apply user", err)
	}

	iof.Desired.PutCompositeConnectionDetail(ctx, xfnv1alpha1.ExplicitConnectionDetail{
		Name:  "BUCKET_NAME",
		Value: bucket.Spec.Parameters.BucketName,
	})

	iof.Desired.PutCompositeConnectionDetail(ctx, xfnv1alpha1.ExplicitConnectionDetail{
		Name:  "AWS_REGION",
		Value: bucket.Spec.Parameters.Region,
	})

	err = populateEndpointConnectionDetails(ctx, iof)
	if err != nil {
		return runtime.NewFatalErr(ctx, "cannot populate connectionDetails", err)
	}

	return runtime.NewNormal()
}

func addBucket(ctx context.Context, iof *runtime.Runtime, bucket *appcatv1.ObjectBucket, config string) error {

	mb := &miniov1.Bucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: bucket.Spec.Parameters.BucketName,
		},
		Spec: miniov1.BucketSpec{
			ForProvider: miniov1.BucketParameters{
				BucketDeletionPolicy: miniov1.DeleteAll,
			},
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: config,
				},
			},
		},
	}

	return iof.Desired.PutWithResourceName(ctx, mb, "minio-bucket")
}

func addUser(ctx context.Context, iof *runtime.Runtime, bucket *appcatv1.ObjectBucket, config string) error {

	user := &miniov1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: bucket.Spec.Parameters.BucketName,
		},
		Spec: miniov1.UserSpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: config,
				},
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name:      bucket.GetName(),
					Namespace: "syn-crossplane",
				},
			},
			ForProvider: miniov1.UserParameters{
				Policies: []string{
					bucket.Spec.Parameters.BucketName,
				},
			},
		},
	}

	cd := []xfnv1alpha1.DerivedConnectionDetail{
		{
			Name:                    pointer.String(secretKeyName),
			FromConnectionSecretKey: pointer.String(secretKeyName),
			Type:                    xfnv1alpha1.ConnectionDetailTypeFromConnectionSecretKey,
		},
		{
			Name:                    pointer.String(accessKeyName),
			FromConnectionSecretKey: pointer.String(accessKeyName),
			Type:                    xfnv1alpha1.ConnectionDetailTypeFromConnectionSecretKey,
		},
	}

	return iof.Desired.PutWithResourceName(ctx, user, "minio-user", runtime.AddDerivedConnectionDetails(cd))
}

func addPolicy(ctx context.Context, iof *runtime.Runtime, bucket *appcatv1.ObjectBucket, config string) error {

	policy := &miniov1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name: bucket.Spec.Parameters.BucketName,
		},
		Spec: miniov1.PolicySpec{
			ResourceSpec: xpv1.ResourceSpec{
				ProviderConfigReference: &xpv1.Reference{
					Name: config,
				},
			},
			ForProvider: miniov1.PolicyParameters{
				AllowBucket: bucket.Spec.Parameters.BucketName,
			},
		},
	}

	return iof.Desired.PutWithResourceName(ctx, policy, "minio-policy")
}

func populateEndpointConnectionDetails(ctx context.Context, iof *runtime.Runtime) error {

	bucket := &miniov1.Bucket{}

	err := iof.Observed.Get(ctx, bucket, "minio-bucket")
	if err != nil && err == runtime.ErrNotFound {
		return nil
	} else if err != nil {
		return err
	}

	iof.Desired.PutCompositeConnectionDetail(ctx, xfnv1alpha1.ExplicitConnectionDetail{
		Name:  "ENDPOINT",
		Value: bucket.Status.Endpoint,
	})

	iof.Desired.PutCompositeConnectionDetail(ctx, xfnv1alpha1.ExplicitConnectionDetail{
		Name:  "ENDPOINT_URL",
		Value: bucket.Status.EndpointURL,
	})

	return nil

}
