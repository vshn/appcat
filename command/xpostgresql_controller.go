package command

import (
	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/vshn/appcat-apiserver/controller/postgres"
	vshnv1 "github.com/vshn/component-appcat/apis/vshn/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

func init() {
	_ = corev1.SchemeBuilder.AddToScheme(s_xpostgres)
	_ = xkube.SchemeBuilder.AddToScheme(s_xpostgres)
	_ = vshnv1.SchemeBuilder.SchemeBuilder.AddToScheme(s_xpostgres)

	xpostgresql.Flags().StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	xpostgresql.Flags().StringVar(&healthAddr, "health-addr", ":8081", "The address the probe endpoint binds to.")
	xpostgresql.Flags().BoolVar(&leaderElect, "leader-elect", false, "Enable leader election for controller manager. "+
		"Enabling this will ensure there is only one active controller manager.")
}

var xpostgresql = &cobra.Command{
	Use:   "postgres-controller",
	Short: "Postgres Controller",
	Long:  "Run the Postgres Controller",
	RunE:  executeXVSHNPostgreSQLController,
}

var s_xpostgres = runtime.NewScheme()

// Run will run the controller mode of the composition function runner.
func executeXVSHNPostgreSQLController(cmd *cobra.Command, _ []string) error {
	log := logr.FromContextOrDiscard(cmd.Context())
	ctrl.SetLogger(log)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 s_xpostgres,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: healthAddr,
		LeaderElection:         leaderElect,
		LeaderElectionID:       "35t6u158.appcat.vshn.io",
	})
	if err != nil {
		return err
	}

	xpg := &postgres.XPostgreSQLDeletionProtectionReconciler{
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
