package vshnnextcloud

import (
	"context"
	_ "embed"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	xkube "github.com/vshn/appcat/v4/apis/kubernetes/v1alpha2"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/commontest"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_getPgInstanceNamespace(t *testing.T) {
	tests := []struct {
		host    string
		want    string
		wantErr bool
	}{
		{host: "myinstance.vshn-postgresql-myinstance.svc.cluster.local", want: "vshn-postgresql-myinstance"},
		{host: "pg.vshn-postgresql-db.svc\n", want: "vshn-postgresql-db"},
		{host: "pg.customer-db.svc.cluster.local", wantErr: true},
		{host: "dns.kube-system.svc.cluster.local", wantErr: true},
		{host: "pg.vshn-postgresql-db", wantErr: true},
		{host: "db.example.com", wantErr: true},
		{host: "10.0.0.5", wantErr: true},
		{host: "pg.customer-db", wantErr: true},
		{host: "nodots", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			ns, err := getPgInstanceNamespace(tt.host)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, ns)
		})
	}
}

func Test_establishConnectionToExistingPG(t *testing.T) {
	namespace := func(name string, labels map[string]string) *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
	}
	org := utils.OrgLabelName
	claimNs := "appcat.vshn.io/claim-namespace"

	tests := []struct {
		name  string
		noOrg bool
		host  string
		pgNs  *corev1.Namespace
		want  string
	}{
		{name: "ExternalHost", host: "db.example.com"},
		{name: "ExternalHostNoOrg", noOrg: true, host: "db.example.com"},
		{name: "OtherNamespaceNoOrg", noOrg: true, host: "pg.acme-db.svc.cluster.local"},
		{name: "KubeSystemNoOrg", noOrg: true, host: "dns.kube-system.svc.cluster.local"},
		{name: "PostgresqlNamespaceNoOrg", noOrg: true, host: "pg.vshn-postgresql-db.svc.cluster.local", want: "vshn-postgresql-db"},
		{name: "KubeSystem", host: "dns.kube-system.svc.cluster.local", pgNs: namespace("kube-system", map[string]string{org: "vshn"})},
		{name: "NotObservedYet", host: "pg.vshn-postgresql-db.svc.cluster.local"},
		{name: "SameOrg", host: "pg.vshn-postgresql-db.svc.cluster.local", pgNs: namespace("vshn-postgresql-db", map[string]string{org: "vshn"}), want: "vshn-postgresql-db"},
		{name: "SameClaimNamespace", host: "pg.vshn-postgresql-own.svc.cluster.local", pgNs: namespace("vshn-postgresql-own", map[string]string{claimNs: "unit-test"}), want: "vshn-postgresql-own"},
		{name: "OtherTenantInstance", host: "pg.vshn-postgresql-victim.svc.cluster.local", pgNs: namespace("vshn-postgresql-victim", map[string]string{claimNs: "other-tenant", org: "acme"})},
		{name: "NoLabels", host: "pg.vshn-postgresql-db.svc.cluster.local", pgNs: namespace("vshn-postgresql-db", nil)},
		{name: "StaleObservation", host: "pg.vshn-postgresql-acme.svc.cluster.local", pgNs: namespace("vshn-postgresql-db", map[string]string{org: "vshn"})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, comp := getNextcloudComp(t, "vshnnextcloud/04_existing_pg.yaml")
			if tt.noOrg {
				setObservedKubeObject(t, svc, comp.GetName()+"-claim-ns-observer", namespace("unit-test", nil))
			}
			setObservedKubeObject(t, svc, comp.GetName()+"-postgresql-connection-secret-claim-observer", &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "tenant-pg", Namespace: "unit-test"},
				Data:       map[string][]byte{"POSTGRESQL_HOST": []byte(tt.host)},
			})
			if tt.pgNs != nil {
				setObservedKubeObject(t, svc, comp.GetName()+"-pg-namespace-observer", tt.pgNs)
			}

			pgSecret, err := establishConnectionToExistingPG(context.TODO(), comp, svc)
			assert.NoError(t, err)
			assert.Equal(t, "tenant-pg", pgSecret)

			netpolName := comp.GetName() + "-netpol-allow-to-pg"
			netpol := &netv1.NetworkPolicy{}
			err = svc.GetDesiredKubeObject(netpol, netpolName)
			if tt.want == "" {
				assert.ErrorIs(t, err, runtime.ErrNotFound)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, netpol.GetNamespace())
				obj := &xkube.Object{}
				require.NoError(t, svc.GetDesiredComposedResourceByName(obj, netpolName))
				assert.Equal(t, "true", obj.GetLabels()[runtime.WebhookAllowDeletionLabel])
			}

			if tt.noOrg {
				assert.ErrorIs(t, svc.GetDesiredKubeObject(&corev1.Namespace{}, comp.GetName()+"-pg-namespace-observer"), runtime.ErrNotFound)
			}
		})
	}
}

func Test_establishConnectionToExistingPG_releasesNetworkPolicy(t *testing.T) {
	tests := []struct {
		name         string
		host         string
		labels       map[string]any
		pgNsObserved bool
		wantKept     bool
	}{
		{name: "ProtectedPolicyGetsLabel", host: "pg.vshn-postgresql-victim.svc.cluster.local", pgNsObserved: true, wantKept: true},
		{name: "ProtectedPolicyNamespaceNotObservedYet", host: "pg.vshn-postgresql-victim.svc.cluster.local", wantKept: true},
		{name: "ProtectedPolicyOtherNamespace", host: "dns.kube-system.svc.cluster.local", wantKept: true},
		{name: "LabelledPolicyIsDropped", host: "pg.vshn-postgresql-victim.svc.cluster.local", pgNsObserved: true, labels: map[string]any{runtime.WebhookAllowDeletionLabel: "true"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, comp := getNextcloudComp(t, "vshnnextcloud/04_existing_pg.yaml")
			netpolName := comp.GetName() + "-netpol-allow-to-pg"
			setObservedKubeObject(t, svc, comp.GetName()+"-postgresql-connection-secret-claim-observer", &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "tenant-pg", Namespace: "unit-test"},
				Data:       map[string][]byte{"POSTGRESQL_HOST": []byte(tt.host)},
			})
			if tt.pgNsObserved {
				setObservedKubeObject(t, svc, comp.GetName()+"-pg-namespace-observer", &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{Name: "vshn-postgresql-victim", Labels: map[string]string{utils.OrgLabelName: "acme"}},
				})
			}
			existing := &netv1.NetworkPolicy{
				TypeMeta:   metav1.TypeMeta{APIVersion: "networking.k8s.io/v1", Kind: "NetworkPolicy"},
				ObjectMeta: metav1.ObjectMeta{Name: "allow-from-" + comp.GetInstanceNamespace(), Namespace: "vshn-postgresql-victim"},
			}
			manifest, err := json.Marshal(existing)
			require.NoError(t, err)
			raw, err := json.Marshal(map[string]any{
				"apiVersion": "kubernetes.crossplane.io/v1alpha2",
				"kind":       "Object",
				"metadata":   map[string]any{"name": netpolName, "labels": tt.labels},
				"spec":       map[string]any{"forProvider": map[string]any{"manifest": json.RawMessage(manifest)}},
			})
			require.NoError(t, err)
			res := &structpb.Struct{}
			require.NoError(t, protojson.Unmarshal(raw, res))
			svc.GetRequest().Observed.Resources[netpolName] = &xfnproto.Resource{Resource: res}

			_, err = establishConnectionToExistingPG(context.TODO(), comp, svc)
			require.NoError(t, err)

			obj := &xkube.Object{}
			err = svc.GetDesiredComposedResourceByName(obj, netpolName)
			if !tt.wantKept {
				assert.ErrorIs(t, err, runtime.ErrNotFound)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "true", obj.GetLabels()[runtime.WebhookAllowDeletionLabel])
			netpol := &netv1.NetworkPolicy{}
			require.NoError(t, svc.GetDesiredKubeObject(netpol, netpolName))
			assert.Equal(t, "vshn-postgresql-victim", netpol.GetNamespace())
		})
	}
}

func Test_addNextcloudExternalPG(t *testing.T) {
	for _, noOrg := range []bool{false, true} {
		t.Run(strconv.FormatBool(noOrg), func(t *testing.T) {
			svc, comp := getNextcloudComp(t, "vshnnextcloud/04_existing_pg.yaml")
			if noOrg {
				setObservedKubeObject(t, svc, comp.GetName()+"-claim-ns-observer", &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "unit-test"}})
			}

			assert.Nil(t, DeployNextcloud(context.TODO(), &vshnv1.VSHNNextcloud{}, svc))

			release := &xhelmv1.Release{}
			require.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()+"-release"))
			values := map[string]any{}
			require.NoError(t, json.Unmarshal(release.Spec.ForProvider.Values.Raw, &values))

			extDb := values["externalDatabase"].(map[string]any)
			assert.Equal(t, "tenant-pg", extDb["existingSecret"].(map[string]any)["secretName"])
			for _, v := range values["nextcloud"].(map[string]any)["extraVolumes"].([]any) {
				vol := v.(map[string]any)
				if vol["name"] == "pg-cert" {
					assert.Equal(t, "tenant-pg", vol["secret"].(map[string]any)["secretName"])
				}
			}
			assert.ErrorIs(t, svc.GetDesiredKubeObject(&netv1.NetworkPolicy{}, comp.GetName()+"-netpol-allow-to-pg"), runtime.ErrNotFound)
		})
	}
}

func setObservedKubeObject(t *testing.T, svc *runtime.ServiceRuntime, name string, obj client.Object) {
	manifest, err := json.Marshal(obj)
	require.NoError(t, err)
	raw, err := json.Marshal(map[string]any{
		"apiVersion": "kubernetes.crossplane.io/v1alpha2",
		"kind":       "Object",
		"metadata":   map[string]any{"name": name},
		"status":     map[string]any{"atProvider": map[string]any{"manifest": json.RawMessage(manifest)}},
	})
	require.NoError(t, err)
	res := &structpb.Struct{}
	require.NoError(t, protojson.Unmarshal(raw, res))
	svc.GetRequest().Observed.Resources[name] = &xfnproto.Resource{Resource: res}
}

func Test_addNextcloud(t *testing.T) {
	svc, comp := getNextcloudComp(t, "vshnnextcloud/01_default.yaml")

	ctx := context.TODO()

	assert.Nil(t, DeployNextcloud(ctx, &vshnv1.VSHNNextcloud{}, svc))

	pg := &vshnv1.XVSHNPostgreSQL{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(pg, comp.GetName()+pgInstanceNameSuffix))

	release := &xhelmv1.Release{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()+"-release"))

	values := map[string]any{}
	assert.NoError(t, json.Unmarshal(release.Spec.ForProvider.Values.Raw, &values))
	extDb := values["externalDatabase"].(map[string]any)
	assert.Equal(t, true, extDb["enabled"])
	intDb := values["internalDatabase"].(map[string]any)
	assert.Equal(t, false, intDb["enabled"])

}

func Test_addReleaseInternalDB(t *testing.T) {
	svc, comp := getNextcloudComp(t, "vshnnextcloud/02_no_pg.yaml")

	ctx := context.TODO()

	assert.Nil(t, DeployNextcloud(ctx, &vshnv1.VSHNNextcloud{}, svc))

	pg := &vshnv1.XVSHNPostgreSQL{}
	assert.Error(t, svc.GetDesiredComposedResourceByName(pg, comp.GetName()+pgInstanceNameSuffix))

	release := &xhelmv1.Release{}
	assert.NoError(t, svc.GetDesiredComposedResourceByName(release, comp.GetName()+"-release"))

	values := map[string]any{}
	assert.NoError(t, json.Unmarshal(release.Spec.ForProvider.Values.Raw, &values))
	extDb := values["externalDatabase"].(map[string]any)
	assert.Equal(t, map[string]any{}, extDb)
	intDb := values["internalDatabase"].(map[string]any)
	assert.Equal(t, true, intDb["enabled"])
}

//go:embed files/vshn-nextcloud.config.php
var testNextcloudConfig string

func Test_setBackgroundJobMaintenance(t *testing.T) {

	tests := []struct {
		name      string
		timeOfDay string
		want      string
	}{
		{
			name:      "10MinEarlierThan20Expect21",
			timeOfDay: "19:50:01",
			want:      "21",
		},
		{
			name:      "21MinEarlierThan20Expect21",
			timeOfDay: "19:39:01",
			want:      "21",
		},
		{
			name:      "50MinEarlierThan20Expect20",
			timeOfDay: "19:10:01",
			want:      "20",
		},
		{
			name:      "1MinAfterThan0Expect1",
			timeOfDay: "00:01:01",
			want:      "1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updatedNextcloudConfig := setBackgroundJobMaintenance(vshnv1.TimeOfDay(tt.timeOfDay), testNextcloudConfig)
			splitted := strings.Split(updatedNextcloudConfig, "'maintenance_window_start' => ")
			actual := strings.Trim(splitted[1][:2], ",\n")
			assert.Equal(t, tt.want, actual)
		})
	}
}

func getNextcloudComp(t *testing.T, file string) (*runtime.ServiceRuntime, *vshnv1.VSHNNextcloud) {
	svc := commontest.LoadRuntimeFromFile(t, file)

	comp := &vshnv1.VSHNNextcloud{}
	err := svc.GetObservedComposite(comp)
	assert.NoError(t, err)

	return svc, comp
}
