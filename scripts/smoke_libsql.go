//go:build integration
// +build integration

package scripts

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/db"
)

func must(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %v", msg, err)
	}
}

// RunSmokeLibSQL executes the smoke checks that were previously in main.
func RunSmokeLibSQL() {
	fmt.Println("Smoke test: LibSQL embedded features")
	tmp := "./smoke.db"
	defer os.Remove(tmp)

	dbconn, err := db.ConnectToDB(tmp)
	must(err, "connect")
	defer dbconn.Close()

	// Basic
	var v int
	err = dbconn.QueryRow("SELECT 1").Scan(&v)
	must(err, "basic SELECT")
	if v != 1 {
		log.Fatalf("basic SELECT returned %v", v)
	}
	fmt.Println("OK: basic SQL")

	// FTS5
	if _, err := dbconn.Exec("CREATE VIRTUAL TABLE IF NOT EXISTS temp._fts5_smoke USING fts5(content)"); err != nil {
		log.Fatalf("FTS5 not available: %v", err)
	}
	fmt.Println("OK: FTS5 create")
	_, _ = dbconn.Exec("DROP TABLE IF EXISTS temp._fts5_smoke")

	// JSON1
	var jsonRes string
	err = dbconn.QueryRow("SELECT json_extract('{\"test\":\"value\"}', '$.test')").Scan(&jsonRes)
	must(err, "JSON1 query")
	if jsonRes != "value" {
		log.Fatalf("JSON1 returned unexpected: %v", jsonRes)
	}
	fmt.Println("OK: JSON1")

	// R*Tree
	if _, err := dbconn.Exec("CREATE VIRTUAL TABLE IF NOT EXISTS temp._rtree_smoke USING rtree(id, minX, maxX, minY, maxY)"); err != nil {
		log.Fatalf("R*Tree not available: %v", err)
	}
	fmt.Println("OK: R*Tree create")
	_, _ = dbconn.Exec("DROP TABLE IF EXISTS temp._rtree_smoke")

	// Vector (native)
	var vtype string
	err = dbconn.QueryRow("SELECT typeof(vector32('[1,2,3]'))").Scan(&vtype)
	if err != nil {
		log.Fatalf("vector32 not available: %v", err)
	}
	fmt.Printf("OK: vector32 typeof=%s\n", vtype)

	// SQLean Math
	var sq float64
	err = dbconn.QueryRow("SELECT sqrt(16)").Scan(&sq)
	must(err, "SQLean math sqrt")
	if sq != 4 {
		log.Fatalf("SQLean math returned %v", sq)
	}
	fmt.Println("OK: SQLean math sqrt")

	// SQLean Stats (median) - tolerant: non-fatal if signature differs
	var med float64
	err = dbconn.QueryRow("SELECT median(1,2,3)").Scan(&med)
	if err != nil {
		log.Printf("WARN: SQLean stats median: failed to execute: %v", err)
	} else {
		fmt.Println("OK: SQLean stats median ->", med)
	}

	// SQLean Text
	var txt string
	err = dbconn.QueryRow("SELECT concat_ws(' ', 'a','b')").Scan(&txt)
	must(err, "SQLean text concat_ws")
	if txt != "a b" {
		log.Fatalf("SQLean text returned %v", txt)
	}
	fmt.Println("OK: SQLean text concat_ws")

	// SQLean Fuzzy (optional)
	var dist int
	err = dbconn.QueryRow("SELECT damerau_levenshtein('test','tset')").Scan(&dist)
	if err != nil {
		log.Printf("WARN: SQLean fuzzy damerau_levenshtein missing or failed: %v", err)
	} else {
		fmt.Println("OK: SQLean fuzzy distance ->", dist)
	}

	// SQLean Crypto (optional)
	var sha string
	err = dbconn.QueryRow("SELECT sha256('test')").Scan(&sha)
	if err != nil {
		log.Printf("WARN: SQLean crypto sha256 missing or failed: %v", err)
	} else {
		fmt.Println("OK: SQLean crypto sha256 ->", sha)
	}

	fmt.Println("Smoke checks completed (required features must pass).")
	// wait a tick to flush logs in some environments
	time.Sleep(100 * time.Millisecond)
}
