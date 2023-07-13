package helper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// SaveFuncIOToConfigMap is a helper function to make it easy to determine the contents of the FunctionIO.
// This function is not a normall composition function! It won't add anything to the functionIO object to
// avoid it being bloated.
// It will keep two configmaps:
// previous generation functionIO
// current generation functionIO
// The config maps are always written to the default namespace and contain the
// composite name as the name.
func SaveFuncIOToConfigMap(funcName string, kubeClient client.Client) func(context.Context, *runtime.Runtime) runtime.Result {
	return func(ctx context.Context, iof *runtime.Runtime) runtime.Result {

		// missusing the xkube object a bit here...
		// I'm only interested in the metadata
		meta := &xkube.Object{}
		if err := iof.Desired.GetComposite(ctx, meta); err != nil {
			return runtime.NewFatalErr(ctx, "can't parse composite for functionIO configmap", err)
		}

		rawIO := iof.GetRawFuncIO()

		sort.Slice(rawIO.Desired.Resources, func(i, j int) bool {
			return rawIO.Desired.Resources[i].Name < rawIO.Desired.Resources[j].Name
		})

		for _, res := range rawIO.Desired.Resources {
			sort.Slice(res.ConnectionDetails, func(i, j int) bool {
				return *res.ConnectionDetails[i].Name < *res.ConnectionDetails[j].Name
			})
		}

		sort.Slice(rawIO.Observed.Resources, func(i, j int) bool {
			return rawIO.Observed.Resources[i].Name < rawIO.Observed.Resources[j].Name
		})

		for _, res := range rawIO.Observed.Resources {
			sort.Slice(res.ConnectionDetails, func(i, j int) bool {
				return res.ConnectionDetails[i].Name < res.ConnectionDetails[j].Name
			})
		}

		sort.Slice(rawIO.Desired.Composite.ConnectionDetails, func(i, j int) bool {
			return rawIO.Desired.Composite.ConnectionDetails[i].Name < rawIO.Desired.Composite.ConnectionDetails[j].Name
		})

		sort.Slice(rawIO.Observed.Composite.ConnectionDetails, func(i, j int) bool {
			return rawIO.Observed.Composite.ConnectionDetails[i].Name < rawIO.Observed.Composite.ConnectionDetails[j].Name
		})

		for i := range rawIO.Observed.Resources {
			err := removeMetadata(&rawIO.Observed.Resources[i].Resource)
			if err != nil {
				return runtime.NewFatalErr(ctx, "cannot cleanup observed resources", err)
			}
		}

		for i := range rawIO.Desired.Resources {
			err := removeMetadata(&rawIO.Desired.Resources[i].Resource)
			if err != nil {
				return runtime.NewFatalErr(ctx, "cannot cleanup desired resources", err)
			}
		}

		err := removeMetadata(&rawIO.Observed.Composite.Resource)
		if err != nil {
			return runtime.NewFatalErr(ctx, "cannote cleanup observed composite", err)
		}

		err = removeMetadata(&rawIO.Desired.Composite.Resource)
		if err != nil {
			return runtime.NewFatalErr(ctx, "cannot cleanup desired composite", err)
		}

		funcIO, err := json.Marshal(rawIO)
		if err != nil {
			return runtime.NewFatalErr(ctx, "cannot marshal functionIO to JSON", err)
		}

		currentIndex := meta.Generation % 2

		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-%d", meta.GetName(), funcName, currentIndex),
				Namespace: "default",
			},
			Data: map[string]string{
				"funcIO": string(funcIO),
			},
		}

		err = diffFuncIOs(ctx, currentIndex, configMap, kubeClient, funcName)
		if err != nil {
			return runtime.NewFatalErr(ctx, "cannot diff funcIO", err)
		}

		_, err = controllerutil.CreateOrUpdate(ctx, kubeClient, configMap, func() error {
			return nil
		})
		if err != nil {
			return runtime.NewFatalErr(ctx, "can't save configmap containing functionIO", err)
		}

		return runtime.NewNormal()
	}
}

func removeMetadata(res *k8sruntime.RawExtension) error {
	obj := map[string]interface{}{}
	err := json.Unmarshal(res.Raw, &obj)
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	// outer kubec objects
	unstructured.RemoveNestedField(obj, "metadata", "generation")
	unstructured.RemoveNestedField(obj, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(obj, "metadata", "annotations", "kubectl.kubernetes.io/last-applied-configuration")
	unstructured.RemoveNestedField(obj, "metadata", "managedFields")

	// inner observed objects
	unstructured.RemoveNestedField(obj, "status", "atProvider", "manifest", "metadata", "generation")
	unstructured.RemoveNestedField(obj, "status", "atProvider", "manifest", "metadata", "resourceVersion")
	unstructured.RemoveNestedField(obj, "status", "atProvider", "manifest", "metadata", "annotations", "kubectl.kubernetes.io/last-applied-configuration")
	unstructured.RemoveNestedField(obj, "status", "atProvider", "manifest", "metadata", "managedFields")

	// inner spec objects
	unstructured.RemoveNestedField(obj, "spec", "forProvider", "manifest", "metadata", "generation")
	unstructured.RemoveNestedField(obj, "spec", "forProvider", "manifest", "metadata", "resourceVersion")
	unstructured.RemoveNestedField(obj, "spec", "forProvider", "manifest", "metadata", "annotations", "kubectl.kubernetes.io/last-applied-configuration")
	unstructured.RemoveNestedField(obj, "spec", "forProvider", "manifest", "metadata", "managedFields")

	res.Raw, err = json.Marshal(&obj)
	if err != nil {
		return err
	}
	return err
}

func diffFuncIOs(ctx context.Context, currentIndex int64, currentMap *corev1.ConfigMap, kubeClient client.Client, funcName string) error {

	prevIndex := 0
	if currentIndex == 0 {
		prevIndex = 1
	}

	prevMap := &corev1.ConfigMap{}

	prevMapName := strings.TrimRight(currentMap.GetName(), fmt.Sprintf("%d", currentIndex)) + fmt.Sprintf("%d", prevIndex)

	prevMap.ObjectMeta.Name = prevMapName
	prevMap.ObjectMeta.Namespace = "default"

	err := kubeClient.Get(ctx, client.ObjectKeyFromObject(prevMap), prevMap)
	if err != nil && apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	prevPretty := bytes.Buffer{}
	err = json.Indent(&prevPretty, []byte(prevMap.Data["funcIO"]), "", "  ")
	if err != nil {
		return err
	}

	currentPretty := bytes.Buffer{}
	err = json.Indent(&currentPretty, []byte(currentMap.Data["funcIO"]), "", "  ")
	if err != nil {
		return err
	}

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(prevPretty.String()),
		B:        difflib.SplitLines(currentPretty.String()),
		FromFile: "Previous",
		ToFile:   "Current",
		Context:  3,
	}
	text, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return err
	}
	controllerruntime.LoggerFrom(ctx).V(1).Info("\n"+text, "functionName", funcName)

	return nil
}
