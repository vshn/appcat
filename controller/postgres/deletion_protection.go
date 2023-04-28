package postgres

import (
	"context"
	"encoding/json"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logging "sigs.k8s.io/controller-runtime/pkg/log"
	"strconv"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	finalizerName  = "appcat.io/deletionProtection"
	currentTimeKey = "now"
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
		log.Info("DeletionProtection is not enabled, ensuring no finalizer set", "objectName", inst.GetName())
		removed := controllerutil.RemoveFinalizer(inst, finalizerName)

		if removed {
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

func getRequeueTime(ctx context.Context, inst client.Object, deletionTime *v1.Time, retention int) time.Duration {
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
