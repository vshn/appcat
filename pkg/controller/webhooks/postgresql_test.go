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
	"k8s.io/apimachinery/pkg/runtime"
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
			client:     fclient,
			log:        logr.Discard(),
			withQuota:  true,
			obj:        &vshnv1.VSHNPostgreSQL{},
			name:       "postgresql",
			nameLength: 30,
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

	// Then no err
	assert.NoError(t, err)

	// When quota breached
	// CPU Limits
	pgInvalid := pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.CPU = "15000m"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	// CPU Requests
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Requests.CPU = "6500m"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	// Memory Limits
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Memory = "25Gi"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	// Memory requests
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Requests.Memory = "25Gi"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	// Disk
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Disk = "25Ti"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	// When invalid size
	// CPU Limits
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.CPU = "foo"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	// CPU Requests
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Requests.CPU = "foo"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	// Memory Limits
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Memory = "foo"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	// Memory requests
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Requests.Memory = "foo"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	// Disk
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Size.Disk = "foo"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	// Instances
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Instances = 1
	pgInvalid.Spec.Parameters.Service.ServiceLevel = "guaranteed"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	// check pgSettings
	pgValid := pgOrig.DeepCopy()
	pgValid.Spec.Parameters.Service.PostgreSQLSettings = runtime.RawExtension{
		Raw: []byte(`{"foo": "bar"}`),
	}
	_, err = handler.ValidateCreate(ctx, pgValid)
	assert.NoError(t, err)

	// check pgSettings
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Service.PostgreSQLSettings = runtime.RawExtension{
		Raw: []byte(`{"fsync": "bar"}`),
	}
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)

	// check pgSettings
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Service.PostgreSQLSettings = runtime.RawExtension{
		Raw: []byte(`{"fsync": "bar", "wal_level": "foo", "max_connections": "bar"}`),
	}

	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)
}

func TestPostgreSQLWebhookHandler_ValidateUpdate(t *testing.T) {
	ctx := context.TODO()
	claimNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "claimns",
			Labels: map[string]string{
				utils.OrgLabelName: "myorg",
			},
		},
	}
	fclient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(claimNS).
		Build()

	viper.Set("PLANS_NAMESPACE", "testns")
	viper.AutomaticEnv()

	handler := PostgreSQLWebhookHandler{
		DefaultWebhookHandler: DefaultWebhookHandler{
			client:     fclient,
			log:        logr.Discard(),
			withQuota:  false,
			obj:        &vshnv1.VSHNPostgreSQL{},
			name:       "postgresql",
			nameLength: 30,
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
					MajorVersion:  "15",
				},
			},
		},
		Status: vshnv1.VSHNPostgreSQLStatus{
			CurrentVersion: "15",
		},
	}

	// check pgSettings with single good setting
	pgValid := pgOrig.DeepCopy()
	pgValid.Spec.Parameters.Service.PostgreSQLSettings = runtime.RawExtension{
		Raw: []byte(`{"foo": "bar"}`),
	}
	_, err := handler.ValidateUpdate(ctx, pgOrig, pgValid)
	assert.NoError(t, err)

	// check pgSettings with single bad setting
	pgInvalid := pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Service.PostgreSQLSettings = runtime.RawExtension{
		Raw: []byte(`{"fsync": "bar"}`),
	}
	_, err = handler.ValidateUpdate(ctx, pgOrig, pgInvalid)
	assert.Error(t, err)

	// check pgSettings, startiong with valid settings
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.Parameters.Service.PostgreSQLSettings = runtime.RawExtension{
		Raw: []byte(`{"foo": "bar", "fsync": "bar", "wal_level": "foo", "max_connections": "bar"}`),
	}
	_, err = handler.ValidateUpdate(ctx, pgOrig, pgInvalid)
	assert.Error(t, err)

	// Allow changing compositionRef IF not yet set
	// Crossplane immediately sets this after creation automatically if the user does not supply it themselves
	pgValid = pgOrig.DeepCopy()
	pgValid.Spec.CompositionRef.Name = "vshnpostgrescnpg.vshn.appcat.vshn.io"
	_, err = handler.ValidateUpdate(ctx, pgOrig, pgValid)
	assert.NoError(t, err)

	// Do not allow changing compositionRef IF already set
	pgRef := pgOrig.DeepCopy()
	pgRef.Spec.CompositionRef.Name = "vshnpostgres.vshn.appcat.vshn.io"
	pgInvalid = pgOrig.DeepCopy()
	pgInvalid.Spec.CompositionRef.Name = "vshnpostgrescnpg.vshn.appcat.vshn.io"
	_, err = handler.ValidateUpdate(ctx, pgRef, pgInvalid)
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
			client:     fclient,
			log:        logr.Discard(),
			withQuota:  true,
			obj:        &vshnv1.VSHNPostgreSQL{},
			name:       "postgresql",
			nameLength: 30,
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

	// Then err
	assert.Error(t, err)

	// Instances
	pgDeletable := pgOrig.DeepCopy()
	pgDeletable.Spec.Parameters.Security.DeletionProtection = false

	_, err = handler.ValidateDelete(ctx, pgDeletable)

	// Then no err
	assert.NoError(t, err)
}

func TestPostgreSQLWebhookHandler_ValidateEncryptionChanges(t *testing.T) {
	ctx := context.TODO()
	claimNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "claimns",
			Labels: map[string]string{
				utils.OrgLabelName: "myorg",
			},
		},
	}
	fclient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(claimNS).
		Build()

	handler := PostgreSQLWebhookHandler{
		DefaultWebhookHandler: DefaultWebhookHandler{
			client:     fclient,
			log:        logr.Discard(),
			withQuota:  false,
			obj:        &vshnv1.VSHNPostgreSQL{},
			name:       "postgresql",
			nameLength: 30,
		},
	}

	// No encryption change should be valid
	pgOrig := &vshnv1.VSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "claimns",
		},
		Spec: vshnv1.VSHNPostgreSQLSpec{
			Parameters: vshnv1.VSHNPostgreSQLParameters{
				Encryption: vshnv1.VSHNPostgreSQLEncryption{
					Enabled: false,
				},
				Service: vshnv1.VSHNPostgreSQLServiceSpec{
					MajorVersion:  "15",
					RepackEnabled: true,
				},
			},
		},
	}

	pgUpdated := pgOrig.DeepCopy()
	// No changes to encryption state

	_, err := handler.ValidateUpdate(ctx, pgOrig, pgUpdated)
	assert.NoError(t, err)

	// Test 2: Enabling encryption after creation should fail
	pgEncryptionEnabled := pgOrig.DeepCopy()
	pgEncryptionEnabled.Spec.Parameters.Encryption.Enabled = true

	_, err = handler.ValidateUpdate(ctx, pgOrig, pgEncryptionEnabled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "encryption setting cannot be changed after instance creation")

	// Test 3: Disabling encryption after creation should fail
	pgOrigEncrypted := pgOrig.DeepCopy()
	pgOrigEncrypted.Spec.Parameters.Encryption.Enabled = true

	pgEncryptionDisabled := pgOrigEncrypted.DeepCopy()
	pgEncryptionDisabled.Spec.Parameters.Encryption.Enabled = false

	_, err = handler.ValidateUpdate(ctx, pgOrigEncrypted, pgEncryptionDisabled)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "encryption setting cannot be changed after instance creation")

	// Test 4: Same encryption state (enabled) should be valid
	pgSameEncryption := pgOrigEncrypted.DeepCopy()
	// No changes to encryption state

	_, err = handler.ValidateUpdate(ctx, pgOrigEncrypted, pgSameEncryption)
	assert.NoError(t, err)
}

func TestValidatePinImageTag(t *testing.T) {
	tests := []struct {
		name         string
		pinImageTag  string
		majorVersion string
		expectErr    bool
		errContains  string
	}{
		{
			name:         "GivenEmptyPinImageTag_ThenNoError",
			pinImageTag:  "",
			majorVersion: "15",
			expectErr:    false,
		},
		{
			name:         "GivenMatchingMajorVersion_ThenNoError",
			pinImageTag:  "15.13",
			majorVersion: "15",
			expectErr:    false,
		},
		{
			name:         "GivenMatchingMajorVersionDifferentMinor_ThenNoError",
			pinImageTag:  "15.9",
			majorVersion: "15",
			expectErr:    false,
		},
		{
			name:         "GivenMajorVersion16Match_ThenNoError",
			pinImageTag:  "16.4",
			majorVersion: "16",
			expectErr:    false,
		},
		{
			name:         "GivenMajorVersion17Match_ThenNoError",
			pinImageTag:  "17.2",
			majorVersion: "17",
			expectErr:    false,
		},
		{
			name:         "GivenMismatchedMajorVersion_ThenError",
			pinImageTag:  "16.4",
			majorVersion: "15",
			expectErr:    true,
			errContains:  "pinImageTag major version \"16\" must match majorVersion \"15\"",
		},
		{
			name:         "GivenPinImageTagWithDifferentMajor_ThenError",
			pinImageTag:  "15.13",
			majorVersion: "16",
			expectErr:    true,
			errContains:  "pinImageTag major version \"15\" must match majorVersion \"16\"",
		},
		{
			name:         "GivenPinImageTagWithMajorOnly_ThenNoError",
			pinImageTag:  "15",
			majorVersion: "15",
			expectErr:    false,
		},
		{
			name:         "GivenPinImageTagWithMajorOnlyMismatch_ThenError",
			pinImageTag:  "16",
			majorVersion: "15",
			expectErr:    true,
			errContains:  "pinImageTag major version \"16\" must match majorVersion \"15\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePinImageTag(tt.pinImageTag, tt.majorVersion)
			if tt.expectErr {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestPostgreSQLWebhookHandler_ValidateCreateWithPinImageTag(t *testing.T) {
	ctx := context.TODO()
	claimNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "claimns",
			Labels: map[string]string{
				utils.OrgLabelName: "myorg",
			},
		},
	}
	fclient := fake.NewClientBuilder().
		WithScheme(pkg.SetupScheme()).
		WithObjects(claimNS).
		Build()

	handler := PostgreSQLWebhookHandler{
		DefaultWebhookHandler: DefaultWebhookHandler{
			client:     fclient,
			log:        logr.Discard(),
			withQuota:  false,
			obj:        &vshnv1.VSHNPostgreSQL{},
			name:       "postgresql",
			nameLength: 30,
		},
	}

	pgBase := &vshnv1.VSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myinstance",
			Namespace: "claimns",
		},
		Spec: vshnv1.VSHNPostgreSQLSpec{
			Parameters: vshnv1.VSHNPostgreSQLParameters{
				Service: vshnv1.VSHNPostgreSQLServiceSpec{
					MajorVersion:  "15",
					RepackEnabled: true,
				},
			},
		},
	}

	// Test: Valid pinImageTag matching majorVersion
	pgValid := pgBase.DeepCopy()
	pgValid.Spec.Parameters.Maintenance.PinImageTag = "15.13"
	_, err := handler.ValidateCreate(ctx, pgValid)
	assert.NoError(t, err)

	// Test: Invalid pinImageTag not matching majorVersion
	pgInvalid := pgBase.DeepCopy()
	pgInvalid.Spec.Parameters.Maintenance.PinImageTag = "16.4"
	_, err = handler.ValidateCreate(ctx, pgInvalid)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pinImageTag major version")

	// Test: No pinImageTag set (should be valid)
	pgNoPin := pgBase.DeepCopy()
	pgNoPin.Spec.Parameters.Maintenance.PinImageTag = ""
	_, err = handler.ValidateCreate(ctx, pgNoPin)
	assert.NoError(t, err)
}

// disabling this temporarily
// func TestPostgreSQLWebhookHandler_ValidateMajorVersionUpgrade(t *testing.T) {
// 	tests := []struct {
// 		name          string
// 		new           *vshnv1.VSHNPostgreSQL
// 		old           *vshnv1.VSHNPostgreSQL
// 		expectErrList field.ErrorList
// 	}{
// 		{
// 			name: "GivenSameMajorVersion_ThenNoError",
// 			new: &vshnv1.VSHNPostgreSQL{
// 				Spec: vshnv1.VSHNPostgreSQLSpec{
// 					Parameters: vshnv1.VSHNPostgreSQLParameters{
// 						Service: vshnv1.VSHNPostgreSQLServiceSpec{
// 							MajorVersion: "15",
// 						},
// 					},
// 				},
// 				Status: vshnv1.VSHNPostgreSQLStatus{
// 					CurrentVersion: "15",
// 				},
// 			},
// 			old: &vshnv1.VSHNPostgreSQL{
// 				Spec: vshnv1.VSHNPostgreSQLSpec{
// 					Parameters: vshnv1.VSHNPostgreSQLParameters{
// 						Service: vshnv1.VSHNPostgreSQLServiceSpec{
// 							MajorVersion: "15",
// 						},
// 					},
// 				},
// 				Status: vshnv1.VSHNPostgreSQLStatus{
// 					CurrentVersion: "15",
// 				},
// 			},
// 			expectErrList: nil,
// 		},
// 		{
// 			name: "GivenOneMajorVersionUpdate_ThenNoError",
// 			new: &vshnv1.VSHNPostgreSQL{
// 				Spec: vshnv1.VSHNPostgreSQLSpec{
// 					Parameters: vshnv1.VSHNPostgreSQLParameters{
// 						Service: vshnv1.VSHNPostgreSQLServiceSpec{
// 							MajorVersion: "16",
// 						},
// 					},
// 				},
// 				Status: vshnv1.VSHNPostgreSQLStatus{
// 					CurrentVersion: "15",
// 				},
// 			},
// 			old: &vshnv1.VSHNPostgreSQL{
// 				Spec: vshnv1.VSHNPostgreSQLSpec{
// 					Parameters: vshnv1.VSHNPostgreSQLParameters{
// 						Service: vshnv1.VSHNPostgreSQLServiceSpec{
// 							MajorVersion: "15",
// 						},
// 					},
// 				},
// 				Status: vshnv1.VSHNPostgreSQLStatus{
// 					CurrentVersion: "15",
// 				},
// 			},
// 			expectErrList: nil,
// 		},
// 		{
// 			name: "GivenTwoMajorVersionsUpdate_ThenError",
// 			new: &vshnv1.VSHNPostgreSQL{
// 				Spec: vshnv1.VSHNPostgreSQLSpec{
// 					Parameters: vshnv1.VSHNPostgreSQLParameters{
// 						Service: vshnv1.VSHNPostgreSQLServiceSpec{
// 							MajorVersion: "17",
// 						},
// 					},
// 				},
// 				Status: vshnv1.VSHNPostgreSQLStatus{
// 					CurrentVersion: "15",
// 				},
// 			},
// 			old: &vshnv1.VSHNPostgreSQL{
// 				Spec: vshnv1.VSHNPostgreSQLSpec{
// 					Parameters: vshnv1.VSHNPostgreSQLParameters{
// 						Service: vshnv1.VSHNPostgreSQLServiceSpec{
// 							MajorVersion: "15",
// 						},
// 					},
// 				},
// 				Status: vshnv1.VSHNPostgreSQLStatus{
// 					CurrentVersion: "15",
// 				},
// 			},
// 			expectErrList: field.ErrorList{
// 				field.Forbidden(
// 					field.NewPath("spec.parameters.service.majorVersion"),
// 					"only one major version upgrade at a time is allowed",
// 				),
// 			},
// 		},
// 		{
// 			name: "GivenOneMajorVersionsBehind_ThenError",
// 			new: &vshnv1.VSHNPostgreSQL{
// 				Spec: vshnv1.VSHNPostgreSQLSpec{
// 					Parameters: vshnv1.VSHNPostgreSQLParameters{
// 						Service: vshnv1.VSHNPostgreSQLServiceSpec{
// 							MajorVersion: "14",
// 						},
// 					},
// 				},
// 				Status: vshnv1.VSHNPostgreSQLStatus{
// 					CurrentVersion: "15",
// 				},
// 			},
// 			old: &vshnv1.VSHNPostgreSQL{
// 				Spec: vshnv1.VSHNPostgreSQLSpec{
// 					Parameters: vshnv1.VSHNPostgreSQLParameters{
// 						Service: vshnv1.VSHNPostgreSQLServiceSpec{
// 							MajorVersion: "15",
// 						},
// 					},
// 				},
// 				Status: vshnv1.VSHNPostgreSQLStatus{
// 					CurrentVersion: "15",
// 				},
// 			},
// 			expectErrList: field.ErrorList{
// 				field.Forbidden(
// 					field.NewPath("spec.parameters.service.majorVersion"),
// 					"only one major version upgrade at a time is allowed",
// 				),
// 			},
// 		},
// 		{
// 			name: "GivenNonHA_ThenNoError",
// 			new: &vshnv1.VSHNPostgreSQL{
// 				Spec: vshnv1.VSHNPostgreSQLSpec{
// 					Parameters: vshnv1.VSHNPostgreSQLParameters{
// 						Instances: 1,
// 						Service: vshnv1.VSHNPostgreSQLServiceSpec{
// 							MajorVersion: "16",
// 						},
// 					},
// 				},
// 				Status: vshnv1.VSHNPostgreSQLStatus{
// 					CurrentVersion: "15",
// 				},
// 			},
// 			old: &vshnv1.VSHNPostgreSQL{
// 				Spec: vshnv1.VSHNPostgreSQLSpec{
// 					Parameters: vshnv1.VSHNPostgreSQLParameters{
// 						Instances: 1,
// 						Service: vshnv1.VSHNPostgreSQLServiceSpec{
// 							MajorVersion: "15",
// 						},
// 					},
// 				},
// 				Status: vshnv1.VSHNPostgreSQLStatus{
// 					CurrentVersion: "15",
// 				},
// 			},
// 			expectErrList: nil,
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			err := validateMajorVersionUpgrade(tt.new, tt.old)
// 			assert.Equal(t, tt.expectErrList, err)
// 		})
// 	}
// }
