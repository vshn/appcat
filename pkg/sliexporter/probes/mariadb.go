package probes

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

var _ Prober = MariaDB{}

// MariaDB is a prober to test the uptime of a MariaDB instance.
type MariaDB struct {
	db *sql.DB

	Service       string
	Name          string
	Namespace     string
	Organization  string
	HighAvailable bool
	ServiceLevel  string
}

// Close closes open connections to the MariaDB server.
func (p MariaDB) Close() error {
	if p.db != nil {
		p.db.Close()
	}
	return nil
}

// GetInfo returns the prober infos
func (p MariaDB) GetInfo() ProbeInfo {
	return ProbeInfo{
		Service:       p.Service,
		Name:          p.Name,
		Namespace:     p.Namespace,
		Organization:  p.Organization,
		HighAvailable: p.HighAvailable,
		ServiceLevel:  p.ServiceLevel,
	}
}

// Probe sends a test query to the configured MariaDB server.
// Will return an error if the prober does not have a valid db connection.
func (p MariaDB) Probe(ctx context.Context) error {
	if p.db == nil {
		return errors.New("invalid credentials")
	}
	_, err := p.db.Exec("SELECT 1")
	return err
}

// NewMariaDB connects to the provided dsn and returns a prober
func NewMariaDB(service, name, namespace, dsn, organization, caCRT, serviceLevel string, ha, TLSEnabled bool) (*MariaDB, error) {
	// regardless of the TLS setting, ca.crt is present in connection secret, therefore, it's safe to keep self-signed cert in a pool
	rootCAs := x509.NewCertPool()
	// open connection to MariaDB
	if ok := rootCAs.AppendCertsFromPEM([]byte(caCRT)); !ok {
		return nil, fmt.Errorf("failed to append PEM")
	}

	mysql.RegisterTLSConfig(name, &tls.Config{
		RootCAs: rootCAs,
	})
	if TLSEnabled {
		// tls must be set to custom name, not into "custom" as it's not supported by the driver
		// name is unique, so collision is impossible
		dsn = dsn + "?tls=" + name
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	return &MariaDB{
		db:            db,
		Service:       service,
		Name:          name,
		Namespace:     namespace,
		Organization:  organization,
		HighAvailable: ha,
		ServiceLevel:  serviceLevel,
	}, nil
}

// NewFailingMariaDB creates a prober that will fail.
// Can be used if the controller can't access valid credentials.
func NewFailingMariaDB(service, name, namespace string) (*MariaDB, error) {
	return &MariaDB{
		Service:   service,
		Name:      name,
		Namespace: namespace,
	}, nil
}
