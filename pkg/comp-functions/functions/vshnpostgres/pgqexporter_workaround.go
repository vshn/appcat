package vshnpostgres

import (
	"context"
	"fmt"

	xkubev1 "github.com/crossplane-contrib/provider-kubernetes/apis/object/v1alpha1"
	"github.com/crossplane/function-sdk-go/proto/v1beta1"
	vshnv1 "github.com/vshn/appcat/v4/apis/vshn/v1"
	"github.com/vshn/appcat/v4/pkg/comp-functions/runtime"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func PgExporterWorkaround(ctx context.Context, svc *runtime.ServiceRuntime) *v1beta1.Result {
	log := controllerruntime.LoggerFrom(ctx)

	comp := &vshnv1.VSHNPostgreSQL{}
	err := svc.GetObservedComposite(comp)
	if err != nil {
		return runtime.NewFatalResult(fmt.Errorf("cannot get composite: %w", err))
	}

	log.Info("Making sure the cluster exposed connection details")
	obj := &xkubev1.Object{}
	err = svc.GetObservedComposedResource(obj, "cluster")
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get the sgcluster object: %s", err))
	}

	// get configmap
	configMap := &corev1.ConfigMap{}
	// create k8s client inside cluster
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	// get configmap
	configMap, err = clientset.CoreV1().ConfigMaps(comp.GetNamespace()).Get(context.Background(), comp.GetName()+"-prometheus-postgres-exporter-config", metav1.GetOptions{})
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot get configmap: %s", err))
	}

	//annotate ConfigMap
	configMap.Annotations["stackgres.io/reconciliation-pause"] = "true"
	_, err = clientset.CoreV1().ConfigMaps(comp.GetNamespace()).Update(context.Background(), configMap, metav1.UpdateOptions{})
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot annotate configmap: %s", err))
	}

	// replace Data inside ConfigMap
	configMap.Data["queries.yaml"] = queries
	_, err = clientset.CoreV1().ConfigMaps(comp.GetNamespace()).Update(context.Background(), configMap, metav1.UpdateOptions{})
	if err != nil {
		return runtime.NewWarningResult(fmt.Sprintf("cannot update configmap: %s", err))
	}

	return nil
}

const queries = `
pg_replication:
  master: true
  query: |
    SELECT
    CASE
      WHEN pg_last_wal_receive_lsn() = pg_last_wal_replay_lsn() THEN 0
      ELSE extract (EPOCH FROM now() - pg_last_xact_replay_timestamp())::integer
    END AS lag,
    CASE
      WHEN pg_is_in_recovery() THEN 1
      ELSE 0
    END AS is_replica;
  metrics:
    - lag:
        usage: "GAUGE"
        description: "Replication lag behind master in seconds"
    - is_replica:
        usage: "GAUGE"
        description: "Indicates if this host is a replica"

pg_postmaster:
  master: true
  query: "SELECT pg_postmaster_start_time as start_time_seconds from pg_postmaster_start_time()"
  metrics:
    - start_time_seconds:
        usage: "GAUGE"
        description: "Time at which postmaster started"

  master: true
  query: |
    SET max_parallel_workers_per_gather = 0;
    WITH databases AS (
      SELECT datname FROM pg_database
      WHERE datname NOT IN ('template0', 'template1')
    )
    SELECT datname, _.* FROM databases,
      LATERAL (SELECT * FROM dblink(
        'host=/var/run/postgresql port=5432 user=postgres sslmode=disable dbname=''' || regexp_replace(datname, '([.\\])', '\\\1', 'g') || '''',
        'SELECT schemaname, relname, indexrelname, idx_blks_read, idx_blks_hit FROM pg_catalog.pg_statio_user_indexes')
        AS (schemaname name, relname name, indexrelname name, idx_blks_read bigint, idx_blks_hit bigint)) AS _;
  metrics:
    - datname:
        usage: "LABEL"
        description: "Database name"
    - schemaname:
        usage: "LABEL"
        description: "Name of the schema that this table is in"
    - relname:
        usage: "LABEL"
        description: "Name of the table for this index"
    - indexrelname:
        usage: "LABEL"
        description: "Name of this index"
    - idx_blks_read:
        usage: "COUNTER"
        description: "Number of disk blocks read from this index"
    - idx_blks_hit:
        usage: "COUNTER"
        description: "Number of buffer hits in this index"

  master: true
  cache_seconds: 120
  query: |
    SET max_parallel_workers_per_gather = 0;
    WITH databases AS (
      SELECT datname FROM pg_database
      WHERE datname NOT IN ('template0', 'template1')
    )
    SELECT datname, _.* FROM databases,
      LATERAL (SELECT * FROM dblink(
        'host=/var/run/postgresql port=5432 user=postgres sslmode=disable dbname=''' || regexp_replace(datname, '([.\\])', '\\\1', 'g') || '''',
        'SELECT relnamespace::regnamespace as schemaname, relname as relname, pg_total_relation_size(oid) bytes FROM pg_catalog.pg_class WHERE relkind = ''r''')
        AS (schemaname name, relname name, bytes bigint)) AS _;
  metrics:
    - datname:
        usage: "LABEL"
        description: "Database name"
    - schemaname:
        usage: "LABEL"
        description: "Name of the schema that table is in"
    - relname:
        usage: "LABEL"
        description: "Name of this table"
    - bytes:
        usage: "GAUGE"
        description: "Total disk space usage for the specified table and associated indexes"


  master: true
  query: |
    SELECT datname, age(datfrozenxid) AS age FROM pg_catalog.pg_database;
  metrics:
    - datname:
        usage: "LABEL"
        description: "Database Name"
    - age:
        usage: "GAUGE"
        description: "Age of the oldest transaction that has not been frozen."

pg_wal_position:
  master: true
  query: |
    SELECT CASE
          WHEN pg_is_in_recovery()
          THEN (pg_last_wal_receive_lsn() - '0/0') % (2^52)::bigint
          ELSE (pg_current_wal_lsn() - '0/0') % (2^52)::bigint
          END AS bytes;
  metrics:
    - bytes:
        usage: "COUNTER"
        description: "Postgres LSN (log sequence number) being generated on primary or replayed on replica (truncated to low 52 bits)"



pgbouncer_show_clients:
  master: true
  query: |
    SELECT _.type,
    _."user",
    _.database,
    _.state,
    _.addr,
    _.port,
    _.local_addr,
    _.local_port,
    _.connect_time,
    _.request_time,
    _.wait,
    _.wait_us,
    _.close_needed,
    _.ptr,
    _.link,
    _.remote_pid,
    _.tls,
    _.application_name,
    _.prepared_statements
    FROM dblink('host=/var/run/postgresql port=6432 dbname=pgbouncer user=pgbouncer', 'show clients'::text)
    AS _(type text, "user" text, database text, state text, addr text, port integer, local_addr text, local_port integer,
    connect_time timestamp with time zone, request_time timestamp with time zone, wait integer, wait_us integer, close_needed integer,
    ptr text, link text, remote_pid integer, tls text, application_name text, prepared_statements integer);
  metrics:
    - type:
        usage: "LABEL"
        description: "C, for client."
    - user:
        usage: "LABEL"
        description: "Client connected user"
    - database:
        usage: "LABEL"
        description: "Database name"
    - state:
        usage: "LABEL"
        description: "State of the client connection, one of active or waiting"
    - addr:
        usage: "LABEL"
        description: "IP address of client"
    - port:
        usage: "GAUGE"
        description: "Port client is connected to"
    - local_addr:
        usage: "LABEL"
        description: "Connection end address on local machine"
    - local_port:
        usage: "GAUGE"
        description: "Connection end port on local machine"
    - connect_time:
        usage: "LABEL"
        description: "Timestamp of connect time"
    - request_time:
        usage: "LABEL"
        description: "Timestamp of latest client request"
    - wait:
        usage: "GAUGE"
        description: "Current waiting time in seconds"
    - wait_us:
        usage: "GAUGE"
        description: "Microsecond part of the current waiting time"
    - close_needed:
        usage: "GAUGE"
        description: "not used for clients"
    - ptr:
        usage: "LABEL"
        description: "Address of internal object for this connection. Used as unique ID"
    - link:
        usage: "LABEL"
        description: "Address of server connection the client is paired with"
    - remote_pid:
        usage: "GAUGE"
        description: "Process ID, in case client connects over Unix socket and OS supports getting it"
    - tls:
        usage: "LABEL"
        description: "A string with TLS connection information, or empty if not using TLS"
    - application_name:
        usage: "LABEL"
        description: "A string containing the application_name set by the client for this connection, or empty if this was not set"
    - prepared_statements:
        usage: "GAUGE"
        description: "The amount of prepared statements that the client has prepared"

pgbouncer_show_pools:
  master: true
  query: |
    SELECT _.database,
    _."user",
    _.cl_active,
    _.cl_waiting,
    _.cl_active_cancel_req,
    _.cl_waiting_cancel_req,
    _.sv_active,
    _.sv_active_cancel,
    _.sv_being_canceled,
    _.sv_idle,
    _.sv_used,
    _.sv_tested,
    _.sv_login,
    _.maxwait,
    _.maxwait_us,
    _.pool_mode
    FROM dblink('host=/var/run/postgresql port=6432 dbname=pgbouncer user=pgbouncer', 'show pools'::text)
    _(database text, "user" text, cl_active integer, cl_waiting integer, cl_active_cancel_req integer, cl_waiting_cancel_req integer,
    sv_active integer, sv_active_cancel integer, sv_being_canceled integer, sv_idle integer, sv_used integer,
    sv_tested integer, sv_login integer, maxwait integer, maxwait_us integer, pool_mode text);
  metrics:
    - database:
        usage: "LABEL"
        description: "Database name"
    - user:
        usage: "LABEL"
        description: "User name"
    - cl_active:
        usage: "GAUGE"
        description: "Client connections that are linked to server connection and can process queries"
    - cl_waiting:
        usage: "GAUGE"
        description: "Client connections that have sent queries but have not yet got a server connection"
    - cl_active_cancel_req:
        usage: "GAUGE"
        description: "Client connections that have forwarded query cancellations to the server and are waiting for the server response"
    - cl_waiting_cancel_req:
        usage: "GAUGE"
        description: "    Client connections that have forwarded query cancellations to the server and are waiting for the server response"
    - cl_waiting_cancel_req:
        usage: "GAUGE"
        description: "Client connections that have not forwarded query cancellations to the server yet"
    - sv_active:
        usage: "GAUGE"
        description: "Server connections that are linked to a client"
    - sv_active_cancel:
        usage: "GAUGE"
        description: "Server connections that are currently forwarding a cancel request"
    - sv_being_canceled:
        usage: "GAUGE"
        description: "Servers that normally could become idle but are waiting to do so until all in-flight cancel requests have completed that were sent to cancel a query on this server"
    - sv_idle:
        usage: "GAUGE"
        description: "Server connections that are unused and immediately usable for client queries"
    - sv_used:
        usage: "GAUGE"
        description: "Server connections that have been idle for more than server_check_delay so they need server_check_query to run on them"
    - sv_tested:
        usage: "GAUGE"
        description: "Server connections that are currently running either server_reset_query or server_check_query"
    - sv_login:
        usage: "GAUGE"
        description: "Server connections currently in the process of logging in"
    - maxwait:
        usage: "GAUGE"
        description: "How long the first oldest client in the queue has waited, in seconds"
    - maxwait_us:
        usage: "GAUGE"
        description: "Microsecond part of the maximum waiting time"
    - pool_mode:
        usage: "LABEL"
        description: "The pooling mode in use"

pgbouncer_show_databases:
  master: true
  query: |
    select _.name,
    _.host,
    _.port,
    _.database,
    _.force_user,
    _.pool_size,
    _.min_pool_size,
    _.reserve_pool,
    _.pool_mode,
    _.max_connections,
    _.current_connections,
    _.paused,
    _.disabled
    FROM dblink('host=/var/run/postgresql port=6432 dbname=pgbouncer user=pgbouncer', 'show databases'::text)
    AS _(name text, host text, port integer, database text, force_user text, pool_size integer, min_pool_size integer,
      reserve_pool integer, pool_mode text, max_connections integer, current_connections integer, paused boolean, disabled boolean);
  metrics:
    - name:
        usage: "LABEL"
        description: "Name of configured database entry"
    - host:
        usage: "LABEL"
        description: "Host pgbouncer connects to"
    - port:
        usage: "GAUGE"
        description: "Port pgbouncer connects to"
    - database:
        usage: "LABEL"
        description: "Actual database name pgbouncer connects to."
    - force_user:
        usage: "LABEL"
        description: "When the user is part of the connection string the connection between pgbouncer and PostgreSQL is forced to the given user"
    - pool_size:
        usage: "GAUGE"
        description: "Maximum number of server connections"
    - min_pool_size:
        usage: "GAUGE"
        description: "Minimum number of server connections"
    - reserve_pool:
        usage: "GAUGE"
        description: "Maximum number of additional connections for this database"
    - pool_mode:
        usage: "LABEL"
        description: "The database override pool_mode"
    - max_connections:
        usage: "GAUGE"
        description: "Maximum number of allowed connections for this database"
    - current_connections:
        usage: "GAUGE"
        description: "Current number of connections for this database"
    - paused:
        usage: "GAUGE"
        description: "1 if this database is currently paused, else 0"
    - disabled:
        usage: "GAUGE"
        description: "1 if this database is currently paused, else 0"

pgbouncer_show_stats:
  master: true
  query: |
    select _.database,
    _.total_xact_count,
    _.total_query_count,
    _.total_received,
    _.total_sent,
    _.total_xact_time,
    _.total_query_time,
    _.total_wait_time,
    _.avg_xact_count,
    _.avg_query_count,
    _.avg_recv,
    _.avg_sent,
    _.avg_xact_time,
    _.avg_query_time,
    _.avg_wait_time
    FROM dblink('host=/var/run/postgresql port=6432 dbname=pgbouncer user=pgbouncer', 'show stats'::text)
    AS _(database text, total_xact_count bigint, total_query_count bigint, total_received bigint, total_sent bigint,total_xact_time bigint, total_query_time bigint,
      total_wait_time bigint, avg_xact_count bigint, avg_query_count bigint, avg_recv bigint, avg_sent bigint, avg_xact_time bigint, avg_query_time bigint,
      avg_wait_time bigint);
  metrics:
    - database:
        usage: "LABEL"
        description: "Database name"
    - total_xact_count:
        usage: "GAUGE"
        description: "Total number of SQL transactions pooled"
    - total_query_count:
        usage: "GAUGE"
        description: "Total number of SQL queries pooled"
    - total_received:
        usage: "GAUGE"
        description: "Total volume in bytes of network traffic received"
    - total_sent:
        usage: "GAUGE"
        description: "Total volume in bytes of network traffic sent"
    - total_xact_time:
        usage: "GAUGE"
        description: "Total number of microseconds spent by pgbouncer when connected to PostgreSQL in a transaction"
    - total_query_time:
        usage: "GAUGE"
        description: "Total number of microseconds spent by pgbouncer when actively connected to PostgreSQL"
    - total_wait_time:
        usage: "GAUGE"
        description: "Time spent by clients waiting for a server, in microseconds"
    - avg_xact_count:
        usage: "GAUGE"
        description: "Average transactions per second in last stat period"
    - avg_query_count:
        usage: "GAUGE"
        description: "Average queries per second in last stat period"
    - avg_recv:
        usage: "GAUGE"
        description: "Average received from clients bytes per second"
    - avg_sent:
        usage: "GAUGE"
        description: "Average sent to clients bytes per second"
    - avg_xact_time:
        usage: "GAUGE"
        description: "Average transaction duration, in microseconds"
    - avg_query_time:
        usage: "GAUGE"
        description: "Average query duration, in microseconds"
    - avg_wait_time:
        usage: "GAUGE"
        description: "Time spent by clients waiting for a server, in microseconds average per second"

node_filesystem:
  master: true
  query: |
    WITH mounts AS (
      SELECT columns[1] AS device,
        columns[2] AS mountpoint
        FROM (SELECT regexp_split_to_array(line, E'\\s+') AS columns
            FROM mounts() AS line) AS mounts
      WHERE columns[2] LIKE '/var/%')
    SELECT CASE WHEN columns[1] <> '-' THEN columns[1] ELSE NULL END AS device,
        CASE WHEN columns[2] <> '-' THEN columns[2] ELSE NULL END AS mountpoint,
        CASE WHEN columns[3] <> '-' THEN columns[3] ELSE NULL END AS fstype,
        CASE WHEN columns[4] <> '-' THEN columns[4] ELSE NULL END AS size_bytes,
        CASE WHEN columns[5] <> '-' THEN columns[5] ELSE NULL END AS avail_bytes,
        CASE WHEN columns[6] <> '-' THEN columns[6] ELSE NULL END AS files,
        CASE WHEN columns[7] <> '-' THEN columns[7] ELSE NULL END AS files_free,
        CASE WHEN columns[8] <> '-' AND columns[8] <> 'timeout' THEN TRUE ELSE FALSE END AS device_error
      FROM (SELECT regexp_split_to_array(line, E'\\s+') AS columns
          FROM (SELECT df(mountpoint) AS line FROM mounts) AS df) AS df;
  metrics:
    - device:
        usage: "LABEL"
        description: "Device of the filesystem."
    - mountpoint:
        usage: "LABEL"
        description: "Mount point of the filesystem."
    - fstype:
        usage: "LABEL"
        description: "The type of filesystem."
    - size_bytes:
        usage: "GAUGE"
        description: "Filesystem size in bytes."
    - avail_bytes:
        usage: "GAUGE"
        description: "Filesystem space available to non-root users in bytes."
    - files:
        usage: "GAUGE"
        description: "Filesystem total file nodes."
    - files_free:
        usage: "GAUGE"
        description: "Filesystem total free file nodes."
    - device_error:
        usage: "GAUGE"
        description: "Whether an error occurred while getting statistics for the given device."

# PG_STAT_STATEMENT
pg_statements:
  query: |
    SELECT * FROM dblink(
        'host=/var/run/postgresql port=5432 user=postgres sslmode=disable dbname=postgres',
        '
        SELECT
            pg_database.datname,
            pg_roles.rolname as usename,
            pg_stat_statements.queryid,
            pg_stat_statements.calls as calls_total,
        '
        || CASE WHEN (SELECT setting FROM pg_settings WHERE name = 'server_version_num')::bigint >= 130000 THEN 'pg_stat_statements.total_exec_time' ELSE 'pg_stat_statements.total_time' END || ' / 1000 AS total_exec_time, '
        || CASE WHEN (SELECT setting FROM pg_settings WHERE name = 'server_version_num')::bigint >= 130000 THEN 'pg_stat_statements.mean_exec_time' ELSE 'pg_stat_statements.mean_time' END || ' / 1000 AS mean_exec_time, '
        || '
            pg_stat_statements.rows as rows_total
        FROM pg_stat_statements
        JOIN pg_roles ON (pg_stat_statements.userid = pg_roles.oid)
        JOIN pg_database ON (pg_stat_statements.dbid = pg_database.oid)
        ')
        AS (datname text, usename text, queryid bigint, calls_total bigint, total_exec_time double precision, mean_exec_time double precision, rows_total bigint)
        WHERE calls_total > 1
        ORDER BY total_exec_time -mean_exec_time * calls_total desc
        LIMIT 20;
  master: true
  metrics:
    - datname:
        usage: "LABEL"
        description: "Database name"
    - usename:
        usage: "LABEL"
        description: "User name"
    - queryid:
        usage: "LABEL"
        description: "Query ID"
    - calls_total:
        usage: "GAUGE"
        description: "Total calls of the query"
    - total_exec_time:
        usage: "GAUGE"
        description: "Total execute time in milliseconds"
    - mean_exec_time:
        usage: "GAUGE"
        description: "Total mean time in milliseconds"
    - rows_total:
        usage: "GAUGE"
        description: "Total rows returned"
`
