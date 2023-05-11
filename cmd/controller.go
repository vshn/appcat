package cmd

import (
	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	vshnv1 "github.com/vshn/appcat-apiserver/apis/vshn/v1"
	"github.com/vshn/appcat-apiserver/pkg/controller/postgres"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

type controller struct {
	scheme                  *runtime.Scheme
	metricsAddr, healthAddr string
	leaderElect             bool
}

var c = controller{
	scheme: runtime.NewScheme(),
}

var ControllerCMD = &cobra.Command{
	Use:   "controller",
	Short: "Controller",
	Long:  "Run the Controller",
	RunE:  c.executeController,
}

func init() {
	_ = corev1.SchemeBuilder.AddToScheme(c.scheme)
	_ = xkube.SchemeBuilder.AddToScheme(c.scheme)
	_ = vshnv1.SchemeBuilder.SchemeBuilder.AddToScheme(c.scheme)

	ControllerCMD.Flags().StringVar(&c.metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	ControllerCMD.Flags().StringVar(&c.healthAddr, "health-addr", ":8081", "The address the probe endpoint binds to.")
	ControllerCMD.Flags().BoolVar(&c.leaderElect, "leader-elect", false, "Enable leader election for controller manager. "+
		"Enabling this will ensure there is only one active controller manager.")
}

// Run will run the controller mode of the composition function runner.
func (c *controller) executeController(cmd *cobra.Command, _ []string) error {
	log := logr.FromContextOrDiscard(cmd.Context())
	ctrl.SetLogger(log)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 c.scheme,
		Port:                   9443,
		HealthProbeBindAddress: c.healthAddr,
		LeaderElection:         c.leaderElect,
		LeaderElectionID:       "35t6u158.appcat.vshn.io",
	})
	if err != nil {
		return err
	}

	xpg := &postgres.XPostgreSQLReconciler{
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
