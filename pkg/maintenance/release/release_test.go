package release_test

import (
	"context"
	"testing"
	"time"

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
	vh := release.NewDefaultVersionHandler(logger, opts)

	// Do
	err := vh.ReleaseLatest(context.Background(), true, fakeClient)

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

	oldTime := metav1.NewTime(time.Now().Add(-8 * 24 * time.Hour)) // 8 days ago

	cr1 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "revision-42",
			Namespace:         "default",
			CreationTimestamp: oldTime,
			Labels: map[string]string{
				"metadata.appcat.vshn.io/serviceID": "service-123",
				"metadata.appcat.vshn.io/revision":  "42",
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 42},
	}

	cr2 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "revision-43",
			Namespace:         "default",
			CreationTimestamp: oldTime,
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
	vh := release.NewDefaultVersionHandler(logger, opts)

	// Do
	err := vh.ReleaseLatest(context.Background(), true, fakeClient)

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
	vh := release.NewDefaultVersionHandler(logger, opts)

	// Do
	err := vh.ReleaseLatest(context.Background(), true, fakeClient)

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
	vh := release.NewDefaultVersionHandler(logger, opts)

	// Do
	err := vh.ReleaseLatest(context.Background(), true, fakeClient)

	// Then: No error, but log message should indicate missing label
	require.Error(t, err)
}

func TestAutoUpdateLabel_UpdateClaim(t *testing.T) {
	// When: Set up a claim with autoUpdate label and composition revisions
	claimObj := claim.New(claim.WithGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "TestKind",
	}))
	claimObj.SetName("test-claim")
	claimObj.SetNamespace("default")
	claimObj.SetCompositionUpdatePolicy(release.UpdatePolicyPtr(xpv1.UpdateManual))
	claimObj.SetLabels(map[string]string{
		"metadata.appcat.vshn.io/autoUpdate": "true",
	})

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

	fakeClient := setupFakeClient(claimObj, cr1, cr2)
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
	vh := release.NewDefaultVersionHandler(logger, opts)

	// Do
	err := vh.ReleaseLatest(context.Background(), true, fakeClient)

	// Then
	require.NoError(t, err)

	// Verify claim was updated with automatic policy and NO revision pinning
	updatedClaim := claim.New(claim.WithGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "TestKind",
	}))
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "test-claim", Namespace: "default"}, updatedClaim)
	require.NoError(t, err)

	// Check if update policy is automatic and NO revision pinning (gets latest automatically)
	assert.Equal(t, xpv1.UpdateAutomatic, *updatedClaim.GetCompositionUpdatePolicy())
	// When autoUpdate is true, the selector should be empty (no revision pinning)
	selector := updatedClaim.GetCompositionRevisionSelector()
	if selector != nil && selector.MatchLabels != nil {
		assert.Equal(t, "", selector.MatchLabels["metadata.appcat.vshn.io/revision"])
	}
}

func TestAutoUpdateLabel_UpdateComposite(t *testing.T) {
	// When: Set up a composite with autoUpdate label and composition revisions
	comp := composite.New()
	comp.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "XTestKind",
	})
	comp.SetName("composite")
	comp.SetCompositionUpdatePolicy(release.UpdatePolicyPtr(xpv1.UpdateManual))
	comp.SetLabels(map[string]string{
		"metadata.appcat.vshn.io/autoUpdate": "true",
	})

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

	fakeClient := setupFakeClient(comp, cr1, cr2)
	logger := testr.New(t)
	opts := release.ReleaserOpts{
		Composite: "composite",
		Group:     "test.group",
		Kind:      "XTestKind",
		Version:   "v1",
		ServiceID: "service-123",
	}
	vh := release.NewDefaultVersionHandler(logger, opts)

	// Do
	err := vh.ReleaseLatest(context.Background(), true, fakeClient)

	// Then
	require.NoError(t, err)

	// Verify composite was updated with automatic policy and NO revision pinning
	updatedComposite := composite.New()
	updatedComposite.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "XTestKind",
	})
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "composite"}, updatedComposite)
	require.NoError(t, err)

	// Check if update policy is automatic and NO revision pinning (gets latest automatically)
	assert.Equal(t, xpv1.UpdateAutomatic, *updatedComposite.GetCompositionUpdatePolicy())
	// When autoUpdate is true, the selector should be empty (no revision pinning)
	selector := updatedComposite.GetCompositionRevisionSelector()
	if selector != nil && selector.MatchLabels != nil {
		assert.Equal(t, "", selector.MatchLabels["metadata.appcat.vshn.io/revision"])
	}
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
	vh := release.NewDefaultVersionHandler(logger, opts)

	// Do
	err := vh.ReleaseLatest(context.Background(), false, fakeClient)

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

func TestRevisionAgeFiltering_SkipsNewRevisions(t *testing.T) {
	// When: Set up a composite with multiple revisions, including some that are too new
	comp := composite.New()
	comp.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "XTestKind",
	})
	comp.SetName("composite")
	comp.SetCompositionUpdatePolicy(release.UpdatePolicyPtr(xpv1.UpdateManual))

	now := time.Now()
	oldTime := metav1.NewTime(now.Add(-8 * 24 * time.Hour))       // 8 days ago
	moderateTime := metav1.NewTime(now.Add(-10 * 24 * time.Hour)) // 10 days ago
	recentTime := metav1.NewTime(now.Add(-3 * 24 * time.Hour))    // 3 days ago (too new!)

	// Old revision (eligible)
	cr1 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "revision-42",
			Namespace:         "default",
			CreationTimestamp: oldTime,
			Labels: map[string]string{
				"metadata.appcat.vshn.io/serviceID": "service-123",
				"metadata.appcat.vshn.io/revision":  "42",
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 42},
	}

	// Moderate age revision (eligible, and the oldest)
	cr2 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "revision-40",
			Namespace:         "default",
			CreationTimestamp: moderateTime,
			Labels: map[string]string{
				"metadata.appcat.vshn.io/serviceID": "service-123",
				"metadata.appcat.vshn.io/revision":  "40",
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 40},
	}

	// Recent revision (NOT eligible - too new, even though it has the highest revision number)
	cr3 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "revision-45",
			Namespace:         "default",
			CreationTimestamp: recentTime,
			Labels: map[string]string{
				"metadata.appcat.vshn.io/serviceID": "service-123",
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
	vh := release.NewDefaultVersionHandler(logger, opts)

	// Do
	err := vh.ReleaseLatest(context.Background(), true, fakeClient)

	// Then
	require.NoError(t, err)

	// Verify composite was updated to revision 42 (highest eligible, not 45 which is too new)
	updatedComposite := composite.New()
	updatedComposite.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "XTestKind",
	})
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "composite"}, updatedComposite)
	require.NoError(t, err)

	// Check if update policy and label selector were set to revision 42 (not 45)
	assert.Equal(t, xpv1.UpdateAutomatic, *updatedComposite.GetCompositionUpdatePolicy())
	assert.Equal(t, "42", updatedComposite.GetCompositionRevisionSelector().MatchLabels["metadata.appcat.vshn.io/revision"])
}

func TestRevisionAgeFiltering_AllRevisionsTooNew(t *testing.T) {
	// When: All revisions are too new
	comp := composite.New()
	comp.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "XTestKind",
	})
	comp.SetName("composite")
	comp.SetCompositionUpdatePolicy(release.UpdatePolicyPtr(xpv1.UpdateManual))

	now := time.Now()
	recentTime := metav1.NewTime(now.Add(-3 * 24 * time.Hour)) // 3 days ago (too new!)

	cr1 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "revision-42",
			Namespace:         "default",
			CreationTimestamp: recentTime,
			Labels: map[string]string{
				"metadata.appcat.vshn.io/serviceID": "service-123",
				"metadata.appcat.vshn.io/revision":  "42",
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 42},
	}

	fakeClient := setupFakeClient(comp, cr1)
	logger := testr.New(t)
	opts := release.ReleaserOpts{
		Composite: "composite",
		Group:     "test.group",
		Kind:      "XTestKind",
		Version:   "v1",
		ServiceID: "service-123",
	}
	vh := release.NewDefaultVersionHandler(logger, opts)

	// Do
	err := vh.ReleaseLatest(context.Background(), true, fakeClient)

	// Then: Should get an error since no revisions are old enough
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no composition revisions found that are at least")
}
