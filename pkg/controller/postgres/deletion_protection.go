package postgres

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"

	logging "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/go-logr/logr"
	"github.com/vshn/appcat/v4/pkg/common/jsonpatch"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	finalizerName = "appcat.io/deletionProtection"
)

func handle(ctx context.Context, inst client.Object) (client.Patch, error) {
	log := logging.FromContext(ctx, "namespace", inst.GetNamespace(), "instance", inst.GetName())
	op := jsonpatch.JSONopNone

	removed := controllerutil.RemoveFinalizer(inst, finalizerName)

	if removed {
		log.Info("Ensuring deprecated finalizer is not set", "objectName", inst.GetName())
		op = jsonpatch.JSONopRemove
	}

	return getPatchObjectFinalizer(log, inst, op)
}

func getPatchObjectFinalizer(log logr.Logger, inst client.Object, op jsonpatch.JSONop) (client.Patch, error) {
	// handle the case if crossplane or something else decides to add more finalizers, or if
	// the finalizer is already there.
	index := len(inst.GetFinalizers())
	dupes := []int{}
	for i, finalizer := range inst.GetFinalizers() {
		if finalizer == finalizerName {
			index = i
			dupes = append(dupes, i)
		}
	}

	// if this is a noop and we have exactly one or no dupe, then we're done here
	if op == jsonpatch.JSONopNone && (len(dupes) == 1 || len(dupes) == 0) {
		return nil, nil
	}

	log.V(1).Info("Index size", "size", index, "found finalizers", inst.GetFinalizers())

	strIndex := strconv.Itoa(index)
	if op == jsonpatch.JSONopAdd {
		strIndex = "-"
	}

	patchOps := []jsonpatch.JSONpatch{}
	if op != jsonpatch.JSONopNone {
		patchOps = append(patchOps, jsonpatch.JSONpatch{
			Op:    op,
			Path:  "/metadata/finalizers/" + strIndex,
			Value: finalizerName,
		})
	}

	// if we have more than one of our finalizers, we need to remove the excess ones
	if len(dupes) > 1 {
		// Jsonpatch doesn't specify how deletion of multiple indices in an array is handled.
		// When starting with the lowest first, it could be that the indices all shift and not match anymore.
		// We reverse sort, so that the patch contains the largest index first.
		// This way we avoid any index shifting during the deletion and are on the safe side.
		sort.Sort(sort.Reverse(sort.IntSlice(dupes)))
		for _, v := range dupes {
			// We skip the first one, so we don't remove all of them
			if v == 0 {
				continue
			}
			patchOps = append(patchOps, jsonpatch.JSONpatch{
				Op:   jsonpatch.JSONopRemove,
				Path: "/metadata/finalizers/" + strconv.Itoa(v),
			})
		}
	}

	patch, err := json.Marshal(patchOps)
	if err != nil {
		return nil, errors.Wrap(err, "can't marshal patch")
	}

	log.V(1).Info("Patching object", "patch", string(patch))

	return client.RawPatch(types.JSONPatchType, patch), nil
}
