package vshnredis

import (
	"context"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	xkubev1 "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	batchv1 "k8s.io/api/batch/v1"
)

func TestPVCResize(t *testing.T) {

	// First reconcile, creation of the job
	svc := commontest.LoadRuntimeFromFile(t, "vshnredis/pvcresize/01_default.yaml")

	comp := &vshnv1.VSHNRedis{}
	err := svc.GetDesiredComposite(comp)
	assert.NoError(t, err)

	ctx := context.TODO()

	assert.Nil(t, ResizePVCs(ctx, &vshnv1.VSHNRedis{}, svc))

	job := &batchv1.Job{}
	assert.NoError(t, svc.GetDesiredKubeObject(job, "redis-gc9x4-sts-deleter"))

	obj := &xkubev1.Object{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(obj, "redis-gc9x4-sts-deleter"))

	// Second reconcile, check for removed job
	svc = commontest.LoadRuntimeFromFile(t, "vshnredis/pvcresize/01_job.yaml")
	assert.Nil(t, ResizePVCs(ctx, &vshnv1.VSHNRedis{}, svc))
	assert.Error(t, svc.GetDesiredKubeObject(job, "redis-gc9x4-sts-deleter"))

}

func Test_needReleasePatch(t *testing.T) {
	type args struct {
		comp   *vshnv1.VSHNRedis
		values map[string]interface{}
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "sizeEqual",
			want: false,
			args: args{
				comp: &vshnv1.VSHNRedis{
					Spec: vshnv1.VSHNRedisSpec{
						Parameters: vshnv1.VSHNRedisParameters{
							Size: vshnv1.VSHNRedisSizeSpec{
								Disk: "15Gi",
							},
						},
					},
				},
				values: map[string]interface{}{
					"replica": map[string]interface{}{
						"persistence": map[string]interface{}{
							"size": "15Gi",
						},
					},
				},
			},
		},
		{
			name: "sizeIncreased",
			want: true,
			args: args{
				comp: &vshnv1.VSHNRedis{
					Spec: vshnv1.VSHNRedisSpec{
						Parameters: vshnv1.VSHNRedisParameters{
							Size: vshnv1.VSHNRedisSizeSpec{
								Disk: "16Gi",
							},
						},
					},
				},
				values: map[string]interface{}{
					"replica": map[string]interface{}{
						"persistence": map[string]interface{}{
							"size": "15Gi",
						},
					},
				},
			},
		},
		{
			name: "sizeDecreased",
			want: false,
			args: args{
				comp: &vshnv1.VSHNRedis{
					Spec: vshnv1.VSHNRedisSpec{
						Parameters: vshnv1.VSHNRedisParameters{
							Size: vshnv1.VSHNRedisSizeSpec{
								Disk: "14Gi",
							},
						},
					},
				},
				values: map[string]interface{}{
					"replica": map[string]interface{}{
						"persistence": map[string]interface{}{
							"size": "15Gi",
						},
					},
				},
			},
		},
		{
			name:    "invalidRequest",
			want:    false,
			wantErr: true,
			args: args{
				comp: &vshnv1.VSHNRedis{
					Spec: vshnv1.VSHNRedisSpec{
						Parameters: vshnv1.VSHNRedisParameters{
							Size: vshnv1.VSHNRedisSizeSpec{
								Disk: "foo",
							},
						},
					},
				},
				values: map[string]interface{}{
					"replica": map[string]interface{}{
						"persistence": map[string]interface{}{
							"size": "15Gi",
						},
					},
				},
			},
		},
		{
			name:    "invalidValues",
			want:    false,
			wantErr: true,
			args: args{
				comp: &vshnv1.VSHNRedis{
					Spec: vshnv1.VSHNRedisSpec{
						Parameters: vshnv1.VSHNRedisParameters{
							Size: vshnv1.VSHNRedisSizeSpec{
								Disk: "15Gi",
							},
						},
					},
				},
				values: map[string]interface{}{
					"replica": map[string]interface{}{
						"persistence": map[string]interface{}{
							"size": "foo",
						},
					},
				},
			},
		},
		{
			name:    "missingSizeInValues",
			want:    false,
			wantErr: true,
			args: args{
				comp: &vshnv1.VSHNRedis{
					Spec: vshnv1.VSHNRedisSpec{
						Parameters: vshnv1.VSHNRedisParameters{
							Size: vshnv1.VSHNRedisSizeSpec{
								Disk: "15Gi",
							},
						},
					},
				},
				values: map[string]interface{}{
					"replica": map[string]interface{}{
						"persistence": map[string]interface{}{},
					},
				},
			},
		},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			got, got1 := needReleasePatch(tt.args.comp, tt.args.values)

			assert.Equal(t, tt.want, got)

			if tt.wantErr {
				assert.NotNil(t, got1)
			} else {
				assert.Nil(t, got1)
			}

		})
	}
}
