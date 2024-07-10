package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/vshn/appcat/v4/pkg/apiserver"

	v1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apiserver/pkg/registry/rest"
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
		table.ColumnDefinitions = apiserver.GetBackupColumnDefinition()
	}
	return &table, nil
}

func backupToTableRow(backup *v1.VSHNPostgresBackup) metav1.TableRow {
	return apiserver.GetBackupTable(
		backup.GetName(),
		backup.Status.DatabaseInstance,
		getProcessStatus(backup.Status.Process),
		duration.HumanDuration(time.Since(backup.GetCreationTimestamp().Time)),
		getStartTime(backup.Status.Process),
		getEndTime(backup.Status.Process),
		backup)
}

func getEndTime(process *runtime.RawExtension) string {
	if process != nil && process.Object != nil {
		if v, err := runtime.DefaultUnstructuredConverter.ToUnstructured(process.Object); err == nil {
			if endTime, exists, _ := unstructured.NestedString(v, v1.Timing, v1.End); exists {
				return endTime
			}
		}
	}
	return ""
}

func getStartTime(process *runtime.RawExtension) string {
	if process != nil && process.Object != nil {
		if v, err := runtime.DefaultUnstructuredConverter.ToUnstructured(process.Object); err == nil {
			if startTime, exists, _ := unstructured.NestedString(v, v1.Timing, v1.Start); exists {
				return startTime
			}
		}
	}
	return ""
}

func getProcessStatus(process *runtime.RawExtension) string {
	if process != nil && process.Object != nil {
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
