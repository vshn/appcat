package vshnminiocontroller

import (
	"context"
	"fmt"
	"testing"
	"time"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/sliexporter/probes"
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
	bucketName        = "vshn-test-bucket-for-sli"
	claimNamespace    = "vshnminio"
	instanceNamespace = "vshn-minio-vshnminio"

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

func TestReconciler(t *testing.T) {
	minio, ns := giveMeMinio(bucketName, claimNamespace)

	ct := metav1.Now().Add(-20 * time.Minute)
	minio.CreationTimestamp = metav1.Time{Time: ct}

	r, manager, client := setupVSHNMinioTest(t,
		minio, ns,
		newTestVSHNMinioCred(bucketName, claimNamespace),
	)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      bucketName,
			Namespace: claimNamespace,
		},
	}

	probeInfo := probes.ProbeInfo{
		Service:           "VSHNMinio",
		Name:              bucketName,
		ClaimNamespace:    claimNamespace,
		InstanceNamespace: instanceNamespace,
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)

	assert.True(t, manager.probers[getFakeKey(probeInfo)])

	require.NoError(t, client.Delete(context.TODO(), minio))
	_, err = r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, manager.probers[getFakeKey(probeInfo)])
}

func TestVSHNMinio_Startup_NoCreds_Dont_Probe(t *testing.T) {
	minio, ns := giveMeMinio(bucketName, claimNamespace)

	r, manager, _ := setupVSHNMinioTest(t,
		minio, ns,
	)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      bucketName,
			Namespace: claimNamespace,
		},
	}

	probeInfo := probes.ProbeInfo{
		Service:           "VSHNMinio",
		Name:              bucketName,
		ClaimNamespace:    claimNamespace,
		InstanceNamespace: instanceNamespace,
	}

	res, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.Greater(t, res.RequeueAfter.Microseconds(), int64(0))

	assert.False(t, manager.probers[getFakeKey(probeInfo)])
}

func TestVSHNMinio_NoRef_Dont_Probe(t *testing.T) {
	db, ns := giveMeMinio("bar", "foo")
	db.Spec.WriteConnectionSecretToReference.Name = ""
	r, manager, _ := setupVSHNMinioTest(t,
		db, ns,
	)

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      bucketName,
			Namespace: claimNamespace,
		},
	}
	pi := probes.ProbeInfo{
		Service:           "VSHNPostgreSQL",
		Name:              "foo",
		ClaimNamespace:    "bar",
		InstanceNamespace: "bar",
	}

	_, err := r.Reconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.False(t, manager.probers[getFakeKey(pi)])
}

func giveMeMinio(bucketName string, namespace string) (*vshnv1.XVSHNMinio, *corev1.Namespace) {
	claim := &vshnv1.XVSHNMinio{
		TypeMeta: metav1.TypeMeta{
			Kind:       "XVSHNMinio",
			APIVersion: "vshn.appcat.vshn.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              bucketName,
			Namespace:         namespace,
			CreationTimestamp: metav1.Now(),
		},
		Spec: vshnv1.XVSHNMinioSpec{
			Parameters: vshnv1.VSHNMinioParameters{
				Instances: 4,
			},
			ResourceSpec: xpv1.ResourceSpec{
				WriteConnectionSecretToReference: &xpv1.SecretReference{
					Name:      bucketName,
					Namespace: "vshn-minio-" + namespace,
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

func setupVSHNMinioTest(t *testing.T, objs ...client.Object) (VSHNMinioReconciler, *fakeProbeManager, client.Client) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, vshnv1.AddToScheme(scheme))
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		Build()

	manager := newFakeProbeManager()
	r := VSHNMinioReconciler{
		Client:             client,
		Scheme:             scheme,
		ProbeManager:       manager,
		StartupGracePeriod: 5 * time.Minute,
		MinioDialer:        fakeMinioDialer,
		ScClient:           client,
	}

	return r, manager, client
}

func fakeMinioDialer(service, name, claimNamespace, instanceNamespace, organization, sla, endpointURL string, ha bool, opts minio.Options) (*probes.VSHNMinio, error) {
	p := &probes.VSHNMinio{
		Service:           service,
		Name:              name,
		ClaimNamespace:    claimNamespace,
		InstanceNamespace: instanceNamespace,
		Organization:      organization,
		HighAvailable:     ha,
		ServiceLevel:      sla,
	}
	return p, nil
}

func newTestVSHNMinioCred(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "vshn-minio-" + name,
		},
		StringData: map[string]string{
			"AWS_ACCESS_KEY_ID":     "vshn-test-bucket-for-sli",
			"AWS_REGION":            "rma",
			"AWS_SECRET_ACCESS_KEY": "zIEjZtouo6jREfKO2MQjflcyhZvUhyiQ4FOaiIatdzNOdjCYc4aAEAOSXBrZ1lsp",
			"BUCKET_NAME":           "vshn-test-bucket-for-sli",
			"ENDPOINT":              "minio-cluster-d2d67.vshn-minio-minio-cluster-d2d67.svc:9000",
			"ENDPOINT_URL":          "http://minio-cluster-d2d67.vshn-minio-minio-cluster-d2d67.svc:9000/",
		},
	}
}
