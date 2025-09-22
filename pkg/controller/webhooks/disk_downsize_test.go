package webhooks

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestValidateDiskDownsizing(t *testing.T) {
	tests := []struct {
		name       string
		oldSize    vshnv1.VSHNSizeSpec
		newSize    vshnv1.VSHNSizeSpec
		shouldFail bool
		errorMsg   string
	}{
		{
			name:       "disk downsize blocked",
			oldSize:    vshnv1.VSHNSizeSpec{Disk: "20Gi"},
			newSize:    vshnv1.VSHNSizeSpec{Disk: "10Gi"},
			shouldFail: true,
			errorMsg:   "disk downsizing not allowed",
		},
		{
			name:       "disk upsize allowed",
			oldSize:    vshnv1.VSHNSizeSpec{Disk: "20Gi"},
			newSize:    vshnv1.VSHNSizeSpec{Disk: "30Gi"},
			shouldFail: false,
		},
		{
			name:       "same disk size allowed",
			oldSize:    vshnv1.VSHNSizeSpec{Disk: "20Gi"},
			newSize:    vshnv1.VSHNSizeSpec{Disk: "20Gi"},
			shouldFail: false,
		},
		{
			name:       "no old disk allowed",
			oldSize:    vshnv1.VSHNSizeSpec{},
			newSize:    vshnv1.VSHNSizeSpec{Disk: "10Gi"},
			shouldFail: false,
		},
		{
			name:       "no new disk allowed",
			oldSize:    vshnv1.VSHNSizeSpec{Disk: "20Gi"},
			newSize:    vshnv1.VSHNSizeSpec{},
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fclient := fake.NewClientBuilder().
				WithScheme(pkg.SetupScheme()).
				Build()

			handler := &DefaultWebhookHandler{
				client: fclient,
				log:    logr.Discard(),
			}

			oldComp := createPostgreSQL(tt.oldSize)
			newComp := createPostgreSQL(tt.newSize)

			err := handler.ValidateDiskDownsizing(context.TODO(), oldComp, newComp, "VSHNPostgreSQL")

			if tt.shouldFail {
				assert.NotNil(t, err)
				if tt.errorMsg != "" && err != nil {
					assert.Contains(t, err.Detail, tt.errorMsg)
				}
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestAllServicesDiskDownsizing(t *testing.T) {
	services := []struct {
		name        string
		serviceName string
		createComp  func(vshnv1.VSHNSizeSpec) common.Composite
	}{
		{"PostgreSQL", "VSHNPostgreSQL", createPostgreSQL},
		{"MariaDB", "VSHNMariaDB", createMariaDB},
		{"Redis", "VSHNRedis", createRedis},
		{"Forgejo", "VSHNForgejo", createForgejo},
		{"Nextcloud", "VSHNNextcloud", createNextcloud},
		{"MinIO", "VSHNMinio", createMinIO},
	}

	for _, service := range services {
		t.Run(service.name, func(t *testing.T) {
			fclient := fake.NewClientBuilder().
				WithScheme(pkg.SetupScheme()).
				Build()

			handler := &DefaultWebhookHandler{
				client: fclient,
				log:    logr.Discard(),
			}

			t.Run("disk_downsize_blocked", func(t *testing.T) {
				oldComp := service.createComp(vshnv1.VSHNSizeSpec{Disk: "20Gi"})
				newComp := service.createComp(vshnv1.VSHNSizeSpec{Disk: "10Gi"})

				err := handler.ValidateDiskDownsizing(context.TODO(), oldComp, newComp, service.serviceName)
				assert.NotNil(t, err)
				if err != nil {
					assert.Contains(t, err.Detail, "disk downsizing not allowed")
				}
			})

			t.Run("disk_upsize_allowed", func(t *testing.T) {
				oldComp := service.createComp(vshnv1.VSHNSizeSpec{Disk: "20Gi"})
				newComp := service.createComp(vshnv1.VSHNSizeSpec{Disk: "30Gi"})

				err := handler.ValidateDiskDownsizing(context.TODO(), oldComp, newComp, service.serviceName)
				assert.Nil(t, err)
			})
		})
	}
}

func createPostgreSQL(size vshnv1.VSHNSizeSpec) common.Composite {
	return &vshnv1.VSHNPostgreSQL{
		Spec: vshnv1.VSHNPostgreSQLSpec{
			Parameters: vshnv1.VSHNPostgreSQLParameters{Size: size},
		},
	}
}

func createMariaDB(size vshnv1.VSHNSizeSpec) common.Composite {
	return &vshnv1.VSHNMariaDB{
		Spec: vshnv1.VSHNMariaDBSpec{
			Parameters: vshnv1.VSHNMariaDBParameters{Size: size},
		},
	}
}

func createRedis(size vshnv1.VSHNSizeSpec) common.Composite {
	return &vshnv1.VSHNRedis{
		Spec: vshnv1.VSHNRedisSpec{
			Parameters: vshnv1.VSHNRedisParameters{
				Size: vshnv1.VSHNRedisSizeSpec{Disk: size.Disk, Plan: size.Plan},
			},
		},
	}
}

func createForgejo(size vshnv1.VSHNSizeSpec) common.Composite {
	return &vshnv1.VSHNForgejo{
		Spec: vshnv1.VSHNForgejoSpec{
			Parameters: vshnv1.VSHNForgejoParameters{Size: size},
		},
	}
}

func createNextcloud(size vshnv1.VSHNSizeSpec) common.Composite {
	return &vshnv1.VSHNNextcloud{
		Spec: vshnv1.VSHNNextcloudSpec{
			Parameters: vshnv1.VSHNNextcloudParameters{Size: size},
		},
	}
}

func createMinIO(size vshnv1.VSHNSizeSpec) common.Composite {
	return &vshnv1.VSHNMinio{
		Spec: vshnv1.VSHNMinioSpec{
			Parameters: vshnv1.VSHNMinioParameters{Size: size},
		},
	}
}
