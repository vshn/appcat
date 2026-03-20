package backup

import (
	"context"
	"testing"

	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"

	"github.com/stretchr/testify/assert"
	appcatv1 "github.com/vshn/appcat/v4/apis/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
)

func TestAddBackupObjectCreation(t *testing.T) {
	svc, comp := getRedisBackupComp(t)

	ctx := context.TODO()

	assert.Nil(t, AddK8upBackup(ctx, svc, comp))

	bucket := &appcatv1.XObjectBucket{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(bucket, comp.Name+"-backup"))

	repoPW := &corev1.Secret{}
	assert.NoError(t, svc.GetDesiredKubeObject(repoPW, comp.Name+"-k8up-repo-pw"))

}

func TestAddBackupDisabled(t *testing.T) {
	svc, comp := getRedisBackupComp(t)

	// Disable backups
	enabled := false
	comp.Spec.Parameters.Backup.Enabled = &enabled

	ctx := context.TODO()

	// Should not return error
	assert.Nil(t, AddK8upBackup(ctx, svc, comp))

	// Should not create any backup resources
	bucket := &appcatv1.XObjectBucket{}
	assert.Error(t, svc.GetDesiredComposedResourceByName(bucket, comp.Name+"-backup"))

	repoPW := &corev1.Secret{}
	assert.Error(t, svc.GetDesiredKubeObject(repoPW, comp.Name+"-k8up-repo-pw"))
}

func getRedisBackupComp(t *testing.T) (*runtime.ServiceRuntime, *vshnv1.VSHNRedis) {
	svc := commontest.LoadRuntimeFromFile(t, "vshnredis/backup/01_default.yaml")

	comp := &vshnv1.VSHNRedis{}
	err := svc.GetDesiredComposite(comp)
	assert.NoError(t, err)

	return svc, comp
}

func Test_setNestedValue(t *testing.T) {
	type args struct {
		values map[string]interface{}
		path   []string
		val    interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name: "GivenPathOfOneLevel_ThenInsertIt",
			args: args{
				values: map[string]interface{}{
					"test": "",
				},
				path: []string{"test"},
				val:  "hello",
			},
			want: map[string]interface{}{
				"test": "hello",
			},
		},
		{
			name: "GivenPathOfTwoLevels_ThenInsertIt",
			args: args{
				values: map[string]interface{}{
					"test": map[string]interface{}{
						"test2": "",
					},
				},
				path: []string{"test", "test2"},
				val:  "hello",
			},
			want: map[string]interface{}{
				"test": map[string]interface{}{
					"test2": "hello",
				},
			},
		},
		{
			name: "GivenPathOfThreeLevels_ThenInsertIt",
			args: args{
				values: map[string]interface{}{
					"test": map[string]interface{}{
						"test2": map[string]interface{}{
							"test3": "",
						},
					},
				},
				path: []string{"test", "test2", "test3"},
				val:  "hello",
			},
			want: map[string]interface{}{
				"test": map[string]interface{}{
					"test2": map[string]interface{}{
						"test3": "hello",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := common.SetNestedObjectValue(tt.args.values, tt.args.path, tt.args.val); (err != nil) != tt.wantErr {
				t.Errorf("setNestedValue() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, tt.want, tt.args.values)
		})
	}
}

func TestGetBucketCredentials(t *testing.T) {
	tests := []struct {
		name    string
		want    BucketInfo
		wantErr bool
		comp    common.InfoGetter
	}{
		{
			name: "GivenNoCustomBucket_thenExpectGeneratedBucket",
			// No region for k8up backups
			want: BucketInfo{
				Endpoint: "test",
				Bucket:   "test",
				KeyID: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "backup-bucket-credentials-redis-gc9x4",
					},
					Key: "AWS_ACCESS_KEY_ID",
				},
				SecretID: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "backup-bucket-credentials-redis-gc9x4",
					},
					Key: "AWS_SECRET_ACCESS_KEY",
				},
			},
		},
		{
			name: "GivenCustomBucket_thenExpectCustomBucket",
			want: BucketInfo{
				Endpoint: "customTest",
				Bucket:   "customTest",
				KeyID: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "customName",
					},
					Key: "customKey",
				},
				SecretID: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "customName",
					},
					Key: "customKey",
				},
			},
			// We don't have the secret in the state, so it will not find it
			wantErr: true,
			comp: &vshnv1.VSHNRedis{
				Spec: vshnv1.VSHNRedisSpec{
					Parameters: vshnv1.VSHNRedisParameters{
						Backup: vshnv1.K8upBackupSpec{
							UnmanagedBucket: &vshnv1.UnmanagedBucket{
								Endpoint: "customTest",
								Bucket:   "customTest",
								AccessKey: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "customName",
									},
									Key: "customKey",
								},
								SecretKey: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "customName",
									},
									Key: "customKey",
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := commontest.LoadRuntimeFromFile(t, "vshnredis/backup/02_backup-credentials.yaml")

			var comp common.InfoGetter
			if tt.comp == nil {
				tmp := &vshnv1.VSHNRedis{}
				err := svc.GetDesiredComposite(tmp)
				assert.NoError(t, err)
				comp = tmp
			} else {
				comp = tt.comp
			}

			got, gotErr := GetBucketCredentials(context.Background(), svc, comp)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("GetBucketCredentials() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("GetBucketCredentials() succeeded unexpectedly")
			}

			assert.Equal(t, tt.want, got)

		})
	}
}
