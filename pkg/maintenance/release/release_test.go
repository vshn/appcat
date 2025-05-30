package release_test

import (
	"context"
	"testing"

	"github.com/crossplane/function-sdk-go/resource/composite"
	"github.com/vshn/appcat/v4/pkg/maintenance/release"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func setupFakeClient(objects ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = v1.AddToScheme(scheme) // Register Crossplane APIs

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()
}

func TestGetLatestRevision_NoRevisions(t *testing.T) {
	// When: No composition revisions exist
	fakeClient := setupFakeClient()
	logger := testr.New(t)
	opts := release.ReleaserOpts{
		ClaimName:      "test-claim",
		Composite:      "test-composite",
		ClaimNamespace: "default",
		Group:          "test.group",
		Kind:           "TestKind",
		Version:        "v1",
		ServiceID:      "service-123",
	}
	vh := release.NewDefaultVersionHandler(fakeClient, logger, opts)

	// Do
	err := vh.ReleaseLatest(context.Background(), true)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no composition revisions found")
}

func TestLatestVersion_UpdateClaim(t *testing.T) {
	// When: Set up a claim and composition revisions
	claimObj := claim.New(claim.WithGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "TestKind",
	}))
	claimObj.SetName("test-claim")
	claimObj.SetNamespace("default")
	claimObj.SetCompositionUpdatePolicy(release.UpdatePolicyPtr(xpv1.UpdateManual))

	cr1 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "revision-42",
			Namespace: "default",
			Labels: map[string]string{
				"metadata.appcat.vshn.io/serviceID": "service-123",
				"metadata.appcat.vshn.io/revision":  "42",
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 42},
	}

	cr2 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "revision-43",
			Namespace: "default",
			Labels: map[string]string{
				"metadata.appcat.vshn.io/serviceID": "service-123",
				"metadata.appcat.vshn.io/revision":  "43",
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 43},
	}

	cr3 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "revision-45",
			Namespace: "default",
			Labels: map[string]string{
				"metadata.appcat.vshn.io/serviceID": "another-service",
				"metadata.appcat.vshn.io/revision":  "45",
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 45},
	}

	fakeClient := setupFakeClient(claimObj, cr1, cr2, cr3)
	logger := testr.New(t)
	opts := release.ReleaserOpts{
		ClaimName:      "test-claim",
		Composite:      "composite",
		ClaimNamespace: "default",
		Group:          "test.group",
		Kind:           "XTestKind",
		Version:        "v1",
		ServiceID:      "service-123",
	}
	vh := release.NewDefaultVersionHandler(fakeClient, logger, opts)

	// Do
	err := vh.ReleaseLatest(context.Background(), true)

	// Then
	require.NoError(t, err)

	// Verify claim was updated
	updatedClaim := claim.New(claim.WithGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "TestKind",
	}))
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "test-claim", Namespace: "default"}, updatedClaim)
	require.NoError(t, err)

	// Check if update policy and label selector were set
	assert.Equal(t, xpv1.UpdateAutomatic, *updatedClaim.GetCompositionUpdatePolicy())
	assert.Equal(t, "43", updatedClaim.GetCompositionRevisionSelector().MatchLabels["metadata.appcat.vshn.io/revision"])
}

func TestLatestVersion_UpdateComposite(t *testing.T) {
	// When: Set up a composite and composition revisions
	comp := composite.New()
	comp.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "XTestKind",
	})
	comp.SetName("composite")
	comp.SetCompositionUpdatePolicy(release.UpdatePolicyPtr(xpv1.UpdateManual))

	cr1 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "revision-42",
			Namespace: "default",
			Labels: map[string]string{
				"metadata.appcat.vshn.io/serviceID": "service-123",
				"metadata.appcat.vshn.io/revision":  "42",
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 42},
	}

	cr2 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "revision-43",
			Namespace: "default",
			Labels: map[string]string{
				"metadata.appcat.vshn.io/serviceID": "service-123",
				"metadata.appcat.vshn.io/revision":  "44",
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 43},
	}

	cr3 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "revision-45",
			Namespace: "default",
			Labels: map[string]string{
				"metadata.appcat.vshn.io/serviceID": "another-service",
				"metadata.appcat.vshn.io/revision":  "45",
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 45},
	}

	fakeClient := setupFakeClient(comp, cr1, cr2, cr3)
	logger := testr.New(t)
	opts := release.ReleaserOpts{
		Composite: "composite",
		Group:     "test.group",
		Kind:      "XTestKind",
		Version:   "v1",
		ServiceID: "service-123",
	}
	vh := release.NewDefaultVersionHandler(fakeClient, logger, opts)

	// Do
	err := vh.ReleaseLatest(context.Background(), true)

	// Then
	require.NoError(t, err)

	// Verify composite was updated
	updatedComposite := composite.New()
	updatedComposite.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "XTestKind",
	})
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "composite"}, updatedComposite)
	require.NoError(t, err)

	// Check if update policy and label selector were set
	assert.Equal(t, xpv1.UpdateAutomatic, *updatedComposite.GetCompositionUpdatePolicy())
	assert.Equal(t, "44", updatedComposite.GetCompositionRevisionSelector().MatchLabels["metadata.appcat.vshn.io/revision"])
}

func TestLatestVersion_MissingRevisionLabel(t *testing.T) {
	// When: Create a claim and a revision without the label
	claimObj := claim.New(claim.WithGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "TestKind",
	}))
	claimObj.SetName("test-claim")
	claimObj.SetNamespace("default")

	cr := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "revision-latest",
			Namespace: "default",
			Labels: map[string]string{
				"metadata.appcat.vshn.io/serviceID": "service-123",
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 42},
	}

	fakeClient := setupFakeClient(claimObj, cr)
	logger := testr.New(t)
	opts := release.ReleaserOpts{
		ClaimName:      "test-claim",
		Composite:      "composite",
		ClaimNamespace: "default",
		Group:          "test.group",
		Kind:           "TestKind",
		Version:        "v1",
		ServiceID:      "service-123",
	}
	vh := release.NewDefaultVersionHandler(fakeClient, logger, opts)

	// Do
	err := vh.ReleaseLatest(context.Background(), true)

	// Then: No error, but log message should indicate missing label
	require.Error(t, err)
}

func TestDisableReleaseManagment(t *testing.T) {
	// When: Set up a composite and composition revisions
	comp := composite.New()
	comp.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "XTestKind",
	})
	comp.SetName("composite")
	comp.SetCompositionUpdatePolicy(release.UpdatePolicyPtr(xpv1.UpdateManual))

	cr1 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "revision-42",
			Namespace: "default",
			Labels: map[string]string{
				"metadata.appcat.vshn.io/serviceID": "service-123",
				"metadata.appcat.vshn.io/revision":  "42",
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 42},
	}

	cr2 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "revision-43",
			Namespace: "default",
			Labels: map[string]string{
				"metadata.appcat.vshn.io/serviceID": "service-123",
				"metadata.appcat.vshn.io/revision":  "44",
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 43},
	}

	cr3 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "revision-45",
			Namespace: "default",
			Labels: map[string]string{
				"metadata.appcat.vshn.io/serviceID": "another-service",
				"metadata.appcat.vshn.io/revision":  "45",
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 45},
	}

	fakeClient := setupFakeClient(comp, cr1, cr2, cr3)
	logger := testr.New(t)
	opts := release.ReleaserOpts{
		Composite: "composite",
		Group:     "test.group",
		Kind:      "XTestKind",
		Version:   "v1",
		ServiceID: "service-123",
	}
	vh := release.NewDefaultVersionHandler(fakeClient, logger, opts)

	// Do
	err := vh.ReleaseLatest(context.Background(), false)

	// Then
	require.NoError(t, err)

	// Verify composite was updated
	updatedComposite := composite.New()
	updatedComposite.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "XTestKind",
	})
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "composite"}, updatedComposite)
	require.NoError(t, err)

	// Check if update policy and label selector were set
	assert.Equal(t, xpv1.UpdateAutomatic, *updatedComposite.GetCompositionUpdatePolicy())
	assert.Equal(t, "", updatedComposite.GetCompositionRevisionSelector().MatchLabels["metadata.appcat.vshn.io/revision"])
}
