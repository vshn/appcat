package vshnredis

import (
	"context"
	_ "embed"
	"testing"

	xkubev1 "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/apis/vshn/v1"
	"github.com/vshn/appcat/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/pkg/comp-functions/runtime"
	batchv1 "k8s.io/api/batch/v1"
)

func TestPVCResize(t *testing.T) {

	// First reconcile, creation of the job
	iof := commontest.LoadRuntimeFromFile(t, "vshnredis/pvcresize/01_default.yaml")

	comp := &vshnv1.VSHNRedis{}
	err := iof.Desired.GetComposite(context.TODO(), comp)
	assert.NoError(t, err)

	ctx := context.TODO()

	assert.Equal(t, runtime.NewNormal(), ResizePVCs(ctx, iof))

	job := &batchv1.Job{}
	assert.NoError(t, iof.Desired.GetFromObject(ctx, job, "redis-gc9x4-sts-deleter"))

	obj := &xkubev1.Object{}
	assert.NoError(t, iof.Desired.Get(ctx, obj, "redis-gc9x4-sts-deleter"))

	// Second reconcile, check for removed job
	iof = commontest.LoadRuntimeFromFile(t, "vshnredis/pvcresize/01_job.yaml")
	assert.Equal(t, runtime.NewNormal(), ResizePVCs(ctx, iof))
	assert.Error(t, iof.Desired.GetFromObject(ctx, job, "redis-gc9x4-sts-deleter"))

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
					"master": map[string]interface{}{
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
					"master": map[string]interface{}{
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
					"master": map[string]interface{}{
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
					"master": map[string]interface{}{
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
					"master": map[string]interface{}{
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
					"master": map[string]interface{}{
						"persistence": map[string]interface{}{},
					},
				},
			},
		},
	}
	for _, tt := range tests {

		ctx := context.TODO()

		t.Run(tt.name, func(t *testing.T) {
			got, got1 := needReleasePatch(ctx, tt.args.comp, tt.args.values)

			assert.Equal(t, tt.want, got)

			if tt.wantErr {
				assert.NotNil(t, got1)
			} else {
				assert.Nil(t, got1)
			}

		})
	}
}
