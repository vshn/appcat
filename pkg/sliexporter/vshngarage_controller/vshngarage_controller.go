package vshngaragecontroller

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/sliexporter/probes"
	slireconciler "github.com/vshn/appcat/v4/pkg/sliexporter/sli_reconciler"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	vshnGarageServiceKey = "VSHNGarage"
	adminTokenSecret     = "garage-admin-token"
)

var garageBucketGVK = schema.GroupVersionKind{
	Group:   "garage.rajsingh.info",
	Version: "v1alpha1",
	Kind:    "GarageBucket",
}

var garageKeyGVK = schema.GroupVersionKind{
	Group:   "garage.rajsingh.info",
	Version: "v1alpha1",
	Kind:    "GarageKey",
}

type VSHNGarageReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	ProbeManager       probeManager
	StartupGracePeriod time.Duration
	GarageDialer       func(service, name, claimNamespace, instanceNamespace, organization, sla, compositionName, endpointURL, adminToken, bucketName string, ha bool, opts minio.Options) (*probes.VSHNGarage, error)
	ScClient           client.Client
}

type probeManager interface {
	slireconciler.ProbeManager
}

//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshngarages,verbs=get;list;watch
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=xvshngarages/status,verbs=get
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=vshngarages,verbs=get;list;watch
//+kubebuilder:rbac:groups=vshn.appcat.vshn.io,resources=vshngarages/status,verbs=get

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

//+kubebuilder:rbac:groups=garage.rajsingh.info,resources=garagebuckets,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups=garage.rajsingh.info,resources=garagekeys,verbs=get;list;watch;create;delete

func (r *VSHNGarageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx).WithValues("namespace", req.Namespace, "instance", req.Name)
	l.Info("Reconciling XVSHNGarage")

	inst := &vshnv1.XVSHNGarage{}
	if err := r.Get(ctx, req.NamespacedName, inst); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if inst.GetDeletionTimestamp() != nil {
		if err := r.cleanupSLIBucketCR(ctx, inst); err != nil {
			return ctrl.Result{}, err
		}
	}

	reconciler := slireconciler.New(inst, l, r.ProbeManager, vshnGarageServiceKey, req.NamespacedName, r.Client, r.StartupGracePeriod, r.getGarageProber, r.ScClient)
	return reconciler.Reconcile(ctx)
}

func (r VSHNGarageReconciler) getGarageProber(ctx context.Context, obj slireconciler.Service) (probes.Prober, error) {
	inst, ok := obj.(*vshnv1.XVSHNGarage)
	if !ok {
		return nil, fmt.Errorf("cannot start probe, object not a valid XVSHNGarage")
	}

	l := log.FromContext(ctx).WithValues(
		"namespace", inst.Labels[slireconciler.ClaimNamespaceLabel],
		"instance", inst.Labels[slireconciler.ClaimNameLabel],
	)

	instanceNamespace := inst.GetInstanceNamespace()
	if instanceNamespace == "" {
		return nil, fmt.Errorf("instance namespace not yet set for %s", inst.Name)
	}

	claimNamespace := inst.Labels[slireconciler.ClaimNamespaceLabel]
	claimName := inst.Labels[slireconciler.ClaimNameLabel]

	if err := r.ensureGarageBucketCR(ctx, claimNamespace, claimName, instanceNamespace); err != nil {
		return nil, fmt.Errorf("cannot ensure SLI bucket CR: %w", err)
	}
	if err := r.ensureGarageKeyCR(ctx, claimNamespace, claimName, instanceNamespace); err != nil {
		return nil, fmt.Errorf("cannot ensure SLI key CR: %w", err)
	}

	bucketName := sliGarageBucketCRName(claimName)
	l.Info("looking for bucket credentials secret", "name", bucketName, "namespace", claimNamespace)
	bucketSecret := corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: bucketName, Namespace: claimNamespace}, &bucketSecret); err != nil {
		return nil, err
	}

	l.Info("looking for secret: "+adminTokenSecret, "namespace", instanceNamespace)
	tokenSecret := corev1.Secret{}
	if err := r.ScClient.Get(ctx, types.NamespacedName{Name: adminTokenSecret, Namespace: instanceNamespace}, &tokenSecret); err != nil {
		return nil, err
	}

	ha := inst.Spec.Parameters.Instances > 1
	sla := inst.Spec.Parameters.Service.ServiceLevel
	compositionName := inst.Spec.CompositionRef.Name

	prober, err := r.GarageDialer(
		vshnGarageServiceKey,
		inst.Name,
		claimNamespace,
		instanceNamespace,
		inst.GetLabels()[utils.OrgLabelName],
		string(sla),
		compositionName,
		string(bucketSecret.Data["endpoint"]),
		string(tokenSecret.Data["token"]),
		bucketName,
		ha,
		minio.Options{
			Creds: credentials.NewStaticV4(
				string(bucketSecret.Data["access-key-id"]),
				string(bucketSecret.Data["secret-access-key"]),
				"",
			),
		},
	)
	if err != nil {
		l.Error(err, "Can't create Garage prober")
		return nil, err
	}
	return prober, nil
}

func (r *VSHNGarageReconciler) ensureGarageBucketCR(ctx context.Context, claimNamespace, claimName, instanceNamespace string) error {
	name := sliGarageBucketCRName(claimName)
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(garageBucketGVK)

	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: claimNamespace}, obj)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	obj = &unstructured.Unstructured{}
	obj.SetGroupVersionKind(garageBucketGVK)
	obj.SetName(name)
	obj.SetNamespace(claimNamespace)
	if err := unstructured.SetNestedField(obj.Object, claimName, "spec", "clusterRef", "name"); err != nil {
		return fmt.Errorf("cannot set clusterRef name: %w", err)
	}
	if err := unstructured.SetNestedField(obj.Object, instanceNamespace, "spec", "clusterRef", "namespace"); err != nil {
		return fmt.Errorf("cannot set clusterRef namespace: %w", err)
	}

	return r.Create(ctx, obj)
}

func (r *VSHNGarageReconciler) ensureGarageKeyCR(ctx context.Context, claimNamespace, claimName, instanceNamespace string) error {
	name := sliGarageBucketCRName(claimName)
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(garageKeyGVK)

	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: claimNamespace}, obj)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	obj = &unstructured.Unstructured{}
	obj.SetGroupVersionKind(garageKeyGVK)
	obj.SetName(name)
	obj.SetNamespace(claimNamespace)
	if err := unstructured.SetNestedField(obj.Object, claimName, "spec", "clusterRef", "name"); err != nil {
		return fmt.Errorf("cannot set clusterRef name: %w", err)
	}
	if err := unstructured.SetNestedField(obj.Object, instanceNamespace, "spec", "clusterRef", "namespace"); err != nil {
		return fmt.Errorf("cannot set clusterRef namespace: %w", err)
	}
	bucketPerms := []interface{}{
		map[string]interface{}{
			"bucketRef": name,
			"read":      true,
			"write":     true,
		},
	}
	if err := unstructured.SetNestedSlice(obj.Object, bucketPerms, "spec", "bucketPermissions"); err != nil {
		return fmt.Errorf("cannot set bucketPermissions: %w", err)
	}

	return r.Create(ctx, obj)
}

func (r *VSHNGarageReconciler) cleanupSLIBucketCR(ctx context.Context, inst *vshnv1.XVSHNGarage) error {
	claimName := inst.Labels[slireconciler.ClaimNameLabel]
	claimNamespace := inst.Labels[slireconciler.ClaimNamespaceLabel]
	if claimName == "" || claimNamespace == "" {
		return nil
	}

	name := sliGarageBucketCRName(claimName)

	bucketObj := &unstructured.Unstructured{}
	bucketObj.SetGroupVersionKind(garageBucketGVK)
	bucketObj.SetName(name)
	bucketObj.SetNamespace(claimNamespace)
	if err := r.Delete(ctx, bucketObj); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete SLI GarageBucket CR: %w", err)
	}

	keyObj := &unstructured.Unstructured{}
	keyObj.SetGroupVersionKind(garageKeyGVK)
	keyObj.SetName(name)
	keyObj.SetNamespace(claimNamespace)
	if err := r.Delete(ctx, keyObj); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete SLI GarageKey CR: %w", err)
	}

	return nil
}

func sliGarageBucketCRName(claimName string) string {
	name := "vshn-sli-probe-" + claimName
	if len(name) > 63 {
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(name)))
		name = name[:57] + "-" + hash[:5]
	}
	return name
}

func (r *VSHNGarageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vshnv1.XVSHNGarage{}).
		Complete(r)
}
