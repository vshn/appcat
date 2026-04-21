package garagebucket

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/go-logr/logr"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

//+kubebuilder:rbac:groups=garage.rajsingh.info,resources=garagebuckets,verbs=get;list;watch
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=vshngarages,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

const adminCredentialsSecret = "admin-s3-credentials"

var garageBucketGVK = schema.GroupVersionKind{
	Group:   "garage.rajsingh.info",
	Version: "v1alpha1",
	Kind:    "GarageBucket",
}

// CleanupReconciler watches GarageBucket resources and empties the underlying
// S3 bucket when deletion is requested, using dedicated admin credentials.
type CleanupReconciler struct {
	client.Client
	log           logr.Logger
	emptyBucketFn func(ctx context.Context, secret *corev1.Secret, bucketName string, log logr.Logger) error
}

// NewCleanupReconciler creates a new CleanupReconciler.
func NewCleanupReconciler(c client.Client, log logr.Logger) *CleanupReconciler {
	r := &CleanupReconciler{
		Client: c,
		log:    log.WithName("controller").WithName("garagebucket-cleanup"),
	}
	r.emptyBucketFn = r.emptyBucket
	return r
}

// SetupWithManager registers the controller with the manager.
func (r *CleanupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(garageBucketGVK)

	inDeletion := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool { return e.Object.GetDeletionTimestamp() != nil },
		UpdateFunc: func(event event.UpdateEvent) bool {
			oldDel := event.ObjectOld.GetDeletionTimestamp() != nil
			newDel := event.ObjectNew.GetDeletionTimestamp() != nil
			return !oldDel && newDel
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(u).
		WithEventFilter(inDeletion).
		Complete(r)
}

// Reconcile empties the S3 bucket when the GarageBucket is marked for deletion.
func (r *CleanupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.log.WithValues("garagebucket", req.NamespacedName)

	garageBucket := &unstructured.Unstructured{}
	garageBucket.SetGroupVersionKind(garageBucketGVK)
	if err := r.Get(ctx, req.NamespacedName, garageBucket); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Safeguard just in case
	if garageBucket.GetDeletionTimestamp() == nil {
		return ctrl.Result{}, nil
	}

	bucketName := garageBucket.GetName()
	log = log.WithValues("bucket", bucketName)

	clusterRefName, found, err := unstructured.NestedString(garageBucket.Object, "spec", "clusterRef", "name")
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot read spec.clusterRef.name from GarageBucket: %w", err)
	}
	if !found || clusterRefName == "" {
		return ctrl.Result{}, fmt.Errorf("GarageBucket %q has no spec.clusterRef.name", garageBucket.GetName())
	}
	vshnGarage := &vshnv1.VSHNGarage{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: garageBucket.GetNamespace(), Name: clusterRefName}, vshnGarage); err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot get VSHNGarage %q: %w", clusterRefName, err)
	}
	instanceNamespace := vshnGarage.Status.InstanceNamespace
	log = log.WithValues("instanceNamespace", instanceNamespace)

	adminSecret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: instanceNamespace, Name: adminCredentialsSecret}, adminSecret); err != nil {
		return ctrl.Result{}, fmt.Errorf("cannot get admin credentials secret: %w", err)
	}

	log.Info("GarageBucket marked for deletion, emptying bucket")
	if err := r.emptyBucketFn(ctx, adminSecret, bucketName, log); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Successfully emptied Garage bucket")
	return ctrl.Result{}, nil
}

func (r *CleanupReconciler) emptyBucket(ctx context.Context, secret *corev1.Secret, bucketName string, log logr.Logger) error {
	requiredKeys := []string{"endpoint", "access-key-id", "secret-access-key"}
	for _, key := range requiredKeys {
		if len(secret.Data[key]) == 0 {
			return fmt.Errorf("admin credentials secret %q missing required key %q", secret.Name, key)
		}
	}

	endpoint := string(secret.Data["endpoint"])
	accessKeyID := string(secret.Data["access-key-id"])
	secretAccessKey := string(secret.Data["secret-access-key"])

	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("cannot parse endpoint %q: %w", endpoint, err)
	}
	if u.Host == "" {
		return fmt.Errorf("endpoint %q is invalid: missing host (add scheme, e.g. https://)", endpoint)
	}
	log = log.WithValues("endpoint", u.Host)
	log.V(1).Info("Connecting to S3 endpoint")

	s3Client, err := minio.New(u.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: u.Scheme == "https",
	})
	if err != nil {
		return fmt.Errorf("cannot create S3 client: %w", err)
	}

	log.Info("Removing all objects from bucket")
	rawObjects := s3Client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{Recursive: true})

	// Intercept listing errors before feeding into RemoveObjects.
	// ObjectInfo.Err is set when the S3 listing itself fails mid-stream.
	// listErr is written by the goroutine before close(filtered), which
	// happens-before RemoveObjects drains, so the read below is race-free.
	filtered := make(chan minio.ObjectInfo)
	var listErr error
	go func() {
		defer close(filtered)
		for obj := range rawObjects {
			if obj.Err != nil {
				listErr = fmt.Errorf("error listing objects in bucket %q: %w", bucketName, obj.Err)
				return
			}
			filtered <- obj
		}
	}()

	removeErrors := s3Client.RemoveObjects(ctx, bucketName, filtered, minio.RemoveObjectsOptions{})

	var errs []error
	for removeErr := range removeErrors {
		log.Error(removeErr.Err, "Failed to remove object", "object", removeErr.ObjectName)
		errs = append(errs, fmt.Errorf("cannot remove object %q: %w", removeErr.ObjectName, removeErr.Err))
	}
	if listErr != nil {
		errs = append(errs, listErr)
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors removing objects from bucket %q: %w", bucketName, errors.Join(errs...))
	}
	return nil
}
