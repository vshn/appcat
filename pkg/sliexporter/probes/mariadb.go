package probes

import (
	"context"
	"database/sql"
	"errors"
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
func NewMariaDB(service, name, namespace, dsn, organization, serviceLevel string, ha bool) (*MariaDB, error) {
	// open connection to MariaDB
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

// // PGWithCA adds the provided CA to the rootCAs of the pgxpool.
// func MariaDBWithCA(ca []byte) func(*pgxpool.Config) error {
// 	return func(conf *pgxpool.Config) error {
// 		if conf.ConnConfig.TLSConfig == nil {
// 			conf.ConnConfig.TLSConfig = &tls.Config{
// 				RootCAs: x509.NewCertPool(),
// 			}
// 		}

// 		if conf.ConnConfig.TLSConfig.RootCAs == nil {
// 			conf.ConnConfig.TLSConfig.RootCAs = x509.NewCertPool()
// 		}

// 		if ca == nil {
// 			return errors.New("got nil CA")
// 		}

// 		if !conf.ConnConfig.TLSConfig.RootCAs.AppendCertsFromPEM(ca) {
// 			return errors.New("cannot append root CA certificates")
// 		}
// 		return nil
// 	}
// }
