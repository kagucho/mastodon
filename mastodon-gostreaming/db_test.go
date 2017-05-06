package main

import (
	"database/sql"
	"fmt"
	"strconv"
	"testing"
)

func TestPQInt64Buffer(t *testing.T) {
	for _, test := range [...]struct {
		number   int
		expected string
	}{
		{0, "{}"},
		{1, "{1}"},
		{2, "{1,1}"},
	} {
		test := test

		t.Run(strconv.Itoa(test.number), func(t *testing.T) {
			buffer := newPQInt64Buffer(test.number)

			for count := 0; count < test.number; count++ {
				buffer.write(1)
			}

			if finalized := string(buffer.finalize()); finalized != test.expected {
				t.Error("expected ", test.expected, ", got ", finalized)
			}
		})
	}
}

func execAffecting1Row(db *sql.DB, query string, args ...interface{}) error {
	oauth, oauthErr := db.Exec(query, args...)
	if oauthErr != nil {
		return oauthErr
	}

	affected, affectedErr := oauth.RowsAffected()
	if affectedErr != nil {
		return affectedErr
	}

	if affected != 1 {
		return fmt.Errorf("expected to affect 1 row, affected %v", affected)
	}

	return nil
}

func openTestDB(t *testing.T) (*sql.DB, int64) {
	db, dbErr := openDB("production")
	if dbErr != nil {
		t.Fatal(dbErr)
	}

	if err := execAffecting1Row(db, "INSERT INTO accounts (domain, created_at, updated_at) VALUES ('mastodon-gostreaming-test-account-domain', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)"); err != nil {
		if err := db.Close(); err != nil {
			t.Error(err)
		}

		t.Fatal(err)
	}

	var id int64
	if err := db.QueryRow("SELECT LastVal()").Scan(&id); err != nil {
		if err := execAffecting1Row(db, "DELETE FROM accounts WHERE username = '' AND domain = 'mastodon-gostreaming-test-account-domain'"); err != nil {
			t.Error(err)
		}

		if err := db.Close(); err != nil {
			t.Error(err)
		}

		t.Fatal(err)
	}

	err := execAffecting1Row(db, "INSERT INTO users (account_id, created_at, updated_at) VALUES ($1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)", id)
	if err != nil {
		if err := execAffecting1Row(db, "DELETE FROM accounts WHERE id = $1", id); err != nil {
			t.Error(err)
		}

		if err := db.Close(); err != nil {
			t.Error(err)
		}

		t.Fatal(err)
	}

	if err := execAffecting1Row(db, "INSERT INTO oauth_access_tokens (created_at, resource_owner_id, token) SELECT CURRENT_TIMESTAMP, LastVal(), 'mastodon-gostreaming-test-token'"); err != nil {
		if err := execAffecting1Row(db, "DELETE FROM users USING accounts WHERE users.account_id = accounts.id AND accounts.id = $1", id); err != nil {
			t.Error(err)
		}

		if err := execAffecting1Row(db, "DELETE FROM accounts WHERE id = $1", id); err != nil {
			t.Error(err)
		}

		if err := db.Close(); err != nil {
			t.Error(err)
		}

		t.Fatal(err)
	}

	return db, id
}

func closeTestDB(t *testing.T, db *sql.DB) {
	if err := execAffecting1Row(db, "DELETE FROM oauth_access_tokens WHERE token = 'mastodon-gostreaming-test-token'"); err != nil {
		t.Error(err)
	}

	if err := execAffecting1Row(db, "DELETE FROM users USING accounts WHERE users.account_id = accounts.id AND accounts.username = '' AND domain = 'mastodon-gostreaming-test-account-domain'"); err != nil {
		t.Error(err)
	}

	if err := execAffecting1Row(db, "DELETE FROM accounts WHERE username = '' AND domain = 'mastodon-gostreaming-test-account-domain'"); err != nil {
		t.Error(err)
	}

	if err := db.Close(); err != nil {
		t.Error(err)
	}
}
