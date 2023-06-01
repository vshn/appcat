package miniodev

import (
	"context"

	crossplanev1 "github.com/crossplane/crossplane/apis/apiextensions/fn/io/v1alpha1"
	appcatv1 "github.com/vshn/appcat/apis/v1"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProvisionMiniobucket will create a bucket in the
func ProvisionMiniobucket(ctx context.Context, iof *runtime.Runtime) runtime.Result {
	claim := &appcatv1.ObjectBucket{}

	err := iof.Observed.GetComposite(ctx, claim)
	if err != nil {
		return runtime.NewFatalErr(ctx, "Could not get ObjectBucket claim", err)
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "create-bucket-" + claim.GetLabels()["crossplane.io/composite"],
			Namespace: "minio",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:  "create-bucket",
							Image: "minio/mc",
							Command: []string{
								"/bin/sh",
								"-c",
							},
							Args: []string{
								"mc alias set minio http://minio-server.minio.svc:9000 minioadmin minioadmin && mc mb minio/" + claim.Spec.Parameters.BucketName,
							},
						},
					},
				},
			},
		},
	}

	err = iof.Desired.PutIntoObject(ctx, job, claim.GetLabels()["crossplane.io/composite"]+"-create-bucket")
	if err != nil {
		return runtime.NewFatal(ctx, "Can't add bucketjob to desired: "+err.Error())
	}

	iof.Desired.PutCompositeConnectionDetail(ctx, crossplanev1.ExplicitConnectionDetail{
		Name:  "AWS_SECRET_ACCESS_KEY",
		Value: "minioadmin",
	})

	iof.Desired.PutCompositeConnectionDetail(ctx, crossplanev1.ExplicitConnectionDetail{
		Name:  "ENDPOINT_URL",
		Value: "http://minio-server.minio.svc:9000",
	})

	iof.Desired.PutCompositeConnectionDetail(ctx, crossplanev1.ExplicitConnectionDetail{
		Name:  "AWS_ACCESS_KEY_ID",
		Value: "minioadmin",
	})

	iof.Desired.PutCompositeConnectionDetail(ctx, crossplanev1.ExplicitConnectionDetail{
		Name:  "BUCKET_NAME",
		Value: claim.Spec.Parameters.BucketName,
	})

	return runtime.NewNormal()
}
