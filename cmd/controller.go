package cmd

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vshn/appcat/v4/pkg"
	"github.com/vshn/appcat/v4/pkg/controller/billing"
	"github.com/vshn/appcat/v4/pkg/controller/events"
	"github.com/vshn/appcat/v4/pkg/controller/webhooks"
	"github.com/vshn/appcat/v4/pkg/odoo"
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
	enableAppcatWebhooks    bool
	enableProviderWebhooks  bool
	enableQuotas            bool
	enableEventForwarding   bool
	enableBilling           bool
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
	ControllerCMD.Flags().BoolVar(&c.enableAppcatWebhooks, "appcat-webhooks", true, "Disable the appcat validation webhooks")
	ControllerCMD.Flags().BoolVar(&c.enableProviderWebhooks, "provider-webhooks", true, "Disable the provider validation webhooks")
	ControllerCMD.Flags().StringVar(&c.certDir, "certdir", "/etc/webhook/certs", "Set the webhook certificate directory")
	ControllerCMD.Flags().BoolVar(&c.enableQuotas, "quotas", false, "Enable the quota webhooks, is only active if webhooks is also true")
	ControllerCMD.Flags().BoolVar(&c.enableEventForwarding, "event-forwarding", true, "Disable event-forwarding")
	ControllerCMD.Flags().BoolVar(&c.enableBilling, "billing", false, "Disable billing")
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

	if c.enableBilling {
		opts := odoo.Options{
			BaseURL:      viper.GetString("ODOO_BASE_URL"),
			DB:           viper.GetString("ODOO_DB"),
			ClientID:     viper.GetString("ODOO_CLIENT_ID"),
			ClientSecret: viper.GetString("ODOO_CLIENT_SECRET"),
			TokenURL:     viper.GetString("ODOO_TOKEN_URL"),
		}

		if opts.BaseURL == "" || opts.DB == "" || opts.ClientID == "" || opts.ClientSecret == "" || opts.TokenURL == "" {
			return fmt.Errorf("billing is enabled but Odoo configuration is incomplete")
		}

		odooClient, err := odoo.NewClient(cmd.Context(), opts)
		if err != nil {
			return fmt.Errorf("initialize Odoo client: %w", err)
		}

		b := billing.New(mgr.GetClient(), mgr.GetScheme(), odooClient)
		if err := b.SetupWithManager(mgr); err != nil {
			return err
		}
	}

	if c.enableEventForwarding {
		events := &events.EventHandler{
			Client: mgr.GetClient(),
			Scheme: mgr.GetScheme(),
		}

		err = events.SetupWithManager(mgr)

		if err != nil {
			return err
		}
	}

	if c.enableWebhooks {

		if !viper.IsSet("PLANS_NAMESPACE") && c.enableQuotas {
			return fmt.Errorf("PLANS_NAMESPACE env variable needs to be set for quota support")
		}

		err := setupWebhooks(mgr, c.enableQuotas, c.enableAppcatWebhooks, c.enableProviderWebhooks)
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

func setupWebhooks(mgr manager.Manager, withQuota bool, withAppcatWebhooks bool, withProviderWebhooks bool) error {
	if withAppcatWebhooks {
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
		err = webhooks.SetupForgejoWebhookHandlerWithManager(mgr, withQuota)
		if err != nil {
			return err
		}
		err = webhooks.SetupCodeyInstanceWebhookHandlerWithManager(mgr, withQuota)
		if err != nil {
			return err
		}
		err = webhooks.SetupXObjectbucketCDeletionProtectionHandlerWithManager(mgr)
		if err != nil {
			return err
		}

		err = webhooks.SetupObjectbucketDeletionProtectionHandlerWithManager(mgr)
		if err != nil {
			return err
		}
	}

	if withProviderWebhooks {
		err := webhooks.SetupReleaseDeletionProtectionHandlerWithManager(mgr)
		if err != nil {
			return err
		}
		err = webhooks.SetupMysqlDatabaseDeletionProtectionHandlerWithManager(mgr)
		if err != nil {
			return err
		}
		err = webhooks.SetupMysqlGrantDeletionProtectionHandlerWithManager(mgr)
		if err != nil {
			return err
		}
		err = webhooks.SetupMysqlUserDeletionProtectionHandlerWithManager(mgr)
		if err != nil {
			return err
		}
		err = webhooks.SetupObjectDeletionProtectionHandlerWithManager(mgr)
		if err != nil {
			return err
		}

		err = webhooks.SetupObjectv1alpha1DeletionProtectionHandlerWithManager(mgr)
		if err != nil {
			return err
		}
	}
	err := webhooks.SetupNamespaceDeletionProtectionHandlerWithManager(mgr)
	if err != nil {
		return err
	}
	return webhooks.SetupPVCDeletionProtectionHandlerWithManager(mgr)
}
