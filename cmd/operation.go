// Package cmd: `appcat operation` is the entry point for one-off, imperative
// operations emitted as Kubernetes Jobs by composition functions.
//
// Each subcommand performs a single ad-hoc action against a managed service
// (e.g. inserting dummy data into a CNPG cluster). The composition function is
// responsible for gating the Job's emission; the subcommand here just runs.
package cmd

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/spf13/cobra"
	"github.com/vshn/appcat/v4/pkg/comp-functions/functions/vshnpostgrescnpg"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// OperationCMD is the parent cobra command for one-off operation subcommands.
var OperationCMD = &cobra.Command{
	Use:   "operation",
	Short: "Run a one-off managed-service operation",
	Long:  "Subcommands here implement one-off imperative actions against managed services. Invoked by Jobs emitted from composition functions.",
}

var (
	insertDummyInstance  string
	insertDummyNamespace string
	insertDummyComposite string

	insertDummyCMD = &cobra.Command{
		Use:   "insert-dummy",
		Short: "Insert dummy data into a CNPG-managed PostgreSQL cluster (PoC)",
		RunE:  runInsertDummy,
	}
)

func init() {
	insertDummyCMD.Flags().StringVar(&insertDummyInstance, "instance", "", "CNPG cluster name (required)")
	insertDummyCMD.Flags().StringVar(&insertDummyNamespace, "namespace", "", "instance namespace (required)")
	insertDummyCMD.Flags().StringVar(&insertDummyComposite, "composite", "", "composite name (used to locate the connection secret)")
	_ = insertDummyCMD.MarkFlagRequired("instance")
	_ = insertDummyCMD.MarkFlagRequired("namespace")
	_ = insertDummyCMD.MarkFlagRequired("composite")

	OperationCMD.AddCommand(insertDummyCMD)
}

// runInsertDummy resolves the CNPG-managed connection secret, opens a pgx
// connection, then runs idempotent CREATE TABLE + conditional INSERT.
func runInsertDummy(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	uri, err := readConnectionURI(ctx, insertDummyNamespace, insertDummyComposite)
	if err != nil {
		return fmt.Errorf("cannot read connection secret: %w", err)
	}

	cfg, err := pgx.ParseConfig(uri)
	if err != nil {
		return fmt.Errorf("cannot parse connection uri: %w", err)
	}
	// CNPG's default app secret encodes database=* (wildcard). Replace w/ a
	// real db so pgx can issue a startup message.
	if cfg.Database == "" || cfg.Database == "*" {
		cfg.Database = "postgres"
	}
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("cannot connect to %s: %w", insertDummyInstance, err)
	}
	defer conn.Close(ctx)

	// Both statements are idempotent — CREATE IF NOT EXISTS, INSERT WHERE NOT EXISTS.
	if _, err := conn.Exec(ctx, "CREATE TABLE IF NOT EXISTS dummy (id serial PRIMARY KEY, msg text NOT NULL, created_at timestamptz DEFAULT now())"); err != nil {
		return fmt.Errorf("cannot create dummy table: %w", err)
	}
	if _, err := conn.Exec(ctx, "INSERT INTO dummy(msg) SELECT 'hello from operations PoC' WHERE NOT EXISTS (SELECT 1 FROM dummy)"); err != nil {
		return fmt.Errorf("cannot insert dummy row: %w", err)
	}

	fmt.Println("dummy data inserted into", insertDummyInstance)
	return nil
}

// readConnectionURI fetches the connection secret written by the
// vshnpostgrescnpg composition function. The secret name is `<composite>-connection`
// and lives in the instance namespace.
func readConnectionURI(ctx context.Context, namespace, composite string) (string, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return "", fmt.Errorf("cannot build in-cluster config: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return "", fmt.Errorf("cannot build kubernetes client: %w", err)
	}

	secretName := composite + "-connection"
	secret, err := cs.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("cannot get secret %s/%s: %w", namespace, secretName, err)
	}

	uri := string(secret.Data[vshnpostgrescnpg.PostgresqlURL])
	if uri == "" {
		return "", fmt.Errorf("secret %s/%s has empty %q", namespace, secretName, vshnpostgrescnpg.PostgresqlURL)
	}
	return uri, nil
}

