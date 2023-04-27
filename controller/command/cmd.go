package command

import (
	xkube "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/urfave/cli/v2"
	"github.com/vshn/appcat-apiserver/controller/postgres"
	vshnv1 "github.com/vshn/component-appcat/apis/vshn/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

type controllerCommand struct {
	metricsAddr, healthAddr string
	leaderElect             bool
}

func init() {
	_ = corev1.SchemeBuilder.AddToScheme(s)
	_ = xkube.SchemeBuilder.AddToScheme(s)
	_ = vshnv1.SchemeBuilder.SchemeBuilder.AddToScheme(s)
}

var s = runtime.NewScheme()

func Command() *cli.Command {
	cmd := &controllerCommand{}
	return &cli.Command{
		Name:   "controller",
		Usage:  "A controller to manage PostgreSQL instance deletion",
		Action: cmd.execute,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "metrics-addr",
				Value:       ":8080",
				Usage:       "The address the metric endpoint binds to.",
				Destination: &cmd.metricsAddr,
			},
			&cli.StringFlag{
				Name:        "health-addr",
				Value:       ":8081",
				Usage:       "The address the probe endpoint binds to.",
				Destination: &cmd.healthAddr,
			},
			&cli.BoolFlag{
				Name:  "leader-elect",
				Value: false,
				Usage: "Enable leader election for controller manager. " +
					"Enabling this will ensure there is only one active controller manager.",
				Destination: &cmd.leaderElect,
			},
		},
	}
}

// Run will run the controller mode of the composition function runner.
func (c *controllerCommand) execute(cli *cli.Context) error {

	log := logr.FromContextOrDiscard(cli.Context)

	ctrl.SetLogger(log)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 s,
		MetricsBindAddress:     c.metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: c.healthAddr,
		LeaderElection:         c.leaderElect,
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
