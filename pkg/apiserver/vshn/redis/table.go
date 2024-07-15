package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/vshn/appcat/v4/pkg/apiserver"
	"k8s.io/apimachinery/pkg/util/duration"

	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
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
		table.ColumnDefinitions = apiserver.GetBackupColumnDefinition()
	}

	return table, nil
}

// ToDo Once k8up exposes start time, update the code here
func backupToTableRow(backup *appcatv1.VSHNRedisBackup) metav1.TableRow {
	return apiserver.GetBackupTable(
		trimStringLength(backup.Status.ID),
		backup.Status.Instance,
		"Completed",
		duration.HumanDuration(time.Since(backup.GetCreationTimestamp().Time)),
		backup.Status.Date.Format(time.RFC3339),
		backup.Status.Date.Format(time.RFC3339),
		backup,
	)
}
