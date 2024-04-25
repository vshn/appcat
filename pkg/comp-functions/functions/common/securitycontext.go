package common

import (
	"fmt"
	"strconv"

	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func SetSELinuxSecurityContextStatefulset(sts *appsv1.StatefulSet, comp InfoGetter, svc *runtime.ServiceRuntime) error {
	return setSELinuxSecurityContext(sts, &sts.Spec.Template.Spec, comp, svc)
}

func SetSELinuxSecurityContextDeployment(depl *appsv1.Deployment, comp InfoGetter, svc *runtime.ServiceRuntime) error {
	return setSELinuxSecurityContext(depl, &depl.Spec.Template.Spec, comp, svc)
}

func setSELinuxSecurityContext(obj client.Object, spec *corev1.PodSpec, comp InfoGetter, svc *runtime.ServiceRuntime) error {
	configString := svc.Config.Data["isOpenshift"]
	isOpenShift, err := strconv.ParseBool(configString)
	if err != nil {
		return fmt.Errorf("cannot determine if this is an OpenShift cluster or not")
	}
	if !isOpenShift {
		return nil
	}

	err = svc.GetObservedKubeObject(obj, comp.GetName()+"-securitycontext")
	if err != nil {
		// it has not yet been deployed
		if err != runtime.ErrNotFound {
			return fmt.Errorf("cannot get observed statefulet or deployment: %w", err)
		}
		err = svc.SetDesiredKubeObject(obj, comp.GetName()+"-securitycontext")
		if err != nil {
			return fmt.Errorf("cannot deploy sts/deployment observer for securitycontext: %W", err)
		}
		return nil
	}

	spec.SecurityContext.FSGroupChangePolicy = ptr.To(corev1.FSGroupChangeOnRootMismatch)
	spec.SecurityContext.SELinuxOptions = &corev1.SELinuxOptions{
		Type: "spc_t",
	}

	err = svc.SetDesiredKubeObject(obj, comp.GetName()+"-securitycontext", runtime.KubeOptionObserveCreateUpdate)
	if err != nil {
		return fmt.Errorf("cannot apply securitycontext: %W", err)
	}

	return nil
}
