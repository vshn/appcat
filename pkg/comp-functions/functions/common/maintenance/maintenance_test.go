package maintenance

import (
	"context"
	"reflect"
	"testing"

	xkubev1 "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/stretchr/testify/assert"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func Test_parseCron(t *testing.T) {
	tests := []struct {
		name    string
		comp    *vshnv1.VSHNPostgreSQL
		want    string
		wantErr bool
	}{
		{
			name: "GivenNormaleSchedule_ThenExpectCronExpression",
			comp: &vshnv1.VSHNPostgreSQL{
				Spec: vshnv1.VSHNPostgreSQLSpec{
					Parameters: vshnv1.VSHNPostgreSQLParameters{
						Maintenance: vshnv1.VSHNDBaaSMaintenanceScheduleSpec{
							DayOfWeek: "tuesday",
							TimeOfDay: "23:32:00",
						},
					},
				},
			},
			want: "32 23 * * 2",
		},
		{
			name: "GivenEmptySchedule_ThenExpectEmptyExpression",
			comp: &vshnv1.VSHNPostgreSQL{
				Spec: vshnv1.VSHNPostgreSQLSpec{
					Parameters: vshnv1.VSHNPostgreSQLParameters{
						Maintenance: vshnv1.VSHNDBaaSMaintenanceScheduleSpec{
							DayOfWeek: "",
							TimeOfDay: "",
						},
					},
				},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Maintenance{
				schedule: tt.comp.Spec.Parameters.Maintenance,
			}
			got, err := m.parseCron()
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCron() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseCron() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddMaintenanceJob(t *testing.T) {
	tests := []struct {
		name          string
		want          *xfnproto.Result
		wantedSa      bool
		wantedRole    bool
		wantedBinding bool
		wantedSecret  bool
		wantedJob     bool
		fileName      string
	}{
		{
			name:          "GivenSchedule_ThenExpectMaintenanceObjects",
			want:          nil,
			wantedSa:      true,
			wantedRole:    true,
			wantedBinding: true,
			wantedSecret:  true,
			wantedJob:     true,
			fileName:      "vshn-postgres/maintenance/01-GivenSchedule.yaml",
		},
		{
			name: "GivenNoSchedule_ThenExpectNoObjects",
			want: &xfnproto.Result{
				Severity: xfnproto.Severity_SEVERITY_NORMAL,
				Message:  "Maintenance schedule not yet populated",
			},
			fileName: "vshn-postgres/maintenance/02-GivenNoSchedule.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			namePrefix := "pgsql-gc9x4-"

			ctx := context.TODO()

			svc := commontest.LoadRuntimeFromFile(t, tt.fileName)

			comp := &vshnv1.VSHNPostgreSQL{}
			err := svc.GetObservedComposite(comp)
			assert.NoError(t, err)

			in := "vshn-postgresql-" + comp.GetName()
			m := New(comp, svc, comp.Spec.Parameters.Maintenance, in, "postgresql").
				WithPolicyRules([]rbacv1.PolicyRule{}).
				WithRole("crossplane:appcat:job:postgres:maintenance").
				WithExtraResources(createMaintenanceSecretTest(in, svc.Config.Data["sgNamespace"], comp.GetName()+"-maintenance-secret"))

			if got := m.Run(ctx); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AddMaintenanceJob() = %v, want %v", got, tt.want)
			}

			sa := &corev1.ServiceAccount{}

			err = svc.GetDesiredKubeObject(sa, namePrefix+"maintenance-serviceaccount")

			if tt.wantedSa {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

			role := &rbacv1.Role{}

			err = svc.GetDesiredKubeObject(role, namePrefix+"maintenance-role")

			if tt.wantedRole {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

			binding := &rbacv1.RoleBinding{}

			err = svc.GetDesiredKubeObject(binding, namePrefix+"maintenance-rolebinding")

			if tt.wantedBinding {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

			secret := &corev1.Secret{}

			err = svc.GetDesiredKubeObject(secret, namePrefix+"maintenance-secret")

			if tt.wantedSecret {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

			job := &batchv1.CronJob{}

			err = svc.GetDesiredKubeObject(job, namePrefix+"maintenancejob")

			if tt.wantedJob {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

		})
	}
}

func createMaintenanceSecretTest(instanceNamespace, sgNamespace, resourceName string) ExtraResource {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test secret",
			Namespace: instanceNamespace,
		},
		StringData: map[string]string{
			"SG_NAMESPACE": sgNamespace,
		},
	}

	ref := xkubev1.Reference{
		PatchesFrom: &xkubev1.PatchesFrom{
			DependsOn: xkubev1.DependsOn{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "stackgres-restapi-admin",
				Namespace:  sgNamespace,
			},
			FieldPath: pointer.String("data"),
		},
		ToFieldPath: pointer.String("data"),
	}
	return ExtraResource{
		Name:     resourceName,
		Resource: secret,
		Refs: []xkubev1.Reference{
			ref,
		},
	}
}
