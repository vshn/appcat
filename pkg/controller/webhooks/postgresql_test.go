package webhooks

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPostgreSQLWebhookHandler_ValidateCreate(t *testing.T) {
	// Given
	claimNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "claimns",
			Labels: map[string]string{
				utils.OrgLabelName: "myorg",
			},
		},
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vshnpostgresqlplans",
			Namespace: "testns",
		},
		Data: map[string]string{
			"sideCars": `
			{
				"clusterController": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "32m",
						"memory": "188Mi"
					}
				},
				"createBackup": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "250m",
						"memory": "256Mi"
					}
				},
				"envoy": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "32m",
						"memory": "64Mi"
					}
				},
				"pgbouncer": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "16m",
						"memory": "32Mi"
					}
				},
				"postgresUtil": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "10m",
						"memory": "4Mi"
					}
				},
				"prometheusPostgresExporter": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "10m",
						"memory": "32Mi"
					}
				},
				"runDbops": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "250m",
						"memory": "256Mi"
					}
				},
				"setDbopsResult": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "250m",
						"memory": "256Mi"
					}
				}
			}
			`,
		},
	}

	ctx := context.TODO()

	fclient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(claimNS, cm).
		Build()

	viper.Set("PLANS_NAMESPACE", "testns")
	viper.AutomaticEnv()

	handler := PostgreSQLWebhookHandler{
		DefaultWebhookHandler: DefaultWebhookHandler{
			client:    fclient,
			log:       logr.Discard(),
			withQuota: true,
			obj:       &vshnv1.VSHNPostgreSQL{},
			name:      "postgresql",
		},
	}

	pgOrig := &vshnv1.VSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "claimns",
		},
		Spec: vshnv1.VSHNPostgreSQLSpec{
			Parameters: vshnv1.VSHNPostgreSQLParameters{
				Size: vshnv1.VSHNSizeSpec{
					CPU:    "500m",
					Memory: "1Gi",
				},
				Instances: 1,
				Service: vshnv1.VSHNPostgreSQLServiceSpec{
					RepackEnabled: true,
				},
			},
		},
	}

	// When within quota
	_, err := handler.ValidateCreate(ctx, pgOrig)

	//Then no err
	assert.NoError(t, err)

	//When quota breached
	// CPU Limits
	pgInvalid := pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.CPU = "15000m"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	//CPU Requests
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Requests.CPU = "6500m"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	//Memory Limits
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Memory = "25Gi"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	//Memory requests
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Requests.Memory = "25Gi"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	//Disk
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Disk = "25Ti"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	//When invalid size
	// CPU Limits
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.CPU = "foo"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	//CPU Requests
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Requests.CPU = "foo"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	//Memory Limits
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Memory = "foo"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	//Memory requests
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Requests.Memory = "foo"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	//Disk
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Disk = "foo"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	//Instances
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Instances = 1
	pgInvalid.Spec.Parameters.Service.ServiceLevel = "guaranteed"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)
}

func TestPostgreSQLWebhookHandler_ValidateDelete(t *testing.T) {
	// Given
	claimNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "claimns",
			Labels: map[string]string{
				utils.OrgLabelName: "myorg",
			},
		},
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vshnpostgresqlplans",
			Namespace: "testns",
		},
		Data: map[string]string{
			"sideCars": `
			{
				"clusterController": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "32m",
						"memory": "188Mi"
					}
				},
				"createBackup": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "250m",
						"memory": "256Mi"
					}
				},
				"envoy": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "32m",
						"memory": "64Mi"
					}
				},
				"pgbouncer": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "16m",
						"memory": "32Mi"
					}
				},
				"postgresUtil": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "10m",
						"memory": "4Mi"
					}
				},
				"prometheusPostgresExporter": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "10m",
						"memory": "32Mi"
					}
				},
				"runDbops": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "250m",
						"memory": "256Mi"
					}
				},
				"setDbopsResult": {
					"limits": {
						"cpu": "600m",
						"memory": "768Mi"
					},
					"requests": {
						"cpu": "250m",
						"memory": "256Mi"
					}
				}
			}
			`,
		},
	}

	ctx := context.TODO()

	fclient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(claimNS, cm).
		Build()

	viper.Set("PLANS_NAMESPACE", "testns")
	viper.AutomaticEnv()

	handler := PostgreSQLWebhookHandler{
		DefaultWebhookHandler: DefaultWebhookHandler{
			client:    fclient,
			log:       logr.Discard(),
			withQuota: true,
			obj:       &vshnv1.VSHNPostgreSQL{},
			name:      "postgresql",
		},
	}

	pgOrig := &vshnv1.VSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "claimns",
		},
		Spec: vshnv1.VSHNPostgreSQLSpec{
			Parameters: vshnv1.VSHNPostgreSQLParameters{
				Size: vshnv1.VSHNSizeSpec{
					CPU:    "500m",
					Memory: "1Gi",
				},
				Instances: 1,
				Service: vshnv1.VSHNPostgreSQLServiceSpec{
					RepackEnabled: true,
				},
				Security: vshnv1.Security{
					DeletionProtection: true,
				},
			},
		},
	}

	// When within quota
	_, err := handler.ValidateDelete(ctx, pgOrig)

	//Then err
	assert.Error(t, err)

	//Instances
	pgDeletable := pgOrig.DeepCopy()
	pgDeletable.Spec.Parameters.Security.DeletionProtection = false

	_, err = handler.ValidateDelete(ctx, pgDeletable)

	//Then no err
	assert.NoError(t, err)

}
