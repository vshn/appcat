package backup

import (
	"testing"

	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"k8s.io/utils/ptr"
)

func TestK8upBackupSpec_IsEnabled(t *testing.T) {
	tests := []struct {
		name   string
		backup vshnv1.K8upBackupSpec
		want   bool
	}{
		{
			name:   "Default (nil) should be enabled",
			backup: vshnv1.K8upBackupSpec{},
			want:   true,
		},
		{
			name: "Explicit true should be enabled",
			backup: vshnv1.K8upBackupSpec{
				Enabled: ptr.To(true),
			},
			want: true,
		},
		{
			name: "Explicit false should be disabled",
			backup: vshnv1.K8upBackupSpec{
				Enabled: ptr.To(false),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.backup.IsEnabled())
		})
	}
}

func TestVSHNPostgreSQLBackup_IsEnabled(t *testing.T) {
	tests := []struct {
		name   string
		backup vshnv1.VSHNPostgreSQLBackup
		want   bool
	}{
		{
			name:   "Default (nil) should be enabled",
			backup: vshnv1.VSHNPostgreSQLBackup{},
			want:   true,
		},
		{
			name: "Explicit true should be enabled",
			backup: vshnv1.VSHNPostgreSQLBackup{
				Enabled: ptr.To(true),
			},
			want: true,
		},
		{
			name: "Explicit false should be disabled",
			backup: vshnv1.VSHNPostgreSQLBackup{
				Enabled: ptr.To(false),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.backup.IsEnabled())
		})
	}
}

func TestVSHNPostgreSQL_IsBackupEnabled(t *testing.T) {
	tests := []struct {
		name string
		pg   vshnv1.VSHNPostgreSQL
		want bool
	}{
		{
			name: "Default should be enabled",
			pg:   vshnv1.VSHNPostgreSQL{},
			want: true,
		},
		{
			name: "Explicit true should be enabled",
			pg: vshnv1.VSHNPostgreSQL{
				Spec: vshnv1.VSHNPostgreSQLSpec{
					Parameters: vshnv1.VSHNPostgreSQLParameters{
						Backup: vshnv1.VSHNPostgreSQLBackup{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			want: true,
		},
		{
			name: "Explicit false should be disabled",
			pg: vshnv1.VSHNPostgreSQL{
				Spec: vshnv1.VSHNPostgreSQLSpec{
					Parameters: vshnv1.VSHNPostgreSQLParameters{
						Backup: vshnv1.VSHNPostgreSQLBackup{
							Enabled: ptr.To(false),
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.pg.IsBackupEnabled())
		})
	}
}

func TestVSHNRedis_IsBackupEnabled(t *testing.T) {
	tests := []struct {
		name  string
		redis vshnv1.VSHNRedis
		want  bool
	}{
		{
			name:  "Default should be enabled",
			redis: vshnv1.VSHNRedis{},
			want:  true,
		},
		{
			name: "Explicit false should be disabled",
			redis: vshnv1.VSHNRedis{
				Spec: vshnv1.VSHNRedisSpec{
					Parameters: vshnv1.VSHNRedisParameters{
						Backup: vshnv1.K8upBackupSpec{
							Enabled: ptr.To(false),
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.redis.IsBackupEnabled())
		})
	}
}

func TestVSHNMariaDB_IsBackupEnabled(t *testing.T) {
	tests := []struct {
		name    string
		mariadb vshnv1.VSHNMariaDB
		want    bool
	}{
		{
			name:    "Default should be enabled",
			mariadb: vshnv1.VSHNMariaDB{},
			want:    true,
		},
		{
			name: "Explicit true should be enabled",
			mariadb: vshnv1.VSHNMariaDB{
				Spec: vshnv1.VSHNMariaDBSpec{
					Parameters: vshnv1.VSHNMariaDBParameters{
						Backup: vshnv1.K8upBackupSpec{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			want: true,
		},
		{
			name: "Explicit false should be disabled",
			mariadb: vshnv1.VSHNMariaDB{
				Spec: vshnv1.VSHNMariaDBSpec{
					Parameters: vshnv1.VSHNMariaDBParameters{
						Backup: vshnv1.K8upBackupSpec{
							Enabled: ptr.To(false),
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.mariadb.IsBackupEnabled())
		})
	}
}

func TestVSHNKeycloak_IsBackupEnabled(t *testing.T) {
	tests := []struct {
		name     string
		keycloak vshnv1.VSHNKeycloak
		want     bool
	}{
		{
			name:     "Default should be enabled",
			keycloak: vshnv1.VSHNKeycloak{},
			want:     true,
		},
		{
			name: "Explicit true should be enabled",
			keycloak: vshnv1.VSHNKeycloak{
				Spec: vshnv1.VSHNKeycloakSpec{
					Parameters: vshnv1.VSHNKeycloakParameters{
						Backup: vshnv1.K8upBackupSpec{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			want: true,
		},
		{
			name: "Explicit false should be disabled",
			keycloak: vshnv1.VSHNKeycloak{
				Spec: vshnv1.VSHNKeycloakSpec{
					Parameters: vshnv1.VSHNKeycloakParameters{
						Backup: vshnv1.K8upBackupSpec{
							Enabled: ptr.To(false),
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.keycloak.IsBackupEnabled())
		})
	}
}

func TestVSHNForgejo_IsBackupEnabled(t *testing.T) {
	tests := []struct {
		name    string
		forgejo vshnv1.VSHNForgejo
		want    bool
	}{
		{
			name:    "Default should be enabled",
			forgejo: vshnv1.VSHNForgejo{},
			want:    true,
		},
		{
			name: "Explicit true should be enabled",
			forgejo: vshnv1.VSHNForgejo{
				Spec: vshnv1.VSHNForgejoSpec{
					Parameters: vshnv1.VSHNForgejoParameters{
						Backup: vshnv1.K8upBackupSpec{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			want: true,
		},
		{
			name: "Explicit false should be disabled",
			forgejo: vshnv1.VSHNForgejo{
				Spec: vshnv1.VSHNForgejoSpec{
					Parameters: vshnv1.VSHNForgejoParameters{
						Backup: vshnv1.K8upBackupSpec{
							Enabled: ptr.To(false),
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.forgejo.IsBackupEnabled())
		})
	}
}

func TestVSHNNextcloud_IsBackupEnabled(t *testing.T) {
	tests := []struct {
		name      string
		nextcloud vshnv1.VSHNNextcloud
		want      bool
	}{
		{
			name:      "Default should be enabled",
			nextcloud: vshnv1.VSHNNextcloud{},
			want:      true,
		},
		{
			name: "Explicit true should be enabled",
			nextcloud: vshnv1.VSHNNextcloud{
				Spec: vshnv1.VSHNNextcloudSpec{
					Parameters: vshnv1.VSHNNextcloudParameters{
						Backup: vshnv1.VSHNNextcloudBackupSpec{
							K8upBackupSpec: vshnv1.K8upBackupSpec{
								Enabled: ptr.To(true),
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "Explicit false should be disabled",
			nextcloud: vshnv1.VSHNNextcloud{
				Spec: vshnv1.VSHNNextcloudSpec{
					Parameters: vshnv1.VSHNNextcloudParameters{
						Backup: vshnv1.VSHNNextcloudBackupSpec{
							K8upBackupSpec: vshnv1.K8upBackupSpec{
								Enabled: ptr.To(false),
							},
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.nextcloud.IsBackupEnabled())
		})
	}
}

func TestVSHNMinio_IsBackupEnabled(t *testing.T) {
	tests := []struct {
		name  string
		minio vshnv1.VSHNMinio
		want  bool
	}{
		{
			name:  "Always returns false (MinIO doesn't support K8up backups)",
			minio: vshnv1.VSHNMinio{},
			want:  false,
		},
		{
			name: "Always returns false even with backup enabled in spec",
			minio: vshnv1.VSHNMinio{
				Spec: vshnv1.VSHNMinioSpec{
					Parameters: vshnv1.VSHNMinioParameters{
						Backup: vshnv1.K8upBackupSpec{
							Enabled: ptr.To(true),
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.minio.IsBackupEnabled())
		})
	}
}
