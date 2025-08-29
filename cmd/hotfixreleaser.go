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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	HotfixerCMD    = newHotfixer()
	serviceIDLabel string
	serviceID      string
)

func init() {
	HotfixerCMD.Flags().StringVar(&serviceIDLabel, "serviceIDLabel", release.DefaultServiceIDLabel, "name of the service id Label")
	HotfixerCMD.Flags().StringVar(&serviceID, "serviceID", "", "name of the service ID to update, if empty will update all")
}

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

		// let's skip if the the serviceID is not empty and matches the service
		if serviceID != "" && xrd.GetLabels()[serviceIDLabel] != serviceID {
			continue
		}

		p := fieldpath.Pave(xrd.Object)

		XRDLabels := xrd.GetLabels()

		if XRDLabels[serviceIDLabel] == "" {
			return fmt.Errorf("xrd does not have the required label " + xrd.GetName())
		}

		compositeResource, err := p.GetString("spec.names.plural")
		if err != nil {
			return err
		}

		// TODO: we currently just take the first version we find
		compositeVersion, err := p.GetString("spec.versions[0].name")
		if err != nil {
			return fmt.Errorf("cannot get version: %w", err)
		}

		compositeGroup, err := p.GetString("spec.group")
		if err != nil {
			return fmt.Errorf("cannot get group: %w", err)
		}

		foundGVK := schema.GroupVersionResource{
			Resource: compositeResource,
			Version:  compositeVersion,
			Group:    compositeGroup,
		}

		l, err := dynClient.Resource(foundGVK).List(ctx, metav1.ListOptions{})
		if err != nil {

			if apierrors.IsNotFound(err) {
				continue
			}

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

				err := h.handleComposite(ctx, composite, XRDLabels[serviceIDLabel], log, kubeClient)
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
		ServiceIDLabel: serviceIDLabel,
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
		ServiceID:      labels[serviceIDLabel],
		ServiceIDLabel: serviceIDLabel,
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
