package cmd

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/controller/events"
	"github.com/vshn/appcat/v4/pkg/controller/postgres"
	"github.com/vshn/appcat/v4/pkg/controller/webhooks"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

type controller struct {
	scheme                  *runtime.Scheme
	metricsAddr, healthAddr string
	leaderElect             bool
	enableWebhooks          bool
	enableQuotas            bool
	certDir                 string
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
	ControllerCMD.Flags().BoolVar(&c.enableWebhooks, "webhooks", true, "Disable the validation webhooks.")
	ControllerCMD.Flags().StringVar(&c.certDir, "certdir", "/etc/webhook/certs", "Set the webhook certificate directory")
	ControllerCMD.Flags().BoolVar(&c.enableQuotas, "quotas", false, "Enable the quota webhooks, is only active if webhooks is also true")
	viper.AutomaticEnv()
	if !viper.IsSet("PLANS_NAMESPACE") {
		viper.Set("PLANS_NAMESPACE", "syn-appcat")
	}
}

// Run will run the controller mode of the composition function runner.
func (c *controller) executeController(cmd *cobra.Command, _ []string) error {
	log := logr.FromContextOrDiscard(cmd.Context())
	ctrl.SetLogger(log)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 c.scheme,
		HealthProbeBindAddress: c.healthAddr,
		LeaderElection:         c.leaderElect,
		LeaderElectionID:       "35t6u158.appcat.vshn.io",
		Metrics: server.Options{
			BindAddress: c.metricsAddr,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:    9443,
			CertDir: c.certDir,
		}),
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

	events := &events.EventHandler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	err = events.SetupWithManager(mgr)

	if err != nil {
		return err
	}

	if c.enableWebhooks {

		if !viper.IsSet("PLANS_NAMESPACE") && c.enableQuotas {
			return fmt.Errorf("PLANS_NAMEPSACE env variable needs to be set for quota support")
		}

		err := setupWebhooks(mgr, c.enableQuotas)
		if err != nil {
			return err
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return err
	}

	return mgr.Start(ctrl.SetupSignalHandler())
}

func setupWebhooks(mgr manager.Manager, withQuota bool) error {
	err := webhooks.SetupPostgreSQLWebhookHandlerWithManager(mgr, withQuota)
	if err != nil {
		return err
	}
	err = webhooks.SetupRedisWebhookHandlerWithManager(mgr, withQuota)
	if err != nil {
		return err
	}
	err = webhooks.SetupMariaDBWebhookHandlerWithManager(mgr, withQuota)
	if err != nil {
		return err
	}
	err = webhooks.SetupMinioWebhookHandlerWithManager(mgr, withQuota)
	if err != nil {
		return err
	}
	err = webhooks.SetupNextcloudWebhookHandlerWithManager(mgr, withQuota)
	if err != nil {
		return err
	}
	err = webhooks.SetupKeycloakWebhookHandlerWithManager(mgr, withQuota)
	if err != nil {
		return err
	}
	err = webhooks.SetupNamespaceDeletionProtectionHandlerWithManager(mgr)
	if err != nil {
		return err
	}

	err = webhooks.SetupObjectbucketCDeletionProtectionHandlerWithManager(mgr)
	if err != nil {
		return err
	}

	return webhooks.SetupPVCDeletionProtectionHandlerWithManager(mgr)
}
