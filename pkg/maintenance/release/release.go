package release

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
)

const (
	ServiceIDLabel = "metadata.appcat.vshn.io/serviceID"
	RevisionLabel  = "metadata.appcat.vshn.io/revision"
)

// Interface for both Claim and Composite objects
type compositionObject interface {
	SetCompositionUpdatePolicy(*xpv1.UpdatePolicy)
	SetCompositionRevisionSelector(*metav1.LabelSelector)
}

// VersionHandler is an interface for handling AppCat versions
type VersionHandler interface {
	ReleaseLatest(ctx context.Context, enabled bool, kubeClient client.Client) error
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

// ReleaserOpts holds all necessary information for a
// release handler to switch the revisions.
type ReleaserOpts struct {
	ClaimName, Composite, ClaimNamespace, Group, Kind, Version, ServiceID string
}

func NewDefaultVersionHandler(l logr.Logger, opts ReleaserOpts) VersionHandler {
	return &DefaultVersionHandler{
		log:            l,
		claimName:      opts.ClaimName,
		compositeName:  opts.Composite,
		claimNamespace: opts.ClaimNamespace,
		ownerGroup:     opts.Group,
		ownerKind:      opts.Kind,
		ownerVersion:   opts.Version,
		serviceId:      opts.ServiceID,
	}
}

// ReleaseLatest function releases the latest AppCat version for a given claim via latest composition revision
func (vh *DefaultVersionHandler) ReleaseLatest(ctx context.Context, enabled bool, kubeClient client.Client) error {
	vh.log.Info("Releasing latest version of AppCat")

	vh.client = kubeClient

	revision := ""
	if enabled {
		// Get latest revision
		rev, err := vh.getLatestRevisionLabel(ctx)
		if err != nil || rev == "" {
			return err
		}
		revision = rev
	} else {
		vh.log.Info("Disabling release management and resetting to default policy")
	}

	err := vh.updateClaim(ctx, revision, enabled)
	if err != nil {

		if apierrors.IsNotFound(err) {
			err := vh.updateComposite(ctx, revision, enabled)
			if err != nil {
				return err
			}
			vh.log.Info("Composite updated successfully", ServiceIDLabel, revision)
			return nil
		}

		return fmt.Errorf("failed to update claim %s/%s: %w", vh.claimNamespace, vh.claimName, err)
	}

	vh.log.Info("Claim updated successfully", RevisionLabel, revision)
	return nil
}

// Helper function to fetch the latest revision label
func (vh *DefaultVersionHandler) getLatestRevisionLabel(ctx context.Context) (string, error) {
	cr, err := vh.getLatestRevision(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest revision: %w", err)
	}

	revision, exists := cr.GetLabels()[RevisionLabel]
	if !exists || revision == "" {
		return "", errors.New("missing " + RevisionLabel + " label in composition revision")
	}

	return revision, nil
}

func (vh *DefaultVersionHandler) getLatestRevision(ctx context.Context) (*v1.CompositionRevision, error) {
	vh.log.Info("Filtering composition revisions by service id", ServiceIDLabel, vh.serviceId)
	crl := &v1.CompositionRevisionList{}
	if err := vh.client.List(ctx, crl, client.MatchingLabelsSelector{
		Selector: labels.SelectorFromSet(labels.Set{ServiceIDLabel: vh.serviceId}),
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
	vh.log.Info("Found latest resource", "composition revision", latestRevision.GetName(), RevisionLabel, latestRevision.GetLabels()[RevisionLabel])

	return &latestRevision, nil
}

func (vh *DefaultVersionHandler) updateClaim(ctx context.Context, revision string, enabled bool) error {
	c := claim.New()
	c.SetGroupVersionKind(vh.getClaimGVK())

	err := vh.client.Get(ctx, types.NamespacedName{
		Name:      vh.claimName,
		Namespace: vh.claimNamespace,
	}, c)

	if err != nil {
		return err
	}

	if enabled {
		vh.applyUpdatePolicy(c, revision)
	} else {
		vh.setAutoPolicy(c)
	}

	if err = vh.client.Update(ctx, c); err != nil {
		if apierrors.IsNotFound(err) {
			return err
		}
		return fmt.Errorf("failed to update claim: %w", err)
	}

	return nil
}

func (vh *DefaultVersionHandler) updateComposite(ctx context.Context, revision string, enabled bool) error {
	comp := composite.New()
	comp.SetGroupVersionKind(vh.getCompositeGVK())

	if err := vh.client.Get(ctx, client.ObjectKey{Name: vh.compositeName}, comp); err != nil {
		return fmt.Errorf("failed to get composite %s of type %s: %w", vh.compositeName, vh.ownerKind, err)
	}

	if enabled {
		vh.applyUpdatePolicy(comp, revision)
	} else {
		vh.setAutoPolicy(comp)
	}

	if err := vh.client.Update(ctx, comp); err != nil {
		return fmt.Errorf("failed to update composite %s: %w", vh.compositeName, err)
	}

	return nil
}

// Helper function to apply update policies
func (vh *DefaultVersionHandler) applyUpdatePolicy(obj compositionObject, revision string) {
	obj.SetCompositionUpdatePolicy(UpdatePolicyPtr(xpv1.UpdateAutomatic))
	obj.SetCompositionRevisionSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{RevisionLabel: revision},
	})
}

// Helper function to set the policy to automatic and remove the label selector
func (vh *DefaultVersionHandler) setAutoPolicy(obj compositionObject) {
	obj.SetCompositionUpdatePolicy(UpdatePolicyPtr(xpv1.UpdateAutomatic))
	obj.SetCompositionRevisionSelector(nil)
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
