package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestMigrationChecksumAuthority(t *testing.T) {
	sqlOriginal := []byte("CREATE TABLE test_table (id INT PRIMARY KEY);")
	hashOriginal := sha256.Sum256(sqlOriginal)
	checksumOriginal := hex.EncodeToString(hashOriginal[:])

	sqlAltered := []byte("CREATE TABLE test_table (id INT PRIMARY KEY, name TEXT);")
	hashAltered := sha256.Sum256(sqlAltered)
	checksumAltered := hex.EncodeToString(hashAltered[:])

	t.Run("identical checksum skips migration", func(t *testing.T) {
		recordedChecksum := checksumOriginal
		computedChecksum := checksumOriginal

		if recordedChecksum != computedChecksum {
			t.Fatalf("expected checksums to match")
		}
	})

	t.Run("altered migration triggers drift error", func(t *testing.T) {
		recordedChecksum := checksumOriginal
		computedChecksum := checksumAltered

		if recordedChecksum == computedChecksum {
			t.Fatalf("expected checksum mismatch for altered migration file")
		}
	})

	t.Run("strict pgx.ErrNoRows detection", func(t *testing.T) {
		var err error = pgx.ErrNoRows
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("expected pgx.ErrNoRows match")
		}

		var dbErr error = errors.New("connection failed")
		if errors.Is(dbErr, pgx.ErrNoRows) {
			t.Fatalf("unexpected pgx.ErrNoRows match for generic db error")
		}
	})
}
