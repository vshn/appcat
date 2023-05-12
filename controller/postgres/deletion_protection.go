package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logging "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	finalizerName = "appcat.io/deletionProtection"
)

type jsonOp string

const (
	opRemove  jsonOp = "remove"
	opAdd     jsonOp = "add"
	opNone    jsonOp = "none"
	opReplace jsonOp = "replace"
)

type jsonpatch struct {
	Op    jsonOp `json:"op,omitempty"`
	Path  string `json:"path,omitempty"`
	Value string `json:"value,omitempty"`
}

func handle(ctx context.Context, inst client.Object, enabled bool, retention int) (client.Patch, error) {
	log := logging.FromContext(ctx, "namespace", inst.GetNamespace(), "instance", inst.GetName())
	op := opNone

	if !enabled {
		removed := controllerutil.RemoveFinalizer(inst, finalizerName)

		if removed {
			log.Info("DeletionProtection is not enabled, ensuring no finalizer set", "objectName", inst.GetName())
			op = opRemove
		}

		return getPatchObjectFinalizer(log, inst, op)
	}

	if !controllerutil.ContainsFinalizer(inst, finalizerName) && inst.GetDeletionTimestamp() == nil {
		added := controllerutil.AddFinalizer(inst, finalizerName)
		if added {
			log.Info("Added finalizer to the object", "objectName", inst.GetName())
			op = opAdd
			return getPatchObjectFinalizer(log, inst, op)
		}
	}

	if inst.GetDeletionTimestamp() != nil {
		op = checkRetention(ctx, inst, retention)
	}

	return getPatchObjectFinalizer(log, inst, op)
}

func checkRetention(ctx context.Context, inst client.Object, retention int) jsonOp {
	log := logging.FromContext(ctx, "namespace", inst.GetNamespace(), "instance", inst.GetName())
	timestamp := inst.GetDeletionTimestamp()
	expireDate := timestamp.AddDate(0, 0, retention)
	op := opNone
	now := getCurrentTime()
	if now.After(expireDate) {
		log.Info("Retention expired, removing finalizer")
		removed := controllerutil.RemoveFinalizer(inst, finalizerName)
		if removed {
			op = opRemove
		}
	}
	return op
}

func getRequeueTime(ctx context.Context, inst client.Object, deletionTime *metav1.Time, retention int) time.Duration {
	log := logging.FromContext(ctx, "namespace", inst.GetNamespace(), "instance", inst.GetName())
	now := getCurrentTime()
	if deletionTime != nil {
		deletionIn := deletionTime.AddDate(0, 0, retention).Sub(now)
		log.V(1).Info("Deletion in: " + deletionIn.String())
		return deletionIn
	}
	return time.Second * 30
}

func getPatchObjectFinalizer(log logr.Logger, inst client.Object, op jsonOp) (client.Patch, error) {
	if op == opNone {
		return nil, nil
	}

	// handle the case if crossplane or something else decides to add more finalizers, or if
	// the finalizer is already there.
	index := len(inst.GetFinalizers())
	for i, finalizer := range inst.GetFinalizers() {
		if finalizer == finalizerName {
			index = i
		}
	}

	log.V(1).Info("Index size", "size", index, "found finalizers", inst.GetFinalizers())

	patchOps := []jsonpatch{
		{
			Op:    op,
			Path:  "/metadata/finalizers/" + strconv.Itoa(index),
			Value: finalizerName,
		},
	}

	patch, err := json.Marshal(patchOps)
	if err != nil {
		return nil, errors.Wrap(err, "can't marshal patch")
	}

	log.V(1).Info("Patching object", "patch", string(patch))

	return client.RawPatch(types.JSONPatchType, patch), nil
}

func getCurrentTime() time.Time {
	t := time.Now()
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, t.Location())
}

func getPostgreSQLNamespace(inst client.Object) string {
	return fmt.Sprintf("vshn-postgresql-%s", inst.GetName())
}

// instanceNamespaceDeleted handles the case, if the instance namespace gets deleted.
// The customer can't delete the namespace by themselves, so this is usally when the customer as a whole gets deleted.
// Or some other administrative action.
// In those cases we should disable the deletionprotection.
// If the namespace is deleted or not found it will return a patch to remove the finalizer.
func instanceNamespaceDeleted(ctx context.Context, log logr.Logger, inst client.Object, enabled bool, c client.Client) (jsonOp, error) {
	ns := &corev1.Namespace{}
	err := c.Get(ctx, client.ObjectKey{Name: getPostgreSQLNamespace(inst)}, ns)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Instance namespace was not found, ignoring")
			return opNone, nil
		}
		return opNone, err
	}

	if ns.DeletionTimestamp != nil && controllerutil.RemoveFinalizer(ns, finalizerName) {
		log.Info("Instance namespace was deleted, overriding deletionprotection")
		return opRemove, c.Update(ctx, ns)
	}

	if enabled && controllerutil.AddFinalizer(ns, finalizerName) {
		log.Info("Deletion protection enabled, protecting instance namespace")
		err := controllerutil.SetControllerReference(inst, ns, c.Scheme())
		if err != nil {
			return opNone, err
		}
		return opNone, c.Update(ctx, ns)
	}

	if !enabled && controllerutil.RemoveFinalizer(ns, finalizerName) {
		log.Info("Deletion protection disabled, removing protection from instance namespace")
		return opRemove, c.Update(ctx, ns)
	}

	return opNone, nil
}

func getInstanceNamespaceOverride(ctx context.Context, inst client.Object, enabled bool, c client.Client) (client.Patch, error) {
	log := logging.FromContext(ctx, "namespace", inst.GetNamespace(), "instance", inst.GetName())

	overrideOp, err := instanceNamespaceDeleted(ctx, log, inst, enabled, c)
	if err != nil {
		return nil, errors.Wrap(err, "could not determine instance namespace status")
	}

	patch, err := getPatchObjectFinalizer(log, inst, overrideOp)
	if err != nil {
		return nil, errors.Wrap(err, "can't create namespace override patch")
	}
	return patch, nil
}
