package vshnmariadbcontroller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/stretchr/testify/require"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/vshn/appcat/v4/pkg/sliexporter/probes"
)

var _ probeManager = &fakeProbeManager{}

type fakeProbeManager struct {
	probers map[key]bool
}

func newFakeProbeManager() *fakeProbeManager {
	return &fakeProbeManager{
		probers: map[key]bool{},
	}
}

type key string

// StartProbe implements probeManager
func (m *fakeProbeManager) StartProbe(p probes.Prober) {
	m.probers[getFakeKey(p.GetInfo())] = true
}

// StopProbe implements probeManager
func (m *fakeProbeManager) StopProbe(p probes.ProbeInfo) {
	m.probers[getFakeKey(p)] = false
}

func getFakeKey(pi probes.ProbeInfo) key {
	return key(fmt.Sprintf("%s; %s", pi.Service, pi.Name))
}

func TestVSHNMariaDB_Reconcile(t *testing.T) {
	mariadb, ns := newTestVSHNMariaDB("bar", "foo", "cred")

	r, manager, client := setupVSHNMariaDBTest(t,
		mariadb, ns,
		newTestVSHNMariaDBCred("bar", "cred"))

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "bar",
			Name:      "foo",
		},
	}
	pi := probes.ProbeInfo{
		Service: "VSHNMariaDB",
		Name:    "foo",
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.True(t, manager.probers[getFakeKey(pi)])

	require.NoError(t, client.Delete(context.TODO(), mariadb))
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, manager.probers[getFakeKey(pi)])
}

func TestVSHNMariaDB_Suspended_Dont_Probe(t *testing.T) {
	mariadb, ns := newTestVSHNMariaDB("bar", "foo", "creds")
	mariadb.Spec.Parameters.Instances = 0
	r, manager, _ := setupVSHNMariaDBTest(t,
		mariadb, ns,
		newTestVSHNMariaDBCred("bar", "creds"),
	)
	pi := probes.ProbeInfo{
		Service: "VSHNMariaDB",
		Name:    "foo",
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "bar",
			Name:      "foo",
		},
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, manager.probers[getFakeKey(pi)])
}

func TestVSHNMariaDB_Suspended_Stop_Running_Probe(t *testing.T) {
	mariadb, ns := newTestVSHNMariaDB("bar", "foo", "creds")
	r, manager, client := setupVSHNMariaDBTest(t,
		mariadb, ns,
		newTestVSHNMariaDBCred("bar", "creds"),
	)
	pi := probes.ProbeInfo{
		Service: "VSHNMariaDB",
		Name:    "foo",
	}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "bar",
			Name:      "foo",
		},
	}

	// Start with instances=1, probe should start
	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.True(t, manager.probers[getFakeKey(pi)])

	// Scale down to instances=0, probe should stop
	mariadb.Spec.Parameters.Instances = 0
	require.NoError(t, client.Update(context.TODO(), mariadb))
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, manager.probers[getFakeKey(pi)])
}

func setupVSHNMariaDBTest(t *testing.T, objs ...client.Object) (VSHNMariaDBReconciler, *fakeProbeManager, client.Client) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, vshnv1.AddToScheme(scheme))
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		Build()

	manager := newFakeProbeManager()
	r := VSHNMariaDBReconciler{
		Client:             client,
		Scheme:             scheme,
		StartupGracePeriod: 5 * time.Minute,
		ProbeManager:       manager,
		ScClient:           client,
	}

	return r, manager, client
}

func newTestVSHNMariaDB(namespace, name, cred string) (*vshnv1.XVSHNMariaDB, *corev1.Namespace) {
	claim := &vshnv1.XVSHNMariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: vshnv1.XVSHNMariaDBSpec{
			Parameters: vshnv1.VSHNMariaDBParameters{
				Instances: 1,
			},
			ResourceSpec: xpv1.ResourceSpec{
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name:      cred,
					Namespace: namespace,
				},
			},
		},
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: claim.GetInstanceNamespace(),
		},
	}

	return claim, ns
}

func newTestVSHNMariaDBCred(namespace, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"MARIADB_HOST":     []byte("mariadb-widera-skrwg.vshn-mariadb-mariadb-widera-skrwg.svc.cluster.local"),
			"MARIADB_PASSWORD": []byte("iiiUV6n3cdDW20Zt"),
			"MARIADB_PORT":     []byte("3306"),
			"MARIADB_URL":      []byte("mysql://root:iiiUV6n3cdDW20Zt@mariadb-widera-skrwg.vshn-mariadb-mariadb-widera-skrwg.svc.cluster.local:3306"),
			"MARIADB_USERNAME": []byte("root"),
			"ca.crt": []byte(`-----BEGIN CERTIFICATE-----
MIIB3TCCAYOgAwIBAgIRAK2AWokJvb9o1OXYbU8CueYwCgYIKoZIzj0EAwIwTjEX
MBUGA1UEChMOdnNobi1hcHBjYXQtY2ExMzAxBgNVBAMTKmtleWNsb2FrLWFwcDEt
cHJvZC01aHA2NC1rZXljbG9ha3gtaHR0cC1jYTAeFw0yNDA0MDMxMjE3MjNaFw0z
NDA0MDExMjE3MjNaME4xFzAVBgNVBAoTDnZzaG4tYXBwY2F0LWNhMTMwMQYDVQQD
EyprZXljbG9hay1hcHAxLXByb2QtNWhwNjQta2V5Y2xvYWt4LWh0dHAtY2EwWTAT
BgcqhkjOPQIBBggqhkjOPQMBBwNCAAQ9dP4bMvhr8ESprfX7Y6jlCUhOvlFeqd3S
v1sJuYCqYBTf87fg+pDOMAzsubdn8jyJUf65WwhcN2fOV4MesDZXo0IwQDAOBgNV
HQ8BAf8EBAMCAqQwDwYDVR0TAQH/BAUwAwEB/zAdBgNVHQ4EFgQUvQClXJloKXVG
M11dLmi/FAOusZUwCgYIKoZIzj0EAwIDSAAwRQIhAOwQ5NAuEz7TQ5dEy41d7TFm
hbzWn3LKJJs7R13dKYJ8AiANYhF7QtPLyGxIkheciQsP+lQA+Yg4dfTfkgguaXHJ
rQ==
-----END CERTIFICATE-----`),
		},
	}
}
