package postgres

import (
	"appcat-apiserver/apis/appcat/v1"
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apiserver/pkg/registry/rest"
	"time"
)

var _ rest.TableConvertor = &vshnPostgresBackupStorage{}

// ConvertToTable translates the given object to a table for kubectl printing
func (v *vshnPostgresBackupStorage) ConvertToTable(_ context.Context, obj runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	var table metav1.Table

	backups := []v1.VSHNPostgresBackup{}
	if meta.IsListType(obj) {
		backupList, ok := obj.(*v1.VSHNPostgresBackupList)
		if !ok {
			return nil, fmt.Errorf("not a vshn postgres backup: %#v", obj)
		}
		backups = backupList.Items
	} else {
		backup, ok := obj.(*v1.VSHNPostgresBackup)
		if !ok {
			return nil, fmt.Errorf("not a vshn postgres backup: %#v", obj)
		}
		backups = append(backups, *backup)
	}

	for _, backup := range backups {
		table.Rows = append(table.Rows, backupToTableRow(&backup))
	}

	if opt, ok := tableOptions.(*metav1.TableOptions); !ok || !opt.NoHeaders {
		desc := metav1.ObjectMeta{}.SwaggerDoc()
		table.ColumnDefinitions = []metav1.TableColumnDefinition{
			{Name: "Backup Name", Type: "string", Format: "name", Description: desc["name"]},
			{Name: "Database Instance", Type: "string", Description: "The database instance"},
			{Name: "Status", Type: "string", Description: "The state of this backup"},
			{Name: "Age", Type: "date", Description: desc["creationTimestamp"]},
		}
	}
	return &table, nil
}

func backupToTableRow(backup *v1.VSHNPostgresBackup) metav1.TableRow {
	return metav1.TableRow{
		Cells: []interface{}{
			backup.GetName(),
			backup.Status.DatabaseInstance,
			getProcessStatus(backup.Status.Process),
			duration.HumanDuration(time.Since(backup.GetCreationTimestamp().Time))},
		Object: runtime.RawExtension{Object: backup},
	}
}

func getProcessStatus(process runtime.RawExtension) string {
	if process.Object != nil {
		unstructuredProcess, err := runtime.DefaultUnstructuredConverter.ToUnstructured(process.Object)
		if err != nil {
			return ""
		}

		status, ok, err := unstructured.NestedString(unstructuredProcess, v1.Status)
		if err != nil || !ok {
			return ""
		}
		return status
	}
	return ""
}
