package telemetry

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	mssql "github.com/microsoft/go-mssqldb"

	"github.com/jorgerojas26/lazysql/drivers"
)

func TestCanonicalEngines(t *testing.T) {
	for _, provider := range []string{drivers.DriverPostgres, drivers.DriverMySQL, drivers.DriverSqlite, drivers.DriverMSSQL, drivers.DriverClickHouse} {
		if Engine(provider) != provider {
			t.Errorf("canonical provider %q lost", provider)
		}
	}
	for _, provider := range []string{"", "sqlite", "oracle", "private-db", "mysql://user:secret@host/database"} {
		if Engine(provider) != "other" {
			t.Errorf("unbounded provider %q", provider)
		}
	}
}

func TestConnectionFailureTypesOnly(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want FailureCategory
	}{
		{context.DeadlineExceeded, Timeout},
		{&net.DNSError{IsTimeout: true, Name: "private-host"}, Timeout},
		{&net.OpError{Op: "dial", Err: errors.New("private-host")}, Network},
		{&net.DNSError{Name: "private-host"}, Network},
		{x509.UnknownAuthorityError{}, TLS},
		{x509.CertificateInvalidError{}, TLS},
		{x509.HostnameError{}, TLS},
		{&tls.CertificateVerificationError{Err: errors.New("private certificate")}, TLS},
		{tls.RecordHeaderError{Msg: "private tls"}, TLS},
		{&pq.Error{Code: "28P01", Message: "private username"}, Auth},
		{&mysql.MySQLError{Number: 1045, Message: "secret username"}, Auth},
		{mssql.Error{Number: 18456, Message: "secret username"}, Auth},
		{driver.ErrBadConn, Driver},
		{errors.New("authentication failed: password=secret; timeout; TLS; dial refused"), Unknown},
		{&pq.Error{Code: "42P01", Message: "unknown table name"}, Unknown},
		{nil, Unknown},
	} {
		if got := ConnectionFailure(tc.err); got != tc.want {
			t.Errorf("%T = %s, want %s", tc.err, got, tc.want)
		}
		if tc.err != nil && ConnectionFailure(fmt.Errorf("wrapped sensitive context: %w", tc.err)) != tc.want {
			t.Errorf("wrapped %T changed category", tc.err)
		}
	}
}

func TestDistributionChannel(t *testing.T) {
	for _, tc := range []struct {
		build, path string
		want        Distribution
	}{
		{"release", "/opt/homebrew/Cellar/lazysql/0.5.8/bin/lazysql", Homebrew},
		{"release", "/usr/local/Cellar/lazysql/0.5.8/bin/lazysql", Homebrew},
		{"release", "/home/linuxbrew/.linuxbrew/Cellar/lazysql/0.5.8/bin/lazysql", Homebrew},
		{"release", "/opt/homebrew/bin/lazysql", ReleaseBuild},
		{"release", "/usr/local/bin/lazysql", ReleaseBuild},
		{"release", "/tmp/Cellar/not-lazysql/0.5.8/bin/lazysql", ReleaseBuild},
		{"release", "", ReleaseBuild},
		{"source", "/usr/local/bin/lazysql", Source},
		{"unknown", "/usr/local/bin/lazysql", UnknownDistribution},
		{"personal-build", "/opt/homebrew/Cellar/lazysql/0.5.8/bin/lazysql", UnknownDistribution},
	} {
		if got := distributionChannel(tc.build, tc.path); got != tc.want {
			t.Errorf("%q %q = %s, want %s", tc.build, tc.path, got, tc.want)
		}
	}
}
