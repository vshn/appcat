package vshnforgejo

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	xhelmv1 "github.com/vshn/appcat/v4/apis/helm/release/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/tcproute"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
)

const (
	sshListenerName  = "ssh"
	sshServicePort   = 22
	sshPodListenPort = 2222
)

// ConfigureSSHAccess creates the Gateway API resources needed for TCP routing
// when SSH access is enabled on a Forgejo instance.
func ConfigureSSHAccess(ctx context.Context, comp *vshnv1.VSHNForgejo, svc *runtime.ServiceRuntime) *xfnproto.Result {
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	if !comp.Spec.Parameters.Service.SSH.Enabled {
		return nil
	}

	cfg := tcproute.TCPRouteConfig{
		ResourceName:       comp.GetName() + "-ssh",
		ListenerName:       sshListenerName,
		BackendServiceName: helmFullname(comp) + "-ssh",
		BackendServicePort: sshServicePort,
		PodListenPort:      sshPodListenPort,
		PodSelectorLabels: map[string]string{
			"app.kubernetes.io/name":     comp.GetServiceName(),
			"app.kubernetes.io/instance": comp.GetName(),
		},
		InstanceNamespace: comp.GetInstanceNamespace(),
	}

	result, state := tcproute.AddTCPRoute(svc, cfg)
	if result != nil {
		return result
	}

	// Forgejo-specific: update connection details + Helm values
	if state.Port > 0 {
		comp.Status.SSHPort = state.Port
		err := svc.SetDesiredCompositeStatus(comp)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("cannot update composite status: %w", err))
		}

		svc.SetConnectionDetail("FORGEJO_SSH_HOST", []byte(state.Domain))
		svc.SetConnectionDetail("FORGEJO_SSH_PORT", []byte(strconv.FormatInt(int64(state.Port), 10)))
		err = enableSSHInRelease(svc, comp, state.Domain, state.Port)
		if err != nil {
			return runtime.NewFatalResult(fmt.Errorf("cannot update forgejo release: %w", err))
		}
	}

	return nil
}

// enableSSHInRelease modifies the Helm release values to enable SSH in Forgejo.
func enableSSHInRelease(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNForgejo, sshDomain string, allocatedPort int32) error {
	release := &xhelmv1.Release{}
	err := svc.GetDesiredComposedResourceByName(release, comp.GetName())
	if err != nil {
		return fmt.Errorf("getting release: %w", err)
	}

	values, err := common.GetReleaseValues(release)
	if err != nil {
		return fmt.Errorf("getting release values: %w", err)
	}

	err = common.SetNestedObjectValue(values, []string{"gitea", "config", "server", "DISABLE_SSH"}, false)
	if err != nil {
		return err
	}

	err = common.SetNestedObjectValue(values, []string{"gitea", "config", "server", "START_SSH_SERVER"}, true)
	if err != nil {
		return err
	}

	if sshDomain != "" {
		err := common.SetNestedObjectValue(values, []string{"gitea", "config", "server", "SSH_DOMAIN"}, sshDomain)
		if err != nil {
			return err
		}
	}
	if allocatedPort > 0 {
		err := common.SetNestedObjectValue(values, []string{"gitea", "config", "server", "SSH_PORT"}, int(allocatedPort))
		if err != nil {
			return err
		}
	}

	byteValues, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("marshaling values: %w", err)
	}
	release.Spec.ForProvider.Values.Raw = byteValues

	return svc.SetDesiredComposedResourceWithName(release, comp.GetName())
}
