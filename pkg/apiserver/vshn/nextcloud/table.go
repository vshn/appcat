package nextcloud

import (
	"context"
	"crypto/sha1"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/duration"

	appcatv1 "github.com/vshn/appcat/v4/apis/apiserver/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.TableConvertor = &vshnNextcloudBackupStorage{}

func (v *vshnNextcloudBackupStorage) ConvertToTable(_ context.Context, obj runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {

	table := &metav1.Table{}

	backups := []appcatv1.VSHNNextcloudBackup{}
	if meta.IsListType(obj) {
		backupList, ok := obj.(*appcatv1.VSHNNextcloudBackupList)
		if !ok {
			return nil, fmt.Errorf("not a vshn nextcloud backup: %#v", obj)
		}
		backups = backupList.Items
	} else {
		backup, ok := obj.(*appcatv1.VSHNNextcloudBackup)
		if !ok {
			return nil, fmt.Errorf("not a vshn nextcloud backup: %#v", obj)
		}
		backups = append(backups, *backup)
	}

	for i := range backups {
		table.Rows = append(table.Rows, backupToTableRow(&backups[i]))
	}

	if opt, ok := tableOptions.(*metav1.TableOptions); !ok || !opt.NoHeaders {
		table.ColumnDefinitions = getNextcloudTableDefinition()

	}

	return table, nil
}

// ToDo Once k8up exposes start time, update the code here
func backupToTableRow(backup *appcatv1.VSHNNextcloudBackup) metav1.TableRow {
	return getNextcloudBackupTable(
		trimStringLength(backup.Status.NextcloudFileBackup.ID),
		backup.Status.NextcloudFileBackup.Instance,
		"Completed",
		duration.HumanDuration(time.Since(backup.GetCreationTimestamp().Time)),
		backup.Status.NextcloudFileBackup.Date.Format(time.RFC3339),
		backup.Status.NextcloudFileBackup.Date.Format(time.RFC3339),
		backup.Status.FileBackupAvailable,
		backup.Status.DatabaseBackupAvailable,
		backup,
	)
}

func getNextcloudTableDefinition() []metav1.TableColumnDefinition {
	desc := metav1.ObjectMeta{}.SwaggerDoc()
	return []metav1.TableColumnDefinition{
		{Name: "Backup ID", Type: "string", Format: "name", Description: desc["name"]},
		{Name: "Database Instance", Type: "string", Description: "The database instance"},
		{Name: "Started", Type: "string", Description: "The backup start time"},
		{Name: "Finished", Type: "string", Description: "The data is available up to this time"},
		{Name: "FileBackup", Type: "bool", Description: "If the backup contains the Nextcloud files"},
		{Name: "DatabaseBackup", Type: "bool", Description: "If the backup contains the Database files"},
		{Name: "Status", Type: "string", Description: "The state of this backup"},
		{Name: "Age", Type: "date", Description: desc["creationTimestamp"]},
	}
}

func getNextcloudBackupTable(id, instance, status, age, started, finished string, files, db bool, backup runtime.Object) metav1.TableRow {
	return metav1.TableRow{
		Cells:  []interface{}{id, instance, started, finished, files, db, status, age}, // Snapshots are created only when the backup successfully finished
		Object: runtime.RawExtension{Object: backup},
	}
}

// hashString returns the first 8 symbols of the sha1 hash
func hashString(name string) string {
	hasher := sha1.New()
	hasher.Write([]byte(name))
	return fmt.Sprintf("%x", hasher.Sum(nil))[:8]
}
