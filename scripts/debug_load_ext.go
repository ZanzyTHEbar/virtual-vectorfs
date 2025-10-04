//go:build integration
// +build integration

package scripts

import (
	"fmt"
	"log"

	customlibsql "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/db/custom-libsql"
)

// RunDebugLoadExt loads the provided SO into a temporary DB and prints results.
func RunDebugLoadExt(so string) {
	rc, msg, err := customlibsql.DebugLoadExtension("./smoke.db", so, "")
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	fmt.Println("rc=", rc, "msg=", msg)
}
