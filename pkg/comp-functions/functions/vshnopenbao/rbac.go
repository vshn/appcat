package vshnopenbao

import (
	"context"
	"fmt"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const discoveryRoleSuffix = "-discovery"

// CreateDiscoveryRBAC creates the Role and RoleBinding that allow the OpenBao
// service account to list pods in its own namespace for Raft auto-join.
// The Helm chart's built-in RBAC is disabled (server.rbac.create=false) because
// provider-helm cannot grant pod permissions it does not itself hold.
func CreateDiscoveryRBAC(ctx context.Context, comp *vshnv1.VSHNOpenBao, svc *runtime.ServiceRuntime) *xfnproto.Result {
	if err := svc.GetObservedComposite(comp); err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	serviceName := comp.GetName()
	ns := comp.GetInstanceNamespace()
	roleName := serviceName + discoveryRoleSuffix

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: ns,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"list"},
			},
		},
	}
	if err := svc.SetDesiredKubeObject(role, roleName); err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot add discovery role: %w", err).Error())
	}

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: ns,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceName,
				Namespace: ns,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     roleName,
		},
	}
	if err := svc.SetDesiredKubeObject(rb, roleName+"-binding"); err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot add discovery rolebinding: %w", err).Error())
	}

	return nil
}
