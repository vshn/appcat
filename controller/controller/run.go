package controller

import (
	"github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"
	vshnv1 "github.com/vshn/component-appcat/apis/vshn/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

var s = runtime.NewScheme()

func init() {
	_ = corev1.SchemeBuilder.AddToScheme(s)
	_ = appsv1.SchemeBuilder.AddToScheme(s)
	_ = v1alpha1.SchemeBuilder.AddToScheme(s)
	_ = vshnv1.SchemeBuilder.SchemeBuilder.AddToScheme(s)
}

// RunController will run the controller mode of the composition function runner.
func RunController(cli *cli.Context) error {

	log := logr.FromContextOrDiscard(cli.Context)

	ctrl.SetLogger(log)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 s,
		MetricsBindAddress:     cli.String("metrics-addr"),
		Port:                   9443,
		HealthProbeBindAddress: cli.String("health-addr"),
		LeaderElection:         cli.Bool("leader-elect"),
		LeaderElectionID:       "35t6u158.appcat.vshn.io",
	})
	if err != nil {
		return err
	}

	xpg := &xpostgreSQLDeletionProtectionReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	err = xpg.SetupWithManager(mgr)
	if err != nil {
		return err
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return err
	}

	return mgr.Start(ctrl.SetupSignalHandler())
}
