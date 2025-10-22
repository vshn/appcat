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

const testMinimumRevisionAge = 7 * 24 * time.Hour // 1 week

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
	err := vh.ReleaseLatest(context.Background(), true, fakeClient, testMinimumRevisionAge)

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
	err := vh.ReleaseLatest(context.Background(), true, fakeClient, testMinimumRevisionAge)

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
	err := vh.ReleaseLatest(context.Background(), true, fakeClient, testMinimumRevisionAge)

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
	err := vh.ReleaseLatest(context.Background(), true, fakeClient, testMinimumRevisionAge)

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
	err := vh.ReleaseLatest(context.Background(), true, fakeClient, testMinimumRevisionAge)

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
	err := vh.ReleaseLatest(context.Background(), true, fakeClient, testMinimumRevisionAge)

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
	err := vh.ReleaseLatest(context.Background(), false, fakeClient, testMinimumRevisionAge)

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
	err := vh.ReleaseLatest(context.Background(), true, fakeClient, testMinimumRevisionAge)

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
	err := vh.ReleaseLatest(context.Background(), true, fakeClient, testMinimumRevisionAge)

	// Then: Should fall back to newest revision when none meet grace period
	require.NoError(t, err)

	// Verify composite was updated to the newest available revision
	updatedComposite := composite.New()
	updatedComposite.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "XTestKind",
	})
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "composite"}, updatedComposite)
	require.NoError(t, err)

	// Check if update policy and label selector were set to revision 42
	assert.Equal(t, xpv1.UpdateAutomatic, *updatedComposite.GetCompositionUpdatePolicy())
	assert.Equal(t, "42", updatedComposite.GetCompositionRevisionSelector().MatchLabels["metadata.appcat.vshn.io/revision"])
}

func TestHotfixerBehavior_NoAgeFiltering(t *testing.T) {
	// When: Hotfixer mode with a brand new revision (less than 7 days old)
	comp := composite.New()
	comp.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "XTestKind",
	})
	comp.SetName("composite")
	comp.SetCompositionUpdatePolicy(release.UpdatePolicyPtr(xpv1.UpdateManual))

	now := time.Now()
	brandNewTime := metav1.NewTime(now.Add(-1 * time.Hour)) // 1 hour ago (too new for maintenance!)

	// Brand new revision - would be rejected by maintenance but accepted by hotfixer
	cr1 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "revision-50",
			Namespace:         "default",
			CreationTimestamp: brandNewTime,
			Labels: map[string]string{
				"metadata.appcat.vshn.io/serviceID": "service-123",
				"metadata.appcat.vshn.io/revision":  "50",
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 50},
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

	// Do: Pass 0 duration to bypass age filtering (hotfixer behavior)
	err := vh.ReleaseLatest(context.Background(), true, fakeClient, 0)

	// Then: Should succeed even though revision is brand new
	require.NoError(t, err)

	// Verify composite was updated to the new revision immediately
	updatedComposite := composite.New()
	updatedComposite.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "XTestKind",
	})
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "composite"}, updatedComposite)
	require.NoError(t, err)

	// Check if update policy and label selector were set to the new revision
	assert.Equal(t, xpv1.UpdateAutomatic, *updatedComposite.GetCompositionUpdatePolicy())
	assert.Equal(t, "50", updatedComposite.GetCompositionRevisionSelector().MatchLabels["metadata.appcat.vshn.io/revision"])
}

func TestHotfixerVsMaintenance_AgeBehavior(t *testing.T) {
	// When: Multiple revisions with different ages
	comp := composite.New()
	comp.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "XTestKind",
	})
	comp.SetName("composite")
	comp.SetCompositionUpdatePolicy(release.UpdatePolicyPtr(xpv1.UpdateManual))

	now := time.Now()
	oldTime := metav1.NewTime(now.Add(-10 * 24 * time.Hour)) // 10 days ago
	newTime := metav1.NewTime(now.Add(-2 * 24 * time.Hour))  // 2 days ago (too new for maintenance)

	// Old revision (eligible for both hotfixer and maintenance)
	cr1 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "revision-40",
			Namespace:         "default",
			CreationTimestamp: oldTime,
			Labels: map[string]string{
				"metadata.appcat.vshn.io/serviceID": "service-123",
				"metadata.appcat.vshn.io/revision":  "40",
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 40},
	}

	// New revision with critical fix (only eligible for hotfixer)
	cr2 := &v1.CompositionRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "revision-45",
			Namespace:         "default",
			CreationTimestamp: newTime,
			Labels: map[string]string{
				"metadata.appcat.vshn.io/serviceID": "service-123",
				"metadata.appcat.vshn.io/revision":  "45",
			},
		},
		Spec: v1.CompositionRevisionSpec{Revision: 45},
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

	// Test 1: Maintenance mode (7-day grace period) should select revision 40
	vh1 := release.NewDefaultVersionHandler(logger, opts)
	err := vh1.ReleaseLatest(context.Background(), true, fakeClient, testMinimumRevisionAge)
	require.NoError(t, err)

	comp1 := composite.New()
	comp1.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "XTestKind",
	})
	err = fakeClient.Get(context.Background(), client.ObjectKey{Name: "composite"}, comp1)
	require.NoError(t, err)
	assert.Equal(t, "40", comp1.GetCompositionRevisionSelector().MatchLabels["metadata.appcat.vshn.io/revision"],
		"Maintenance mode should select the old revision (40)")

	// Test 2: Create a fresh composite and client for hotfixer test
	comp2 := composite.New()
	comp2.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "XTestKind",
	})
	comp2.SetName("composite-hotfix")
	comp2.SetCompositionUpdatePolicy(release.UpdatePolicyPtr(xpv1.UpdateManual))

	fakeClient2 := setupFakeClient(comp2, cr1, cr2)
	opts2 := release.ReleaserOpts{
		Composite: "composite-hotfix",
		Group:     "test.group",
		Kind:      "XTestKind",
		Version:   "v1",
		ServiceID: "service-123",
	}

	// Hotfixer mode (no age check) should select the newest revision 45
	vh2 := release.NewDefaultVersionHandler(logger, opts2)
	err = vh2.ReleaseLatest(context.Background(), true, fakeClient2, 0)
	require.NoError(t, err)

	updatedComp2 := composite.New()
	updatedComp2.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "test.group",
		Version: "v1",
		Kind:    "XTestKind",
	})
	err = fakeClient2.Get(context.Background(), client.ObjectKey{Name: "composite-hotfix"}, updatedComp2)
	require.NoError(t, err)
	assert.Equal(t, "45", updatedComp2.GetCompositionRevisionSelector().MatchLabels["metadata.appcat.vshn.io/revision"],
		"Hotfixer mode should select the newest revision (45) immediately")
}
