//go:build integration
// +build integration

package scripts

import (
	"fmt"

	customlibsql "github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/db/custom-libsql"
)

// RunCallExtDebug executes the original debug main logic.
// Call from tests or manually when needed.
func RunCallExtDebug() {
	p := "build/artifacts/artifacts/sqlean/crypto.so"
	res, msg := customlibsql.CallExtensionInit(p, "crypto_init")
	fmt.Println("crypto_init ->", res, msg)
	res2, msg2 := customlibsql.CallExtensionInit(p, "sqlite3_crypto_init")
	fmt.Println("sqlite3_crypto_init ->", res2, msg2)
}
