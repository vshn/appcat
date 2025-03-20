package release

import (
	"context"
	"errors"
	"fmt"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/crossplane-runtime/pkg/resource/unstructured/claim"
	v1 "github.com/crossplane/crossplane/apis/apiextensions/v1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// VersionHandler is an interface for handling AppCat versions
type VersionHandler interface {
	LatestVersion(ctx context.Context) error
}

// DefaultVersionHandler handles AppCat version change for a claim using composition revisions.
type DefaultVersionHandler struct {
	client         client.Client
	log            logr.Logger
	claimName      string
	claimNamespace string
	ownerGroup     string
	ownerKind      string
	ownerVersion   string
	serviceId      string
}

func NewDefaultVersionHandler(k8sClient client.Client, l logr.Logger, name, namespace, group, kind, version, service string) VersionHandler {
	return &DefaultVersionHandler{
		client:         k8sClient,
		log:            l,
		claimName:      name,
		claimNamespace: namespace,
		ownerGroup:     group,
		ownerKind:      kind,
		ownerVersion:   version,
		serviceId:      service,
	}
}

// LatestVersion function releases the latest AppCat version for a given claim via latest composition revision
func (vh *DefaultVersionHandler) LatestVersion(ctx context.Context) error {
	vh.log.Info("Releasing latest version of AppCat")
	cr, err := vh.getLatestRevision(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch latest revision: %w", err)
	}

	revision, exists := cr.GetLabels()["metadata.appcat.vshn.io/revision"]
	if !exists || revision == "" {
		vh.log.Info("Label metadata.appcat.vshn.io/revision is missing from composition revision")
		return nil
		//TODO return this error when CI/CD is enabled
		//return errors.New("missing metadata.appcat.vshn.io/revision label in composition revision")
	}

	c, err := vh.getClaim(ctx)
	if err != nil {
		return fmt.Errorf("failed to get claim: %w", err)
	}

	c.SetCompositionUpdatePolicy(stringPtr(xpv1.UpdateAutomatic))
	c.SetCompositionRevisionSelector(&metav1.LabelSelector{
		MatchLabels: map[string]string{"metadata.appcat.vshn.io/revision": revision},
	})
	err = vh.client.Update(ctx, c)
	if err != nil {
		return fmt.Errorf("failed to update claim: %w", err)
	}
	vh.log.Info("Claim updated successfully", "revision", revision)

	return nil
}

func (vh *DefaultVersionHandler) getClaim(ctx context.Context) (resource.CompositeClaim, error) {
	c := claim.New(claim.WithGroupVersionKind(schema.GroupVersionKind{
		Group:   vh.ownerGroup,
		Version: vh.ownerVersion,
		Kind:    vh.ownerKind,
	}))

	if err := vh.client.Get(ctx, types.NamespacedName{
		Namespace: vh.claimNamespace,
		Name:      vh.claimName,
	}, c); err != nil {
		return nil, fmt.Errorf("failed to get claim %s/%s of type %s: %w", vh.claimNamespace, vh.claimName, vh.ownerKind, err)
	}

	return c, nil
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

func stringPtr(s xpv1.UpdatePolicy) *xpv1.UpdatePolicy {
	return &s
}
