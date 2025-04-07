package postgres

import (
	"context"
	"fmt"

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

var (
	SGbackupGroupVersionResource = schema.GroupVersionResource{
		Group:    "stackgres.io",
		Version:  "v1",
		Resource: "sgbackups",
	}
)

// sgbackupProvider is an abstraction to interact with the K8s API
type sgbackupProvider interface {
	GetSGBackup(ctx context.Context, name, namespace string, client *client.DynamicClient) (*v1.SGBackupInfo, error)
	ListSGBackup(ctx context.Context, namespace string, client *client.DynamicClient, options *metainternalversion.ListOptions) (*[]v1.SGBackupInfo, error)
	WatchSGBackup(ctx context.Context, namespace string, options *metainternalversion.ListOptions) (watch.Interface, error)
}

type KubeSGBackupProvider struct {
	DynamicClient client.NamespaceableResourceInterface
}

// GetSGBackup fetches SGBackup resource into unstructured.Unstructured. Relevant data is saved to v1.SGBackupInfo
func (k *KubeSGBackupProvider) GetSGBackup(ctx context.Context, name, namespace string, scClient *client.DynamicClient) (*v1.SGBackupInfo, error) {
	var unstructuredObject *unstructured.Unstructured
	var err error
	if scClient != nil {
		dc := scClient.Resource(SGbackupGroupVersionResource)
		unstructuredObject, err = dc.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		unstructuredObject, err = k.DynamicClient.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, err
	}
	return convertToSGBackupInfo(unstructuredObject)
}

// ListSGBackup fetches SGBackup resources into unstructured.UnstructuredList. Relevant data is saved to v1.SGBackupInfo
func (k *KubeSGBackupProvider) ListSGBackup(ctx context.Context, namespace string, scClient *client.DynamicClient, options *metainternalversion.ListOptions) (*[]v1.SGBackupInfo, error) {

	var unstructuredList *unstructured.UnstructuredList
	var err error
	if scClient != nil {
		dc := scClient.Resource(SGbackupGroupVersionResource)
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
	sgbackupsInfos := make([]v1.SGBackupInfo, 0, len(unstructuredList.Items))
	for _, v := range unstructuredList.Items {
		backupsInfo, err := convertToSGBackupInfo(&v)
		if err != nil {
			continue
		}
		sgbackupsInfos = append(sgbackupsInfos, *backupsInfo)
	}
	return &sgbackupsInfos, nil
}

// WatchSGBackup watches SGBackup resources.
func (k *KubeSGBackupProvider) WatchSGBackup(ctx context.Context, namespace string, options *metainternalversion.ListOptions) (watch.Interface, error) {
	return k.DynamicClient.Namespace(namespace).Watch(ctx, metav1.ListOptions{
		TypeMeta:      options.TypeMeta,
		LabelSelector: options.LabelSelector.String(),
		FieldSelector: options.FieldSelector.String(),
		Limit:         options.Limit,
		Continue:      options.Continue,
	})
}

// GetFromEvent resolves watch.Event into v1.SGBackupInfo
func GetFromEvent(in watch.Event) *v1.SGBackupInfo {
	u, ok := in.Object.(*unstructured.Unstructured)
	if !ok {
		return nil
	}

	backup, err := convertToSGBackupInfo(u)
	if err != nil {
		return nil
	}
	return backup
}

func convertToSGBackupInfo(object *unstructured.Unstructured) (*v1.SGBackupInfo, error) {
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

	p, _, err := unstructured.NestedMap(content, v1.Status, v1.Process)
	if err != nil {
		return nil, fmt.Errorf("cannot parse status.process field from object name %s", name)
	}

	bi, _, err := unstructured.NestedMap(content, v1.Status, v1.BackupInformation)
	if err != nil {
		return nil, fmt.Errorf("cannot parse status.backupInformation field from object name %s", name)
	}

	b := &v1.SGBackupInfo{ObjectMeta: *o}
	if p != nil {
		b.Process = runtime.RawExtension{Object: &unstructured.Unstructured{Object: p}}
	}
	if bi != nil {
		b.BackupInformation = runtime.RawExtension{Object: &unstructured.Unstructured{Object: bi}}
	}

	return b, nil
}
