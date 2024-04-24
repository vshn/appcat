package cmd

import (
	"os"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/vshn/appcat/v4/pkg"
	maintenancecontroller "github.com/vshn/appcat/v4/pkg/sliexporter/maintenance_controller"
	"github.com/vshn/appcat/v4/pkg/sliexporter/probes"
	vshnkeycloakcontroller "github.com/vshn/appcat/v4/pkg/sliexporter/vshnkeycloak_controller"
	vshnmariadbcontroller "github.com/vshn/appcat/v4/pkg/sliexporter/vshnmariadb_controller"
	vshnminiocontroller "github.com/vshn/appcat/v4/pkg/sliexporter/vshnminio_controller"
	vshnpostgresqlcontroller "github.com/vshn/appcat/v4/pkg/sliexporter/vshnpostgresql_controller"
	vshnrediscontroller "github.com/vshn/appcat/v4/pkg/sliexporter/vshnredis_controller"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

type sliProber struct {
	scheme                                                                                                                      *runtime.Scheme
	metricsAddr, probeAddr                                                                                                      string
	leaderElect, enableVSHNPostgreSQL, enableVSHNRedis, enableVSHNMinio, enableMaintenanceStatus, enableKeycloak, enableMariaDB bool
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
	SLIProberCMD.Flags().BoolVar(&s.enableVSHNRedis, "vshn-redis", getEnvBool("APPCAT_SLI_VSHNREDIS"),
		"Enable probing of VSHNRedis instances")
	SLIProberCMD.Flags().BoolVar(&s.enableVSHNMinio, "vshn-minio", getEnvBool("APPCAT_SLI_VSHNMINIO"),
		"Enable probing of VSHNMinio instances")
	SLIProberCMD.Flags().BoolVar(&s.enableMaintenanceStatus, "vshn-track-oc-maintenance-status", getEnvBool("APPCAT_SLI_TRACK_OC_MAINTENANCE_STATUS"),
		"Enable oc maintenance status observer. Will set the labels 'maintenance' accordingly.")
	SLIProberCMD.Flags().BoolVar(&s.enableKeycloak, "vshn-keycloak", getEnvBool("APPCAT_SLI_VSHNKEYCLOAK"), "Enable probing of VSHNKeycloak instances")
	SLIProberCMD.Flags().BoolVar(&s.enableMariaDB, "vshn-mariadb", getEnvBool("APPCAT_SLI_VSHNMARIADB"), "Enable probing of VSHNMariaDB instances")
}

func (s *sliProber) executeSLIProber(cmd *cobra.Command, _ []string) error {
	log := logr.FromContextOrDiscard(cmd.Context())
	ctrl.SetLogger(log)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 s.scheme,
		HealthProbeBindAddress: s.probeAddr,
		LeaderElection:         s.leaderElect,
		LeaderElectionID:       "05f8b574.appcat.vshn.io",
		Metrics: server.Options{
			BindAddress: s.metricsAddr,
		},
	})
	if err != nil {
		log.Error(err, "unable to start manager")
		return err
	}

	maintenanceRecociler := &maintenancecontroller.MaintenanceReconciler{
		Client: mgr.GetClient(),
	}

	probeManager := probes.NewManager(log, maintenanceRecociler)

	err = metrics.Registry.Register(probeManager.Collector())
	if err != nil {
		log.Error(err, "unable to register metrics")
		return err
	}

	if s.enableVSHNPostgreSQL {
		if err = (&vshnpostgresqlcontroller.VSHNPostgreSQLReconciler{
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
	if s.enableVSHNRedis {
		log.Info("Enabling VSHNRedis controller")
		if err = (&vshnrediscontroller.VSHNRedisReconciler{
			Client:             mgr.GetClient(),
			Scheme:             mgr.GetScheme(),
			ProbeManager:       &probeManager,
			StartupGracePeriod: 10 * time.Minute,
			RedisDialer:        probes.NewRedis,
		}).SetupWithManager(mgr); err != nil {
			log.Error(err, "unable to create controller", "controller", "VSHNRedis")
			return err
		}
	}

	if s.enableVSHNMinio {
		log.Info("Enabling VSHNRedis controller")
		if err = (&vshnminiocontroller.VSHNMinioReconciler{
			Client:             mgr.GetClient(),
			Scheme:             mgr.GetScheme(),
			ProbeManager:       &probeManager,
			StartupGracePeriod: 10 * time.Minute,
			MinioDialer:        probes.NewMinio,
		}).SetupWithManager(mgr); err != nil {
			log.Error(err, "unable to create controller", "controller", "VSHNRedis")
			return err
		}
	}

	if s.enableKeycloak {
		log.Info("Enablign VSHNKeycloak controller")
		if err = (&vshnkeycloakcontroller.VSHNKeycloakReconciler{
			Client:             mgr.GetClient(),
			Scheme:             mgr.GetScheme(),
			ProbeManager:       &probeManager,
			StartupGracePeriod: 10 * time.Minute,
		}).SetupWithManager(mgr); err != nil {
			log.Error(err, "unable to create controller", "controller", "VSHNKeycloak")
			return err
		}
	}

	if s.enableMaintenanceStatus {
		log.Info("Enable OC maintenance observer")
		if err = maintenanceRecociler.SetupWithManager(mgr); err != nil {
			log.Error(err, "unable to create controller", "controller", "Maintenance Observer")
			return err
		}
	}
	if true {
		log.Info("Enabling VSHNMariaDB controller")
		if err = (&vshnmariadbcontroller.VSHNMariaDBReconciler{
			Client:             mgr.GetClient(),
			Scheme:             mgr.GetScheme(),
			ProbeManager:       &probeManager,
			StartupGracePeriod: 1 * time.Minute,
			MariaDBDialer:      probes.NewMariaDB,
		}).SetupWithManager(mgr); err != nil {
			log.Error(err, "unable to create controller", "controller", "VSHNMariadb")
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
