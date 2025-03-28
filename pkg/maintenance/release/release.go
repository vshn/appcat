package release

import (
	"context"
	"errors"
	"fmt"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/crossplane/function-sdk-go/resource/composite"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

// Interface for both Claim and Composite objects
type compositionObject interface {
	SetCompositionUpdatePolicy(*xpv1.UpdatePolicy)
	SetCompositionRevisionSelector(*metav1.LabelSelector)
}

// VersionHandler is an interface for handling AppCat versions
type VersionHandler interface {
	ReleaseLatest(ctx context.Context) error
}

// DefaultVersionHandler handles AppCat version change for a claim using composition revisions.
type DefaultVersionHandler struct {
	client         client.Client
	log            logr.Logger
	claimName      string
	compositeName  string
	claimNamespace string
	ownerGroup     string
	ownerKind      string
	ownerVersion   string
	serviceId      string
}

func NewDefaultVersionHandler(k8sClient client.Client, l logr.Logger, name, composite, namespace, group, kind, version, service string) VersionHandler {
	return &DefaultVersionHandler{
		client:         k8sClient,
		log:            l,
		claimName:      name,
		compositeName:  composite,
		claimNamespace: namespace,
		ownerGroup:     group,
		ownerKind:      kind,
		ownerVersion:   version,
		serviceId:      service,
	}
}

// ReleaseLatest function releases the latest AppCat version for a given claim via latest composition revision
func (vh *DefaultVersionHandler) ReleaseLatest(ctx context.Context) error {
	vh.log.Info("Releasing latest version of AppCat")

	// Get latest revision
	revision, err := vh.getLatestRevisionLabel(ctx)
	if err != nil || revision == "" {
		return err
	}

	// Try to update claim first
	if err := vh.updateClaim(ctx, revision); err != nil {
		if apierrors.IsNotFound(err) {
			// If claim is not found, fallback to updating composite
			return vh.updateComposite(ctx, revision)
		}
		return fmt.Errorf("failed to update claim %s/%s: %w", vh.claimNamespace, vh.claimName, err)
	}

	vh.log.Info("Claim updated successfully", "revision", revision)
	return nil
}

// Helper function to fetch the latest revision label
func (vh *DefaultVersionHandler) getLatestRevisionLabel(ctx context.Context) (string, error) {
	cr, err := vh.getLatestRevision(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest revision: %w", err)
	}

	revision, exists := cr.GetLabels()["metadata.appcat.vshn.io/revision"]
	if !exists || revision == "" {
		vh.log.Info("Label metadata.appcat.vshn.io/revision is missing from composition revision")
		return "", nil
		//TODO return this error when CI/CD is enabled
		//return errors.New("missing metadata.appcat.vshn.io/revision label in composition revision")
	}

	return revision, nil
}

func (vh *DefaultVersionHandler) getLatestRevision(ctx context.Context) (*v1.CompositionRevision, error) {
	vh.log.Info("Filtering composition revisions by service id")
	crl := &v1.CompositionRevisionList{}
	if err := vh.client.List(ctx, crl, client.MatchingLabelsSelector{
		Selector: labels.SelectorFromSet(labels.Set{"metadata.appcat.vshn.io/serviceID": vh.serviceId}),
	}); err != nil {
		return nil, fmt.Errorf("failed to list composition revisions: %w", err)
	}

	if len(crl.Items) == 0 {
		return nil, errors.New("no composition revisions found")
	}
	vh.log.V(1).Info("Found", "composition revisions", crl.Items)
	latestRevision := crl.Items[0]
	for _, item := range crl.Items[1:] {
		if item.Spec.Revision > latestRevision.Spec.Revision {
			latestRevision = item
		}
	}
	vh.log.Info("Found latest resource", "composition revision", latestRevision.GetName())

	return &latestRevision, nil
}

func (vh *DefaultVersionHandler) updateClaim(ctx context.Context, revision string) error {
	c := claim.New()
	c.SetGroupVersionKind(vh.getClaimGVK())

	err := vh.client.Get(ctx, types.NamespacedName{
		Name:      vh.claimName,
		Namespace: vh.claimNamespace,
	}, c)

	if err != nil {
		return err
	}

	vh.applyUpdatePolicy(c, revision)

	if err = vh.client.Update(ctx, c); err != nil {
		return fmt.Errorf("failed to update claim: %w", err)
	}

	return nil
}

func (vh *DefaultVersionHandler) updateComposite(ctx context.Context, revision string) error {
	comp := composite.New()
	comp.SetGroupVersionKind(vh.getCompositeGVK())

	if err := vh.client.Get(ctx, client.ObjectKey{Name: vh.compositeName}, comp); err != nil {
		return fmt.Errorf("failed to get composite %s of type %s: %w", vh.compositeName, vh.ownerKind, err)
	}

	vh.applyUpdatePolicy(comp, revision)

	if err := vh.client.Update(ctx, comp); err != nil {
		return fmt.Errorf("failed to update composite %s: %w", vh.compositeName, err)
	}

	vh.log.Info("Composite updated successfully", "revision", revision)
	return nil
}

// Helper function to apply update policies
func (vh *DefaultVersionHandler) applyUpdatePolicy(obj compositionObject, revision string) {
	obj.SetCompositionUpdatePolicy(UpdatePolicyPtr(xpv1.UpdateAutomatic))
	obj.SetCompositionRevisionSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{"metadata.appcat.vshn.io/revision": revision},
	})
}

// Helper function to get the GVK for claims
func (vh *DefaultVersionHandler) getClaimGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   vh.ownerGroup,
		Version: vh.ownerVersion,
		Kind:    removeLeadingX(vh.ownerKind),
	}
}

// Helper function to get the GVK for composites
func (vh *DefaultVersionHandler) getCompositeGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   vh.ownerGroup,
		Version: vh.ownerVersion,
		Kind:    vh.ownerKind,
	}
}

func UpdatePolicyPtr(s xpv1.UpdatePolicy) *xpv1.UpdatePolicy {
	return &s
}

func removeLeadingX(s string) string {
	return strings.TrimLeft(s, "Xx")
}
