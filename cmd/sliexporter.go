package cmd

import (
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/vshn/appcat/pkg"
	"github.com/vshn/appcat/pkg/sliexporter"
	"github.com/vshn/appcat/pkg/sliexporter/probes"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"strconv"
	"time"
)

type sliProber struct {
	scheme                            *runtime.Scheme
	metricsAddr, probeAddr            string
	leaderElect, enableVSHNPostgreSQL bool
}

var s = sliProber{
	scheme: pkg.SetupScheme(),
}

var SLIProberCMD = &cobra.Command{
	Use:   "sliprober",
	Short: "SLI Prober Controller",
	Long:  "Run the SLI Prober Controller",
	RunE:  s.executeSLIProber,
}

func init() {
	SLIProberCMD.Flags().StringVar(&s.metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	SLIProberCMD.Flags().StringVar(&s.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	SLIProberCMD.Flags().BoolVar(&s.leaderElect, "leader-elect", false, "Enable leader election for controller manager. "+
		"Enabling this will ensure there is only one active controller manager.")
	SLIProberCMD.Flags().BoolVar(&s.enableVSHNPostgreSQL, "vshn-postgresql", getEnvBool("APPCAT_SLI_VSHNPOSTGRESQL"),
		"Enable probing of VSHNPostgreSQL instances")
}

func (s *sliProber) executeSLIProber(cmd *cobra.Command, _ []string) error {
	log := logr.FromContextOrDiscard(cmd.Context())
	ctrl.SetLogger(log)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 s.scheme,
		MetricsBindAddress:     s.metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: s.probeAddr,
		LeaderElection:         s.leaderElect,
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

	if s.enableVSHNPostgreSQL {
		if err = (&sliexporter.VSHNPostgreSQLReconciler{
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