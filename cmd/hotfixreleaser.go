package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/maintenance/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var HotfixerCMD = newHotfixer()

type hotfixer struct {
}

func newHotfixer() cobra.Command {
	h := hotfixer{}

	return cobra.Command{
		Use:   "hotfixer",
		Short: "Hotfixer",
		Long:  "Run the Hotfixer",
		RunE:  h.runHotfixer,
	}
}

func (h *hotfixer) runHotfixer(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	log := logr.FromContextOrDiscard(ctx)

	dynClient, err := dynamic.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		return fmt.Errorf("failed to initialize kube client: %w", err)
	}

	kubeClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: pkg.SetupScheme(),
	})
	if err != nil {
		return err
	}

	xrds, err := dynClient.Resource(schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1",
		Resource: "compositeresourcedefinitions",
	}).List(ctx, metav1.ListOptions{})

	if err != nil {
		return err
	}

	// collect all installed xrds
	// and then loop over every single composite
	for _, xrd := range xrds.Items {

		// objectbuckets get ignored
		if xrd.GetName() == "xobjectbuckets.appcat.vshn.io" {
			continue
		}

		p := fieldpath.Pave(xrd.Object)

		XRDLabels := xrd.GetLabels()

		if XRDLabels[release.ServiceIDLabel] == "" {
			return fmt.Errorf("xrd does not have the required label " + xrd.GetName())
		}

		compositeResource, err := p.GetString("spec.names.plural")
		if err != nil {
			return err
		}

		// TODO: currently we only have v1, but that might change at some point...
		compositeVersion := "v1"
		compositeGroup, err := p.GetString("spec.group")
		if err != nil {
			return err
		}

		foundGVK := schema.GroupVersionResource{
			Resource: compositeResource,
			Version:  compositeVersion,
			Group:    compositeGroup,
		}

		l, err := dynClient.Resource(foundGVK).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, composite := range l.Items {
			p := fieldpath.Pave(composite.Object)
			log := log.WithValues("xrd", compositeResource, "composite", composite.GetName())

			log.Info("checking for new compositionrevision")

			claimRef, err := p.GetStringObject("spec.claimRef")
			if err != nil {
				if !fieldpath.IsNotFound(err) {
					return err
				}

				err := h.handleComposite(ctx, composite, XRDLabels[release.ServiceIDLabel], log, kubeClient)
				if err != nil {
					return err
				}
			} else {
				err = h.handleClaimRef(ctx, claimRef, log, kubeClient, XRDLabels, composite.GetName())
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (h *hotfixer) handleComposite(ctx context.Context, comp unstructured.Unstructured, serviceID string, log logr.Logger, kubeClient client.Client) error {

	gv, err := schema.ParseGroupVersion(comp.GetAPIVersion())
	if err != nil {
		return err
	}

	opts := release.ReleaserOpts{
		Composite:      comp.GetName(),
		ClaimName:      comp.GetLabels()["crossplane.io/claim-name"],
		ClaimNamespace: comp.GetLabels()["crossplane.io/claim-namespace"],
		Group:          gv.Group,
		Kind:           comp.GetKind(),
		Version:        gv.Version,
		ServiceID:      serviceID,
	}

	r := release.NewDefaultVersionHandler(
		log,
		opts,
	)

	enabled, err := strconv.ParseBool(viper.GetString("RELEASE_MANAGEMENT_ENABLED"))
	if err != nil {
		return fmt.Errorf("cannot determine if release management is enabled: %w", err)
	}

	return r.ReleaseLatest(ctx, enabled, kubeClient)
}

func (h *hotfixer) handleClaimRef(ctx context.Context, ref map[string]string, log logr.Logger, kubeClient client.Client, labels map[string]string, compName string) error {

	gv, err := schema.ParseGroupVersion(ref["apiVersion"])
	if err != nil {
		return err
	}

	opts := release.ReleaserOpts{
		ClaimName:      ref["name"],
		Composite:      compName,
		ClaimNamespace: ref["namespace"],
		Group:          gv.Group,
		Version:        gv.Version,
		Kind:           ref["kind"],
		ServiceID:      labels[release.ServiceIDLabel],
	}

	r := release.NewDefaultVersionHandler(
		log,
		opts,
	)

	enabled, err := strconv.ParseBool(viper.GetString("RELEASE_MANAGEMENT_ENABLED"))
	if err != nil {
		return fmt.Errorf("cannot determine if release management is enabled: %w", err)
	}

	return r.ReleaseLatest(ctx, enabled, kubeClient)
}
