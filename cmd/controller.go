package cmd

import (
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/controller/postgres"
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
	scheme: pkg.SetupScheme(),
}

var ControllerCMD = &cobra.Command{
	Use:   "controller",
	Short: "Controller",
	Long:  "Run the Controller",
	RunE:  c.executeController,
}

func init() {
	ControllerCMD.Flags().StringVar(&c.metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
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
