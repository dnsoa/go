module test

go 1.24.0

require (
	github.com/dnsoa/go/assert v0.0.0-00010101000000-000000000000
	github.com/dnsoa/go/sqldb v0.0.0-00010101000000-000000000000
	github.com/go-sql-driver/mysql v1.8.1
	github.com/jackc/pgx/v5 v5.7.4
	github.com/mattn/go-sqlite3 v1.14.23
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/stretchr/testify v1.9.0 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/text v0.21.0 // indirect
)

replace github.com/dnsoa/go/sqldb => ../

replace github.com/dnsoa/go/assert => ../../assert
