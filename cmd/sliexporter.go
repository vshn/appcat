package cmd

import (
	"os"
	"time"

	managedupgradev1beta1 "github.com/appuio/openshift-upgrade-controller/api/v1beta1"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/common/utils"
	maintenancecontroller "github.com/vshn/appcat/v4/pkg/sliexporter/maintenance_controller"
	"github.com/vshn/appcat/v4/pkg/sliexporter/probes"
	vshnkeycloakcontroller "github.com/vshn/appcat/v4/pkg/sliexporter/vshnkeycloak_controller"
	vshnmariadbcontroller "github.com/vshn/appcat/v4/pkg/sliexporter/vshnmariadb_controller"
	vshnminiocontroller "github.com/vshn/appcat/v4/pkg/sliexporter/vshnminio_controller"
	vshnpostgresqlcontroller "github.com/vshn/appcat/v4/pkg/sliexporter/vshnpostgresql_controller"
	vshnrediscontroller "github.com/vshn/appcat/v4/pkg/sliexporter/vshnredis_controller"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

type sliProber struct {
	scheme                                    *runtime.Scheme
	metricsAddr, probeAddr, serviceKubeConfig string
	leaderElect                               bool
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

const (
	startupGraceMin = 10
)

func init() {
	SLIProberCMD.Flags().StringVar(&s.metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	SLIProberCMD.Flags().StringVar(&s.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	SLIProberCMD.Flags().BoolVar(&s.leaderElect, "leader-elect", false, "Enable leader election for controller manager. "+
		"Enabling this will ensure there is only one active controller manager.")
	SLIProberCMD.Flags().StringVar(&s.serviceKubeConfig, "service-kubeconfig", os.Getenv("SERVICE_KUBECONFIG"),
		"Kubeconfig for the service cluster itself, usually only for debugging purpose. The sliprober should run on the service clusters and thus use the in-cluster-config.")
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

	config, err := getServiceClusterConfig()
	if err != nil {
		return err
	}

	scClient, err := client.New(config, client.Options{
		Scheme: pkg.SetupScheme(),
	})
	if err != nil {
		return err
	}

	maintenanceRecociler := &maintenancecontroller.MaintenanceReconciler{
		Client: scClient,
	}

	probeManager := probes.NewManager(log, maintenanceRecociler)

	err = metrics.Registry.Register(probeManager.Collector())
	if err != nil {
		log.Error(err, "unable to register metrics")
		return err
	}

	if utils.IsKindAvailable(vshnv1.GroupVersion, "XVSHNPostgreSQL", ctrl.GetConfigOrDie()) {
		log.Info("Enabling VSHNPostgreSQL controller")
		if err = (&vshnpostgresqlcontroller.VSHNPostgreSQLReconciler{
			Client:             mgr.GetClient(),
			Scheme:             mgr.GetScheme(),
			ProbeManager:       &probeManager,
			StartupGracePeriod: startupGraceMin * time.Minute,
			PostgreDialer:      probes.NewPostgreSQL,
			ScClient:           scClient,
		}).SetupWithManager(mgr); err != nil {
			log.Error(err, "unable to create controller", "controller", "VSHNPostgreSQL")
			return err
		}
	}
	if utils.IsKindAvailable(vshnv1.GroupVersion, "XVSHNRedis", ctrl.GetConfigOrDie()) {
		log.Info("Enabling VSHNRedis controller")
		if err = (&vshnrediscontroller.VSHNRedisReconciler{
			Client:             mgr.GetClient(),
			Scheme:             mgr.GetScheme(),
			ProbeManager:       &probeManager,
			StartupGracePeriod: startupGraceMin * time.Minute,
			RedisDialer:        probes.NewRedis,
			ScClient:           scClient,
		}).SetupWithManager(mgr); err != nil {
			log.Error(err, "unable to create controller", "controller", "VSHNRedis")
			return err
		}
	}

	if utils.IsKindAvailable(vshnv1.GroupVersion, "XVSHNMinio", ctrl.GetConfigOrDie()) {
		log.Info("Enabling VSHNRedis controller")
		if err = (&vshnminiocontroller.VSHNMinioReconciler{
			Client:             mgr.GetClient(),
			Scheme:             mgr.GetScheme(),
			ProbeManager:       &probeManager,
			StartupGracePeriod: startupGraceMin * time.Minute,
			MinioDialer:        probes.NewMinio,
			ScClient:           scClient,
		}).SetupWithManager(mgr); err != nil {
			log.Error(err, "unable to create controller", "controller", "VSHNRedis")
			return err
		}
	}

	if utils.IsKindAvailable(vshnv1.GroupVersion, "XVSHNKeycloak", ctrl.GetConfigOrDie()) {
		log.Info("Enablign VSHNKeycloak controller")
		if err = (&vshnkeycloakcontroller.VSHNKeycloakReconciler{
			Client:             mgr.GetClient(),
			Scheme:             mgr.GetScheme(),
			ProbeManager:       &probeManager,
			StartupGracePeriod: startupGraceMin * time.Minute,
			ScClient:           scClient,
		}).SetupWithManager(mgr); err != nil {
			log.Error(err, "unable to create controller", "controller", "VSHNKeycloak")
			return err
		}
	}

	if utils.IsKindAvailable(managedupgradev1beta1.GroupVersion, "UpgradeJob", config) {
		log.Info("Enable OC maintenance observer")
		serviceCluster, err := getServiceCluster(config)
		if err != nil {
			return err
		}
		if err = maintenanceRecociler.SetupWithManager(mgr, serviceCluster); err != nil {
			log.Error(err, "unable to create controller", "controller", "Maintenance Observer")
			return err
		}
		err = mgr.Add(*serviceCluster)
		if err != nil {
			return err
		}
	}
	if utils.IsKindAvailable(vshnv1.GroupVersion, "XVSHNMariaDB", ctrl.GetConfigOrDie()) {
		log.Info("Enabling VSHNMariaDB controller")
		if err = (&vshnmariadbcontroller.VSHNMariaDBReconciler{
			Client:             mgr.GetClient(),
			Scheme:             mgr.GetScheme(),
			ProbeManager:       &probeManager,
			StartupGracePeriod: startupGraceMin * time.Minute,
			MariaDBDialer:      probes.NewMariaDB,
			ScClient:           scClient,
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

func getServiceCluster(config *rest.Config) (*cluster.Cluster, error) {
	serviceCluster, err := cluster.New(config, func(o *cluster.Options) {
		o.Scheme = pkg.SetupScheme()
	})
	return &serviceCluster, err
}

// getServiceClusterConfig will create an incluster config by default.
// If serviceKubeConfig is set, it will use that instead.
func getServiceClusterConfig() (*rest.Config, error) {

	if s.serviceKubeConfig != "" {
		kubeconfig, err := os.ReadFile(s.serviceKubeConfig)
		if err != nil {
			return nil, err
		}

		clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
		if err != nil {
			return nil, err
		}

		config, err := clientConfig.ClientConfig()
		if err != nil {
			return nil, err
		}
		return config, nil
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	return config, nil
}
