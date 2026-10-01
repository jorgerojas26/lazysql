package telemetry

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql/driver"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	mssql "github.com/microsoft/go-mssqldb"
)

type Feature string

const (
	QueryExecute          Feature = "query_execute"
	QueryHistory          Feature = "query_history"
	ForeignKeyJump        Feature = "foreign_key_jump"
	ReverseForeignKeyJump Feature = "reverse_foreign_key_jump"
	JSONViewer            Feature = "json_viewer"
	CSVExport             Feature = "csv_export"
	ExternalEditor        Feature = "external_editor"
	RowInsert             Feature = "row_insert"
	RowUpdate             Feature = "row_update"
	RowDelete             Feature = "row_delete"
	MaxFeatureCount               = 10000
)

func validFeature(feature Feature) bool {
	switch feature {
	case QueryExecute, QueryHistory, ForeignKeyJump, ReverseForeignKeyJump, JSONViewer,
		CSVExport, ExternalEditor, RowInsert, RowUpdate, RowDelete:
		return true
	}
	return false
}

// Engine accepts only canonical LazySQL provider names, never a URL or other
// connection metadata. Unknown providers collapse to one bounded bucket.
func Engine(provider string) string {
	switch provider {
	case "postgres", "mysql", "sqlite3", "sqlserver", "clickhouse":
		return provider
	}
	return "other"
}

type FailureCategory string

const (
	Auth              FailureCategory = "auth"
	Network           FailureCategory = "network"
	Timeout           FailureCategory = "timeout"
	TLS               FailureCategory = "tls"
	InvalidConnection FailureCategory = "invalid_connection"
	Driver            FailureCategory = "driver"
	Unknown           FailureCategory = "unknown"
)

func boundedFailure(category FailureCategory) FailureCategory {
	switch category {
	case Auth, Network, Timeout, TLS, InvalidConnection, Driver:
		return category
	}
	return Unknown
}

// ConnectionFailure inspects types/codes only, never error strings or hashes.
// Call only at connection/setup boundaries, not for query or application errors.
func ConnectionFailure(err error) FailureCategory {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return Timeout
	}
	var certErr *tls.CertificateVerificationError
	var unknownCA x509.UnknownAuthorityError
	var invalidCert x509.CertificateInvalidError
	var hostname x509.HostnameError
	var record tls.RecordHeaderError
	if errors.As(err, &certErr) || errors.As(err, &unknownCA) || errors.As(err, &invalidCert) || errors.As(err, &hostname) || errors.As(err, &record) {
		return TLS
	}
	var pgErr *pq.Error
	var mysqlErr *mysql.MySQLError
	var mssqlErr mssql.Error
	if (errors.As(err, &pgErr) && pgErr.Code.Class() == "28") ||
		(errors.As(err, &mysqlErr) && (mysqlErr.Number == 1045 || mysqlErr.Number == 1698)) ||
		(errors.As(err, &mssqlErr) && (mssqlErr.Number == 18456 || mssqlErr.Number == 18452)) {
		return Auth
	}
	var opErr *net.OpError
	var dnsErr *net.DNSError
	if errors.As(err, &opErr) || errors.As(err, &dnsErr) {
		return Network
	}
	if errors.Is(err, driver.ErrBadConn) {
		return Driver
	}
	return Unknown
}

type StartupMode string

const (
	Picker        StartupMode = "picker"
	ConnectionArg StartupMode = "connection_arg"
)

type Distribution string

const (
	Homebrew            Distribution = "homebrew"
	ReleaseBuild        Distribution = "release"
	Source              Distribution = "source"
	UnknownDistribution Distribution = "unknown"
)

// DistributionChannel reads no device metadata. GoReleaser marks its binaries
// as release builds; a resolved Homebrew formula path is the only local check.
func DistributionChannel(build string) Distribution {
	if build != "release" {
		return distributionChannel(build, "")
	}
	executable, err := os.Executable()
	if err == nil {
		executable, err = filepath.EvalSymlinks(executable)
	}
	if err != nil {
		executable = ""
	}
	return distributionChannel(build, executable)
}

func distributionChannel(build, executable string) Distribution {
	if build == "release" {
		parts := strings.Split(filepath.ToSlash(filepath.Clean(executable)), "/")
		if len(parts) >= 5 {
			tail := parts[len(parts)-5:]
			if tail[0] == "Cellar" && tail[1] == "lazysql" && tail[2] != "" && tail[3] == "bin" && tail[4] == "lazysql" {
				return Homebrew
			}
		}
		return ReleaseBuild
	}
	if build == "source" {
		return Source
	}
	return UnknownDistribution
}
