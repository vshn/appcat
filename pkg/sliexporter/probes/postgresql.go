package probes

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var _ Prober = PostgreSQL{}

// PostgreSQL is a prober to test the uptime of a PostgreSQL instance.
type PostgreSQL struct {
	db *pgxpool.Pool

	Service           string
	Name              string
	ClaimNamespace    string
	InstanceNamespace string
	Organization      string
	HighAvailable     bool
	TLSEnabled        bool
	ServiceLevel      string
	CompositionName   string
}

// Close closes open connections to the PostgreSQL server.
func (p PostgreSQL) Close() error {
	if p.db != nil {
		p.db.Close()
	}
	return nil
}

// GetInfo returns the prober infos
func (p PostgreSQL) GetInfo() ProbeInfo {
	return ProbeInfo{
		Service:           p.Service,
		Name:              p.Name,
		ClaimNamespace:    p.ClaimNamespace,
		InstanceNamespace: p.InstanceNamespace,
		Organization:      p.Organization,
		HighAvailable:     p.HighAvailable,
		ServiceLevel:      p.ServiceLevel,
		CompositionName:   p.CompositionName,
	}
}

// Probe sends a test query to the configured PostgreSQL server.
// Will return an error if the prober does not have a valid db connection.
func (p PostgreSQL) Probe(ctx context.Context) error {
	if p.db == nil {
		return errors.New("invalid credentials")
	}
	_, err := p.db.Exec(ctx, "SELECT 1")
	return err
}

// NewPostgreSQL connects to the provided dsn and returns a prober
func NewPostgreSQL(service, name, claimNamespace, instanceNamespace, dsn, organization, sla, compositionName string, ha bool, ops ...func(*pgxpool.Config) error) (*PostgreSQL, error) {
	conf, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	conf.ConnConfig.ConnectTimeout = 5 * time.Second
	conf.MaxConns = 1

	for _, op := range ops {
		err := op(conf)
		if err != nil {
			return nil, err
		}
	}

	db, err := pgxpool.NewWithConfig(context.Background(), conf)
	if err != nil {
		return nil, err
	}

	return &PostgreSQL{
		db:                db,
		Service:           service,
		Name:              name,
		ClaimNamespace:    claimNamespace,
		InstanceNamespace: instanceNamespace,
		Organization:      organization,
		HighAvailable:     ha,
		ServiceLevel:      sla,
		CompositionName:   compositionName,
	}, nil
}

// NewFailingPostgreSQL creates a prober that will fail.
// Can be used if the controller can't access valid credentials.
func NewFailingPostgreSQL(service, name, claimNamespace, instanceNamespace string) (*PostgreSQL, error) {
	return &PostgreSQL{
		Service:           service,
		Name:              name,
		ClaimNamespace:    claimNamespace,
		InstanceNamespace: instanceNamespace,
	}, nil
}

// PGWithCA adds the provided CA to the rootCAs of the pgxpool.
func PGWithCA(ca []byte, tlsEnabled bool) func(*pgxpool.Config) error {
	if !tlsEnabled {
		return func(conf *pgxpool.Config) error {
			return nil
		}
	}

	return func(conf *pgxpool.Config) error {
		if conf.ConnConfig.TLSConfig == nil {
			conf.ConnConfig.TLSConfig = &tls.Config{
				RootCAs: x509.NewCertPool(),
			}
		}

		if conf.ConnConfig.TLSConfig.RootCAs == nil {
			conf.ConnConfig.TLSConfig.RootCAs = x509.NewCertPool()
		}

		if ca == nil {
			return errors.New("got nil CA")
		}

		if !conf.ConnConfig.TLSConfig.RootCAs.AppendCertsFromPEM(ca) {
			return errors.New("cannot append root CA certificates")
		}

		return nil
	}
}
