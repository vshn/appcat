package vshnredis

import (
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common/tcproute"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
)

// gateExternalCertReadiness keeps the server certificate (and thus the
// composite) unready until the external endpoint (gateway domain or
// loadbalancer IP) is present in the connection details and covered by the
// server certificate's SANs. This forces Crossplane to keep reconciling so the
// external connection details get published promptly.
func gateExternalCertReadiness(svc *runtime.ServiceRuntime, comp *vshnv1.VSHNRedis, gatewayHost, loadbalancerIP string) error {
	if !common.ExternalAccessEnabled(svc) {
		return nil
	}

	certResource := comp.GetName() + "-server-cert"
	observer := certResource + "-tls-observer"

	switch comp.Spec.Parameters.Network.ServiceType {
	case tcproute.ServiceTypeTCPGateway:
		if gatewayHost == "" {
			svc.SetDesiredResourceReadiness(certResource, runtime.ResourceUnReady)
			return nil
		}
		return common.WaitForCertSAN(svc, certResource, observer, serverCertificateSecretName, comp.GetInstanceNamespace(), common.CertHasDNS(gatewayHost))
	case string(corev1.ServiceTypeLoadBalancer):
		if loadbalancerIP == "" {
			svc.SetDesiredResourceReadiness(certResource, runtime.ResourceUnReady)
			return nil
		}
		return common.WaitForCertSAN(svc, certResource, observer, serverCertificateSecretName, comp.GetInstanceNamespace(), common.CertHasIP(loadbalancerIP))
	}

	return nil
}
