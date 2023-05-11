package command

import (
	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	vshnv1 "github.com/vshn/appcat-apiserver/apis/vshn/v1"
	sli_exporter "github.com/vshn/appcat-apiserver/controller/sli-exporter"
	"github.com/vshn/appcat-apiserver/controller/sli-exporter/probes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"strconv"
	"time"
)

func init() {
	_ = corev1.SchemeBuilder.AddToScheme(s_prober)
	_ = xkube.SchemeBuilder.AddToScheme(s_prober)
	_ = vshnv1.SchemeBuilder.SchemeBuilder.AddToScheme(s_prober)

	sliProber.Flags().StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	sliProber.Flags().StringVar(&healthAddr, "health-addr", ":8081", "The address the probe endpoint binds to.")
	sliProber.Flags().BoolVar(&leaderElect, "leader-elect", false, "Enable leader election for controller manager. "+
		"Enabling this will ensure there is only one active controller manager.")
	sliProber.Flags().BoolVar(&enableVSHNPostgreSQL, "vshn-postgresql", getEnvBool("APPCAT_SLI_VSHNPOSTGRESQL"),
		"Enable probing of VSHNPostgreSQL instances")
}

var sliProber = &cobra.Command{
	Use:   "sli-prober",
	Short: "SLI Prober Controller",
	Long:  "Run the SLI Prober Controller",
	RunE:  executeProberController,
}

var s_prober = runtime.NewScheme()

// Run will run the controller mode of the composition function runner.
func executeProberController(cmd *cobra.Command, _ []string) error {
	log := logr.FromContextOrDiscard(cmd.Context())
	ctrl.SetLogger(log)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 s_prober,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         leaderElect,
		LeaderElectionID:       "05f8b574.appcat.vshn.io",
	})
	if err != nil {
		log.Error(err, "unable to start manager")
		return err
	}
	probeManager := probes.NewManager(log)

	err = metrics.Registry.Register(probeManager.Collector())
	if err != nil {
		log.Error(err, "unable to register metrics")
		return err
	}

	if enableVSHNPostgreSQL {
		if err = (&sli_exporter.VSHNPostgreSQLReconciler{
			Client:             mgr.GetClient(),
			Scheme:             mgr.GetScheme(),
			ProbeManager:       &probeManager,
			StartupGracePeriod: 15 * time.Minute,
			PostgreDialer:      probes.NewPostgreSQL,
		}).SetupWithManager(mgr); err != nil {
			log.Error(err, "unable to create controller", "controller", "VSHNPostgreSQL")
			return err
		}
	}
	//+kubebuilder:scaffold:builder

	if err = mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up health check")
		return err
	}
	if err = mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error(err, "unable to set up ready check")
		return err
	}

	log.Info("starting manager")
	if err = mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Error(err, "problem running manager")
		return err
	}
	return nil
}

func getEnvBool(key string) bool {
	b, err := strconv.ParseBool(os.Getenv(key))
	return err == nil && b
}
