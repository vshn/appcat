package vshnpostgresqlcontroller

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/sliexporter/probes"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
)

func TestVSHNPostgreSQL_StartStop(t *testing.T) {
	db, ns := newTestVSHNPostgres("bar", "foo", "creds", 1)
	r, manager, client := setupVSHNPostgreTest(t,
		db, ns,
		newTestVSHNPostgresCred("bar", "creds"),
	)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "bar",
			Name:      "foo",
		},
	}
	pi := probes.ProbeInfo{
		Service: "VSHNPostgreSQL",
		Name:    "foo",
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.True(t, manager.probers[getFakeKey(pi)])

	require.NoError(t, client.Delete(context.TODO(), db))
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, manager.probers[getFakeKey(pi)])
}

func TestVSHNPostgreSQL_StartStop_WithFinalizer(t *testing.T) {
	db, ns := newTestVSHNPostgres("bar", "foo", "creds", 1)
	db.SetFinalizers([]string{"foobar.vshn.io"})
	r, manager, client := setupVSHNPostgreTest(t,
		db, ns,
		newTestVSHNPostgresCred("bar", "creds"),
	)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "bar",
			Name:      "foo",
		},
	}
	pi := probes.ProbeInfo{
		Service: "VSHNPostgreSQL",
		Name:    "foo",
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.True(t, manager.probers[getFakeKey(pi)])

	require.NoError(t, client.Delete(context.TODO(), db))
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, manager.probers[getFakeKey(pi)])
}

func TestVSHNPostgreSQL_Multi(t *testing.T) {
	dbBar, nsBar := newTestVSHNPostgres("bar", "foo", "creds", 1)
	dbBarer, nsBarer := newTestVSHNPostgres("bar", "fooer", "credentials", 1)
	dbBuzz, nsBuzz := newTestVSHNPostgres("buzz", "foobar", "creds", 1)
	r, manager, c := setupVSHNPostgreTest(t,
		dbBar, nsBar,
		newTestVSHNPostgresCred("bar", "creds"),
		dbBarer, nsBarer,
		newTestVSHNPostgresCred("bar", "credentials"),
		dbBuzz, nsBuzz,
		newTestVSHNPostgresCred("buzz", "creds"),
	)

	barPi := probes.ProbeInfo{
		Service: "VSHNPostgreSQL",
		Name:    "foo",
	}
	barerPi := probes.ProbeInfo{
		Service: "VSHNPostgreSQL",
		Name:    "fooer",
	}
	buzzPi := probes.ProbeInfo{
		Service: "VSHNPostgreSQL",
		Name:    "foobar",
	}

	_, err := r.Reconcile(context.TODO(), recReq("bar", "foo"))
	require.NoError(t, err)
	_, err = r.Reconcile(context.TODO(), recReq("bar", "fooer"))
	require.NoError(t, err)

	require.True(t, manager.probers[getFakeKey(barPi)])
	require.True(t, manager.probers[getFakeKey(barerPi)])
	require.False(t, manager.probers[getFakeKey(buzzPi)])

	require.NoError(t, c.Delete(context.TODO(), dbBar))
	_, err = r.Reconcile(context.TODO(), recReq("bar", "foo"))
	require.NoError(t, err)
	_, err = r.Reconcile(context.TODO(), recReq("buzz", "foobar"))
	require.NoError(t, err)

	require.False(t, manager.probers[getFakeKey(barPi)])
	require.True(t, manager.probers[getFakeKey(barerPi)])
	require.True(t, manager.probers[getFakeKey(buzzPi)])
}

func TestVSHNPostgreSQL_Startup_NoCreds_Dont_Probe(t *testing.T) {
	db, ns := newTestVSHNPostgres("bar", "foo", "creds", 1)
	cd := metav1.Now().Add(time.Minute * -6)
	db.SetCreationTimestamp(metav1.Time{Time: cd})
	r, manager, _ := setupVSHNPostgreTest(t,
		db, ns,
	)
	pi := probes.ProbeInfo{
		Service:   "VSHNPostgreSQL",
		Name:      "foo",
		Namespace: "bar",
	}

	res, err := r.Reconcile(context.TODO(), recReq("bar", "foo"))
	assert.NoError(t, err)
	assert.Greater(t, res.RequeueAfter.Microseconds(), int64(0))

	// we now get a failing probe as the composite will always have
	// connection details
	assert.True(t, manager.probers[getFakeKey(pi)])
}

func TestVSHNPostgreSQL_NoRef_Dont_Probe(t *testing.T) {
	db, ns := newTestVSHNPostgres("bar", "foo", "creds", 1)
	db.Spec.WriteConnectionSecretToReference.Name = ""
	r, manager, _ := setupVSHNPostgreTest(t,
		db, ns,
	)
	pi := probes.ProbeInfo{
		Service:   "VSHNPostgreSQL",
		Name:      "foo",
		Namespace: "bar",
	}

	_, err := r.Reconcile(context.TODO(), recReq("bar", "foo"))
	assert.NoError(t, err)
	assert.False(t, manager.probers[getFakeKey(pi)])
}

func TestVSHNPostgreSQL_Started_NoCreds_Probe_Failure(t *testing.T) {
	db, ns := newTestVSHNPostgres("bar", "foo", "creds", 1)
	db.SetCreationTimestamp(metav1.Time{Time: time.Now().Add(-1 * time.Hour)})
	r, manager, _ := setupVSHNPostgreTest(t,
		db, ns,
	)
	pi := probes.ProbeInfo{
		Service: "VSHNPostgreSQL",
		Name:    "foo",
	}

	res, err := r.Reconcile(context.TODO(), recReq("bar", "foo"))
	assert.NoError(t, err)
	assert.Greater(t, res.RequeueAfter.Microseconds(), int64(0))

	assert.True(t, manager.probers[getFakeKey(pi)])
}

func TestVSHNPostgreSQL_PassCredentials(t *testing.T) {
	db, ns := newTestVSHNPostgres("bar", "foo", "creds", 3)
	cred := newTestVSHNPostgresCred("bar", "creds")
	cred.Data = map[string][]byte{
		"POSTGRESQL_USER":     []byte("userfoo"),
		"POSTGRESQL_PASSWORD": []byte("password"),
		"POSTGRESQL_HOST":     []byte("foo.bar"),
		"POSTGRESQL_PORT":     []byte("5433"),
		"POSTGRESQL_DB":       []byte("pg"),
		"ca.crt":              []byte("-----BEGIN CERTIFICATE-----MIICNDCCAaECEAKtZn5ORf5eV288mBle3cAwDQYJKoZIhvcNAQECBQAwXzELMAkG..."),
	}
	r, manager, client := setupVSHNPostgreTest(t,
		db, ns,
		cred,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bar",
				Labels: map[string]string{
					utils.OrgLabelName:   "bar",
					"appcat.vshn.io/sla": "besteffort",
				},
			},
		},
	)
	r.PostgreDialer = func(service, name, namespace, dsn, organization, sla string, ha bool, ops ...func(*pgxpool.Config) error) (*probes.PostgreSQL, error) {

		assert.Equal(t, "VSHNPostgreSQL", service)
		assert.Equal(t, "foo", name)
		assert.Equal(t, "bar", namespace)
		assert.Equal(t, "postgresql://userfoo:password@foo.bar:5433/pg?sslmode=verify-ca", dsn)
		assert.Equal(t, true, ha)
		assert.Equal(t, "bar", organization)
		assert.Equal(t, "besteffort", sla)

		return fakePostgreDialer(service, name, namespace, dsn, organization, sla, ha, ops...)
	}
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "bar",
			Name:      "foo",
		},
	}
	pi := probes.ProbeInfo{
		Service: "VSHNPostgreSQL",
		Name:    "foo",
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.True(t, manager.probers[getFakeKey(pi)])

	require.NoError(t, client.Delete(context.TODO(), db))
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, manager.probers[getFakeKey(pi)])
}

func fakePostgreDialer(service string, name string, namespace string, dsn string, organization string, sla string, ha bool, ops ...func(*pgxpool.Config) error) (*probes.PostgreSQL, error) {
	p := &probes.PostgreSQL{
		Service:       service,
		Name:          name,
		Namespace:     namespace,
		Organization:  organization,
		HighAvailable: ha,
		ServiceLevel:  sla,
	}
	return p, nil
}

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

func setupVSHNPostgreTest(t *testing.T, objs ...client.Object) (VSHNPostgreSQLReconciler, *fakeProbeManager, client.Client) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, vshnv1.AddToScheme(scheme))
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		Build()

	manager := newFakeProbeManager()
	r := VSHNPostgreSQLReconciler{
		Client:             client,
		Scheme:             scheme,
		ProbeManager:       manager,
		StartupGracePeriod: 5 * time.Minute,
		PostgreDialer:      fakePostgreDialer,
		ScClient:           client,
	}

	return r, manager, client
}

func newTestVSHNPostgres(namespace, name, cred string, instances int) (*vshnv1.XVSHNPostgreSQL, *corev1.Namespace) {
	claim := &vshnv1.XVSHNPostgreSQL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: vshnv1.XVSHNPostgreSQLSpec{
			Parameters: vshnv1.VSHNPostgreSQLParameters{
				Instances: instances,
			},
			ResourceSpec: xpv1.ResourceSpec{
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name: cred,
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
func newTestVSHNPostgresCred(namespace, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"POSTGRESQL_USER":     []byte("user"),
			"POSTGRESQL_PASSWORD": []byte("password"),
			"POSTGRESQL_HOST":     []byte("foo.bar"),
			"POSTGRESQL_PORT":     []byte("5432"),
			"POSTGRESQL_DB":       []byte("pg"),
			"ca.crt":              []byte("-----BEGIN CERTIFICATE-----MIICNDCCAaECEAKtZn5ORf5eV288mBle3cAwDQYJKoZIhvcNAQECBQAwXzELMAkG..."),
		},
	}
}

func recReq(namespace, name string) ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      name,
		},
	}
}
