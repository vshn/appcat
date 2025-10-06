package postgres

import (
	"context"
	"fmt"
	"strings"

	v1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	client "k8s.io/client-go/dynamic"
)

// +kubebuilder:rbac:groups="stackgres.io",resources=sgbackups,verbs=get;list;watch
// +kubebuilder:rbac:groups="postgresql.cnpg.io",resources=backups,verbs=get;list;watch

var (
	SGbackupGroupVersionResource = schema.GroupVersionResource{
		Group:    "stackgres.io",
		Version:  "v1",
		Resource: "sgbackups",
	}
	CNPGbackupGroupVersionResource = schema.GroupVersionResource{
		Group:    "postgresql.cnpg.io",
		Version:  "v1",
		Resource: "backups",
	}
)

// backupProvider is an abstraction to interact with the K8s API
type backupProvider interface {
	GetBackup(ctx context.Context, name, namespace string, schema schema.GroupVersionResource, client *client.DynamicClient) (*v1.BackupInfo, error)
	ListBackup(ctx context.Context, namespace string, schema schema.GroupVersionResource, client *client.DynamicClient, options *metainternalversion.ListOptions) (*[]v1.BackupInfo, error)
	WatchBackup(ctx context.Context, namespace string, options *metainternalversion.ListOptions) (watch.Interface, error)
}

type KubeBackupProvider struct {
	DynamicClient client.NamespaceableResourceInterface
}

// GetBackup fetches a SG/CNPG backup resource into unstructured.Unstructured. Relevant data is saved to v1.BackupInfo
func (k *KubeBackupProvider) GetBackup(ctx context.Context, name, namespace string, schema schema.GroupVersionResource, scClient *client.DynamicClient) (*v1.BackupInfo, error) {
	var unstructuredObject *unstructured.Unstructured
	var err error
	if scClient != nil {
		dc := scClient.Resource(schema)
		unstructuredObject, err = dc.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		unstructuredObject, err = k.DynamicClient.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, err
	}
	return convertToBackupInfo(unstructuredObject)
}

// ListBackup fetches SG/CNPG resources into unstructured.UnstructuredList. Relevant data is saved to v1.BackupInfo
func (k *KubeBackupProvider) ListBackup(ctx context.Context, namespace string, schema schema.GroupVersionResource, scClient *client.DynamicClient, options *metainternalversion.ListOptions) (*[]v1.BackupInfo, error) {
	var unstructuredList *unstructured.UnstructuredList
	var err error
	if scClient != nil {
		dc := scClient.Resource(schema)
		unstructuredList, err = dc.Namespace(namespace).List(ctx, metav1.ListOptions{
			Limit:    options.Limit,
			Continue: options.Continue,
		})
	} else {
		unstructuredList, err = k.DynamicClient.Namespace(namespace).List(ctx, metav1.ListOptions{
			Limit:    options.Limit,
			Continue: options.Continue,
		})
	}
	if err != nil {
		return nil, err
	}
	backupInfos := make([]v1.BackupInfo, 0, len(unstructuredList.Items))
	for _, v := range unstructuredList.Items {
		backupsInfo, err := convertToBackupInfo(&v)
		if err != nil {
			continue
		}
		backupInfos = append(backupInfos, *backupsInfo)
	}
	return &backupInfos, nil
}

// WatchBackup watches SG/CNPG backup resources.
func (k *KubeBackupProvider) WatchBackup(ctx context.Context, namespace string, options *metainternalversion.ListOptions) (watch.Interface, error) {
	return k.DynamicClient.Namespace(namespace).Watch(ctx, metav1.ListOptions{
		TypeMeta:      options.TypeMeta,
		LabelSelector: options.LabelSelector.String(),
		FieldSelector: options.FieldSelector.String(),
		Limit:         options.Limit,
		Continue:      options.Continue,
	})
}

// GetFromEvent resolves watch.Event into v1.BackupInfo
func GetFromEvent(in watch.Event) *v1.BackupInfo {
	u, ok := in.Object.(*unstructured.Unstructured)
	if !ok {
		return nil
	}

	backup, err := convertToBackupInfo(u)
	if err != nil {
		return nil
	}
	return backup
}

// Converts an unstructured.Unstructured into a v1.BackupInfo
func convertToBackupInfo(object *unstructured.Unstructured) (*v1.BackupInfo, error) {
	content := object.UnstructuredContent()
	objectMeta, exists, err := unstructured.NestedMap(content, v1.Metadata)
	if err != nil || !exists {
		return nil, fmt.Errorf("cannot parse metadata from object %s", object)
	}

	name := object.GetName()

	o := &metav1.ObjectMeta{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(objectMeta, o)
	if err != nil {
		return nil, err
	}

	// oject.GetKind() will be the singular form, but schema.GroupVersionResource.Resource (which we compare against) contains the plural.
	// As such, we need to convert the singular into plural first so that the switch..case actually works.
	kind := getPluralOfSingularKind(strings.ToLower(object.GetKind()))

	// BackupInfo gets populated differently depending on SG or CNPG
	b := &v1.BackupInfo{ObjectMeta: *o}
	switch kind {
	case CNPGbackupGroupVersionResource.Resource:
		// CNPG
		status, _, err := unstructured.NestedMap(content, v1.Status)
		if err != nil {
			return nil, fmt.Errorf("cannot parse status field from object name %s", name)
		}

		if status != nil {
			var podName any
			if instanceID, ok := status["instanceID"].(map[string]any); ok {
				podName = instanceID["podName"]
			}

			b.Process = runtime.RawExtension{Object: &unstructured.Unstructured{Object: map[string]any{
				"jobPod": podName,
				"status": status["phase"],
				"timing": map[string]any{
					"start": status["startedAt"],
					"end":   status["stoppedAt"],
				},
			}}}
			b.BackupInformation = runtime.RawExtension{Object: &unstructured.Unstructured{Object: map[string]any{
				"backupId":        status["backupId"],
				"backupName":      status["backupName"],
				"destinationPath": status["destinationPath"],
				"beginLSN":        status["beginLSN"],
				"beginWal":        status["beginWal"],
				"endLSN":          status["endLSN"],
				"endWal":          status["endWal"],
				"serverName":      status["serverName"],
			}}}
		}
	default:
		// StackGres
		p, _, err := unstructured.NestedMap(content, v1.Status, v1.Process)
		if err != nil {
			return nil, fmt.Errorf("cannot parse status.process field from object name %s", name)
		}

		bi, _, err := unstructured.NestedMap(content, v1.Status, v1.BackupInformation)
		if err != nil {
			return nil, fmt.Errorf("cannot parse status.backupInformation field from object name %s", name)
		}

		if p != nil {
			b.Process = runtime.RawExtension{Object: &unstructured.Unstructured{Object: p}}
		}
		if bi != nil {
			b.BackupInformation = runtime.RawExtension{Object: &unstructured.Unstructured{Object: bi}}
		}
	}

	return b, nil
}

// Returns the plural form of a kind (if it isn't already)
func getPluralOfSingularKind(kind string) string {
	if strings.HasSuffix(kind, "s") {
		return kind
	} else {
		return fmt.Sprintf("%ss", kind)
	}
}
