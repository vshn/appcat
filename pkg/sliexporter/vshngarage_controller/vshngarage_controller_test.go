package vshngaragecontroller

import (
	"context"
	"fmt"
	"testing"
	"time"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	cpv1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/sliexporter/probes"
	slireconciler "github.com/vshn/appcat/v4/pkg/sliexporter/sli_reconciler"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	garageName       = "test-garage"
	garageClaimNS    = "garagens"
	garageInstanceNS = "vshn-garage-test-garage"

	_ probeManager = &fakeProbeManager{}
)

type fakeProbeManager struct {
	probers map[key]bool
}

func newFakeProbeManager() *fakeProbeManager {
	return &fakeProbeManager{
		probers: map[key]bool{},
	}
}

type key string

func (m *fakeProbeManager) StartProbe(p probes.Prober) {
	m.probers[getFakeKey(p.GetInfo())] = true
}

func (m *fakeProbeManager) StopProbe(p probes.ProbeInfo) {
	m.probers[getFakeKey(p)] = false
}

func getFakeKey(pi probes.ProbeInfo) key {
	return key(fmt.Sprintf("%s; %s", pi.Service, pi.Name))
}

func TestGarageReconciler(t *testing.T) {
	garage, ns := giveMeGarage(garageName, garageClaimNS)

	ct := metav1.Now().Add(-20 * time.Minute)
	garage.CreationTimestamp = metav1.Time{Time: ct}

	r, manager, c := setupVSHNGarageTest(t,
		garage, ns,
		newTestBucketCreds(garageClaimNS, garageName),
		newTestAdminToken(garageInstanceNS),
	)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      garageName,
			Namespace: garageClaimNS,
		},
	}

	probeInfo := probes.ProbeInfo{
		Service:           "VSHNGarage",
		Name:              garageName,
		ClaimNamespace:    garageClaimNS,
		InstanceNamespace: garageInstanceNS,
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.True(t, manager.probers[getFakeKey(probeInfo)])

	require.NoError(t, c.Delete(context.TODO(), garage))
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, manager.probers[getFakeKey(probeInfo)])
}

func TestVSHNGarage_Startup_NoCreds_Dont_Probe(t *testing.T) {
	garage, ns := giveMeGarage(garageName, garageClaimNS)

	r, manager, _ := setupVSHNGarageTest(t, garage, ns)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      garageName,
			Namespace: garageClaimNS,
		},
	}

	probeInfo := probes.ProbeInfo{
		Service:           "VSHNGarage",
		Name:              garageName,
		ClaimNamespace:    garageClaimNS,
		InstanceNamespace: garageInstanceNS,
	}

	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.Greater(t, res.RequeueAfter.Microseconds(), int64(0))
	assert.False(t, manager.probers[getFakeKey(probeInfo)])
}

func TestVSHNGarage_NoRef_Dont_Probe(t *testing.T) {
	garage, ns := giveMeGarage(garageName, garageClaimNS)
	garage.Spec.WriteConnectionSecretToReference.Name = ""

	r, manager, _ := setupVSHNGarageTest(t, garage, ns)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      garageName,
			Namespace: garageClaimNS,
		},
	}

	pi := probes.ProbeInfo{
		Service: "VSHNGarage",
		Name:    garageName,
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, manager.probers[getFakeKey(pi)])
}

func TestVSHNGarage_Suspended_Dont_Probe(t *testing.T) {
	garage, ns := giveMeGarage(garageName, garageClaimNS)
	garage.Spec.Parameters.Instances = 0

	ct := metav1.Now().Add(-20 * time.Minute)
	garage.CreationTimestamp = metav1.Time{Time: ct}

	r, manager, _ := setupVSHNGarageTest(t,
		garage, ns,
		newTestBucketCreds(garageClaimNS, garageName),
		newTestAdminToken(garageInstanceNS),
	)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      garageName,
			Namespace: garageClaimNS,
		},
	}

	pi := probes.ProbeInfo{
		Service: "VSHNGarage",
		Name:    garageName,
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, manager.probers[getFakeKey(pi)])
}

func TestVSHNGarage_Suspended_Stop_Running_Probe(t *testing.T) {
	garage, ns := giveMeGarage(garageName, garageClaimNS)

	ct := metav1.Now().Add(-20 * time.Minute)
	garage.CreationTimestamp = metav1.Time{Time: ct}

	r, manager, c := setupVSHNGarageTest(t,
		garage, ns,
		newTestBucketCreds(garageClaimNS, garageName),
		newTestAdminToken(garageInstanceNS),
	)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      garageName,
			Namespace: garageClaimNS,
		},
	}

	pi := probes.ProbeInfo{
		Service: "VSHNGarage",
		Name:    garageName,
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.True(t, manager.probers[getFakeKey(pi)])

	garage.Spec.Parameters.Instances = 0
	require.NoError(t, c.Update(context.TODO(), garage))
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, manager.probers[getFakeKey(pi)])
}

func giveMeGarage(name, claimNamespace string) (*vshnv1.XVSHNGarage, *corev1.Namespace) {
	garage := &vshnv1.XVSHNGarage{
		TypeMeta: metav1.TypeMeta{
			Kind:       "XVSHNGarage",
			APIVersion: "vshn.appcat.vshn.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         claimNamespace,
			CreationTimestamp: metav1.Now(),
			Labels: map[string]string{
				slireconciler.ClaimNamespaceLabel: claimNamespace,
				slireconciler.ClaimNameLabel:      name,
			},
		},
		Spec: vshnv1.XVSHNGarageSpec{
			Parameters: vshnv1.VSHNGarageParameters{
				Instances: 3,
			},
			CompositionRef: cpv1.CompositionReference{Name: "vshngarage.vshn.appcat.vshn.io"},
			ResourceSpec: xpv1.ResourceSpec{
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name:      name,
					Namespace: "vshn-garage-" + name,
				},
			},
		},
		Status: vshnv1.XVSHNGarageStatus{
			VSHNGarageStatus: vshnv1.VSHNGarageStatus{
				InstanceNamespace: "vshn-garage-" + name,
			},
		},
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: garage.GetInstanceNamespace(),
		},
	}

	return garage, ns
}

func setupVSHNGarageTest(t *testing.T, objs ...client.Object) (VSHNGarageReconciler, *fakeProbeManager, client.Client) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, vshnv1.AddToScheme(scheme))
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		Build()

	manager := newFakeProbeManager()
	r := VSHNGarageReconciler{
		Client:             c,
		Scheme:             scheme,
		ProbeManager:       manager,
		StartupGracePeriod: 5 * time.Minute,
		GarageDialer:       fakeGarageDialer,
		ScClient:           c,
	}

	return r, manager, c
}

func fakeGarageDialer(service, name, claimNamespace, instanceNamespace, organization, sla, compositionName, endpointURL, adminToken, bucketName string, ha bool, opts minio.Options) (*probes.VSHNGarage, error) {
	return &probes.VSHNGarage{
		Service:           service,
		Name:              name,
		ClaimNamespace:    claimNamespace,
		InstanceNamespace: instanceNamespace,
		Organization:      organization,
		HighAvailable:     ha,
		ServiceLevel:      sla,
	}, nil
}

func newTestBucketCreds(namespace, claimName string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      sliGarageBucketCRName(claimName),
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"endpoint":          []byte("http://s3.garage.svc:3900"),
			"access-key-id":     []byte("GK1234567890abcdef"),
			"secret-access-key": []byte("supersecret"),
		},
	}
}

func newTestAdminToken(namespace string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      adminTokenSecret,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"token": []byte("admin-bearer-token"),
		},
	}
}
