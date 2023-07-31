package redis

import (
	"context"
	"fmt"

	appcatv1 "github.com/vshn/appcat/apis/appcat/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.TableConvertor = &vshnRedisBackupStorage{}

func (v *vshnRedisBackupStorage) ConvertToTable(_ context.Context, obj runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {

	table := &metav1.Table{}

	backups := []appcatv1.VSHNRedisBackup{}
	if meta.IsListType(obj) {
		backupList, ok := obj.(*appcatv1.VSHNRedisBackupList)
		if !ok {
			return nil, fmt.Errorf("not a vshn redis backup: %#v", obj)
		}
		backups = backupList.Items
	} else {
		backup, ok := obj.(*appcatv1.VSHNRedisBackup)
		if !ok {
			return nil, fmt.Errorf("not a vshn redis backup: %#v", obj)
		}
		backups = append(backups, *backup)
	}

	for i := range backups {
		table.Rows = append(table.Rows, backupToTableRow(&backups[i]))
	}

	if opt, ok := tableOptions.(*metav1.TableOptions); !ok || !opt.NoHeaders {
		table.ColumnDefinitions = []metav1.TableColumnDefinition{
			{Name: "Backup ID", Type: "string", Format: "name", Description: "ID of the snapshot"},
			{Name: "Database Instance", Type: "string", Description: "The redis instance"},
			{Name: "Backup Time", Type: "string", Description: "When backup was made"},
		}
	}

	return table, nil
}

func backupToTableRow(backup *appcatv1.VSHNRedisBackup) metav1.TableRow {

	return metav1.TableRow{
		Cells: []interface{}{
			trimStringLength(backup.Status.ID),
			backup.Status.Instance,
			backup.Status.Date},
		Object: runtime.RawExtension{Object: backup},
	}
}
