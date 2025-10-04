//go:build integration
// +build integration

package scripts

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/db"
)

func tryQuery(db *sql.DB, q string) error {
	var out interface{}
	err := db.QueryRow(q).Scan(&out)
	if err != nil {
		return err
	}
	fmt.Printf("OK: %s -> %v\n", q, out)
	return nil
}

// RunSqleanProbe executes probe queries against a temporary DB.
func RunSqleanProbe() {
	path := "./probe.db"
	defer os.Remove(path)
	dbconn, err := db.ConnectToDB(path)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer dbconn.Close()

	tests := []string{
		"SELECT json_extract('{\"test\":\"value\"}', '$.test')",
		"SELECT typeof(vector32('[1,2,3]'))",
		"SELECT typeof(vector_distance_cos(vector32('[1,2,3]'), vector32('[1,2,3]')))",
		// median as aggregate
		"CREATE TEMP TABLE _t(x); INSERT INTO _t(x) VALUES (1),(2),(3); SELECT median(x) FROM _t;",
		// median as varargs
		"SELECT median(1,2,3)",
		// fuzzy candidates
		"SELECT damerau_levenshtein('test','tset')",
		"SELECT levenshtein('test','tset')",
		"SELECT fuzzy_compare('test','tset')",
		// crypto
		"SELECT sha256('test')",
		"SELECT hex(sha1('test'))",
	}

	for _, q := range tests {
		fmt.Printf("--- Trying: %s\n", q)
		err := tryQuery(dbconn, q)
		if err != nil {
			fmt.Printf("ERR: %s -> %v\n", q, err)
		}
	}
}
