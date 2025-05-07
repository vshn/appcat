package cmd

import (
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/maintenance/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	xrds, err := dynClient.Resource(schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1",
		Resource: "compositeresourcedefinitions",
	}).List(ctx, metav1.ListOptions{})

	if err != nil {
		return err
	}

	// collect all installed xrds
	foundGVK := []schema.GroupVersionResource{}
	for _, xrd := range xrds.Items {
		p := fieldpath.Pave(xrd.Object)

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

		foundGVK = append(foundGVK, schema.GroupVersionResource{
			Resource: compositeResource,
			Version:  compositeVersion,
			Group:    compositeGroup,
		})
	}

	// get every composite of every xrd
	for _, f := range foundGVK {
		l, err := dynClient.Resource(f).List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, composite := range l.Items {
			p := fieldpath.Pave(composite.Object)
			claimRef, err := p.GetValue("spec.claimRef")
			if err != nil {
				if !fieldpath.IsNotFound(err) {
					return err
				}
				//TODO: do something with this information
				labels, err := p.GetStringObject("metadata.labels")
				if err != nil {
					return err
				}
				fmt.Println(labels)
			}
			fmt.Println(claimRef)
		}

	}

	return nil
}

func (h *hotfixer) handleLabels(labels map[string]string, log logr.Logger) error {

	kubeClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{
		Scheme: pkg.SetupScheme(),
	})
	if err != nil {
		return err
	}

	release.NewDefaultVersionHandler(
		kubeClient,
		log,
		labels["crossplane.io/claim-name"],
		labels["crossplane.io/composite"],
		labels["crossplane.io/claim-namespace"],
		labels["appcat.vshn.io/ownergroup"],
		labels["appcat.vshn.io/ownerkind"],
		labels["appcat.vshn.io/ownergroup"],
		"hotfixer",
	)

	return nil
}

func (h *hotfixer) handleClaimRef() error {
	return nil
}
