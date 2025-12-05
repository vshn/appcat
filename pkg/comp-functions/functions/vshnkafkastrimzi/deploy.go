package vshnkafkastrimzi

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	cmv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	xfnproto "github.com/crossplane/function-sdk-go/proto/v1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"

	"github.com/vshn/appcat/v4/pkg/common/utils"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/common"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	certificateSecretName = "tls-certificate"
	namespaceResName      = "namespace-conditions"
)

func DeployKafkaStrimzi(ctx context.Context, comp *vshnv1.VSHNKafkaStrimzi, svc *runtime.ServiceRuntime) *xfnproto.Result {
	l := svc.Log

	l.Info("Deploying Kafka Strimzi...")

	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get observed composite: %w", err))
	}

	l.Info("Bootstrapping instance namespace and rbac rules")
	err = common.BootstrapInstanceNs(ctx, comp, "kafkastrimzi", namespaceResName, svc, map[string]string{})
	if err != nil {
		return runtime.NewWarningResult(fmt.Errorf("cannot bootstrap instance namespace: %w", err).Error())
	}

	if comp.Spec.Parameters.TLS.TLSEnabled {
		l.Info("Create tls certificate")
		err = createCerts(comp, svc)
		if err != nil {
			return runtime.NewWarningResult(fmt.Errorf("cannot create tls certificate: %w", err).Error())
		}
	}

	return deployKafkaStrimziUsingHelm(ctx, comp, svc)
}

func createCerts(comp *vshnv1.VSHNKafkaStrimzi, svc *runtime.ServiceRuntime) error {
	selfSignedIssuer := &cmv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName(),
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: cmv1.IssuerSpec{
			IssuerConfig: cmv1.IssuerConfig{
				SelfSigned: &cmv1.SelfSignedIssuer{
					CRLDistributionPoints: []string{},
				},
			},
		},
	}

	// KubeOptionProtectedBy will set to the helm release which is comp.GetName()
	protectedBy := comp.GetName()

	err := svc.SetDesiredKubeObjectWithName(selfSignedIssuer, comp.GetName()+"-localca", "local-ca", runtime.KubeOptionProtectedBy(protectedBy))
	if err != nil {
		err = fmt.Errorf("cannot create local ca object: %w", err)
		return err
	}

	svcName := comp.GetName() + "-kafka-bootstrap"
	certificate := &cmv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      comp.GetName(),
			Namespace: comp.GetInstanceNamespace(),
		},
		Spec: cmv1.CertificateSpec{
			SecretName: certificateSecretName,
			Duration: &metav1.Duration{
				Duration: time.Duration(87600 * time.Hour),
			},
			RenewBefore: &metav1.Duration{
				Duration: time.Duration(2400 * time.Hour),
			},
			Subject: &cmv1.X509Subject{
				Organizations: []string{
					"vshn-appcat",
				},
			},
			IsCA: false,
			PrivateKey: &cmv1.CertificatePrivateKey{
				Algorithm: cmv1.RSAKeyAlgorithm,
				Encoding:  cmv1.PKCS1,
				Size:      4096,
			},
			Usages: []cmv1.KeyUsage{"server auth", "client auth"},
			DNSNames: []string{
				svcName + "." + comp.GetInstanceNamespace() + ".svc.cluster.local",
				svcName + "." + comp.GetInstanceNamespace() + ".svc",
			},
			IssuerRef: certmgrv1.ObjectReference{
				Name:  comp.GetName(),
				Kind:  selfSignedIssuer.GetObjectKind().GroupVersionKind().Kind,
				Group: selfSignedIssuer.GetObjectKind().GroupVersionKind().Group,
			},
		},
	}

	err = svc.SetDesiredKubeObjectWithName(certificate, comp.GetName()+"-certificate", "certificate", runtime.KubeOptionProtectedBy(protectedBy))
	if err != nil {
		err = fmt.Errorf("cannot create local ca object: %w", err)
		return err
	}

	return nil
}

// Deploy Kafka using the Strimzi Kafka helm chart
func deployKafkaStrimziUsingHelm(ctx context.Context, comp *vshnv1.VSHNKafkaStrimzi, svc *runtime.ServiceRuntime) *xfnproto.Result {
	values, err := createKafkaHelmValues(ctx, svc, comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create helm values: %w", err))
	}

	svc.Log.Info("Creating Helm release for Kafka Strimzi")
	release, err := common.NewRelease(ctx, svc, comp, values, comp.GetName()+"-kafka")
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot create release: %w", err))
	}

	// Release overrides - use vshnkafkastrimzicluster chart
	release.Spec.ForProvider.Chart.Repository = svc.Config.Data["kafkaStrimziChartSource"]
	release.Spec.ForProvider.Chart.Version = svc.Config.Data["kafkaStrimziChartVersion"]
	release.Spec.ForProvider.Chart.Name = "vshnkafkastrimzicluster"

	err = svc.SetDesiredComposedResource(release)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot set desired release: %w", err))
	}
	return nil
}

// Generate Kafka Strimzi helm chart values
func createKafkaHelmValues(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKafkaStrimzi) (map[string]any, error) {
	values := map[string]any{
		"nameOverride": comp.GetName(),
		"namespace":    comp.GetInstanceNamespace(),
		"size": map[string]any{
			"cpu":    "",
			"memory": "",
			"disk":   "",
		},
		"kafka": map[string]any{
			"version":  comp.Spec.Parameters.Service.Version,
			"replicas": 3,
			"rack": map[string]any{
				"topologyKey": comp.GetTopologyKey(),
			},
			"controllers": map[string]any{
				"replicas": 3,
				"cpu":      "250m",
				"memory":   "1Gi",
				"disk":     "5Gi",
			},
			"config": map[string]any{
				"offsetsTopicReplicationFactor":        3,
				"transactionStateLogReplicationFactor": 3,
				"transactionStateLogMinIsr":            2,
				"defaultReplicationFactor":             3,
				"minInsyncReplicas":                    2,
			},
			"storage": map[string]any{
				"type":        "persistent-claim",
				"deleteClaim": false,
			},
		},
		"entityOperator": map[string]any{
			"topicOperator": map[string]any{
				"resources": map[string]any{
					"requests": map[string]any{
						"memory": "512Mi",
						"cpu":    "100m",
					},
					"limits": map[string]any{
						"memory": "512Mi",
						"cpu":    "200m",
					},
				},
			},
			"userOperator": map[string]any{
				"resources": map[string]any{
					"requests": map[string]any{
						"memory": "512Mi",
						"cpu":    "100m",
					},
					"limits": map[string]any{
						"memory": "512Mi",
						"cpu":    "200m",
					},
				},
			},
		},
	}

	// Compute resources
	svc.Log.Info("Fetching and setting compute resources")
	plan := comp.Spec.Parameters.Size.GetPlan(svc.Config.Data["defaultPlan"])
	res, err := getResourcesForPlan(ctx, svc, comp, plan)
	if err != nil {
		return map[string]any{}, fmt.Errorf("could not set resources: %w", err)
	}

	err = setResourcesKafka(values, res)
	if err != nil {
		return map[string]any{}, fmt.Errorf("cannot set resources: %w", err)
	}

	return values, nil
}

// Set compute resources in the values map
func setResourcesKafka(values map[string]any, resources common.Resources) error {
	err := common.SetNestedObjectValue(values, []string{"size", "cpu"}, resources.CPU.String())
	if err != nil {
		return fmt.Errorf("cannot set cpu: %w", err)
	}

	err = common.SetNestedObjectValue(values, []string{"size", "memory"}, resources.Mem.String())
	if err != nil {
		return fmt.Errorf("cannot set memory: %w", err)
	}

	err = common.SetNestedObjectValue(values, []string{"size", "disk"}, resources.Disk.String())
	if err != nil {
		return fmt.Errorf("cannot set disk size: %w", err)
	}

	return nil
}

// Get resources for a given plan
func getResourcesForPlan(ctx context.Context, svc *runtime.ServiceRuntime, comp *vshnv1.VSHNKafkaStrimzi, plan string) (common.Resources, error) {
	resources, err := utils.FetchPlansFromConfig(ctx, svc, plan)
	if err != nil {
		return common.Resources{}, err
	}

	res, errs := common.GetResources(&comp.Spec.Parameters.Size, resources)
	if len(errs) > 0 {
		return common.Resources{}, errors.Join(errs...)
	}

	return res, nil
}
