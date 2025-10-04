//go:build integration
// +build integration

package scripts

import (
	"fmt"
	"log"
	"os"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/db"
)

// RunTryLoadExtension attempts to load several extensions into a temporary DB.
func RunTryLoadExtension() {
	path := "./smoke.db"
	defer os.Remove(path)
	dbconn, err := db.ConnectToDB(path)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer dbconn.Close()

	s := []string{
		"SELECT load_extension('build/artifacts/artifacts/sqlean/sqlean.so')",
		"SELECT load_extension('build/artifacts/artifacts/sqlean/crypto.so')",
		"SELECT load_extension('build/artifacts/artifacts/sqlean/text.so')",
	}
	for _, q := range s {
		fmt.Println("Attempting:", q)
		if _, err := dbconn.Exec(q); err != nil {
			fmt.Println("Exec error:", err)
		} else {
			fmt.Println("Exec OK")
		}
	}

	fmt.Println("Done")
}
