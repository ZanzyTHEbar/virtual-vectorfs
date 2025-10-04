package customlibsql

/*
#cgo CFLAGS: -I${SRCDIR}/../../../build/artifacts/artifacts
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/../../../build/artifacts/artifacts -lsql-amd64 -lm -ldl
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/../../../build/artifacts/artifacts -lsql-arm64 -lm -ldl

#include "libsql.h"
#include <stdlib.h>
#include <dlfcn.h>

// Attempt to dlopen the module and call its init entry point.
// Returns 0 on success, non-zero on failure. out_err_msg is malloc'd by dlerror() or a static string.
static int call_extension_init(const char* path, const char* init_name, const char** out_err_msg) {
    void* h = dlopen(path, RTLD_NOW | RTLD_GLOBAL);
    if (!h) {
        const char* e = dlerror();
        if (e) {
            *out_err_msg = e;
        } else {
            *out_err_msg = "dlopen failed";
        }
        return 1;
    }
    void* sym = dlsym(h, init_name);
    if (!sym) {
        const char* e = dlerror();
        if (e) {
            *out_err_msg = e;
        } else {
            *out_err_msg = "dlsym failed";
        }
        dlclose(h);
        return 2;
    }
    // assume init has signature int (*)(void*) or int (*)(sqlite3*). Call with NULL.
    int (*initf)(void*) = (int (*)(void*))sym;
    int r = initf(NULL);
    if (r != 0) {
        *out_err_msg = "init returned non-zero";
        // keep handle open to keep symbols alive
        return 3;
    }
    return 0;
}
*/
import "C"

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
	"unsafe"
)

// Connection represents a libSQL database connection
type Connection struct {
	conn C.libsql_connection_t
}

// Open creates a new libSQL database connection
func Open(dsn string) (*sql.DB, error) {
	connector := &Connector{dsn: dsn}
	return sql.OpenDB(connector), nil
}

// Connector implements database/sql/driver.Connector
type Connector struct {
	dsn string
}

// Connect creates a new database connection
func (c *Connector) Connect(ctx context.Context) (driver.Conn, error) {
	// Extract filename from DSN (basic parsing for file: DSN)
	dsn := c.dsn
	var filename string

	if len(dsn) > 5 && dsn[:5] == "file:" {
		filename = dsn[5:]
	} else {
		filename = dsn
	}

	// Convert to C string
	cFilename := C.CString(filename)
	defer C.free(unsafe.Pointer(cFilename))

	var db C.libsql_database_t
	var outErr *C.char
	rc := C.libsql_open_file(cFilename, &db, (**C.char)(unsafe.Pointer(&outErr)))
	if rc != 0 {
		if outErr != nil {
			return nil, fmt.Errorf("failed to open libsql database: %s", C.GoString(outErr))
		}
		return nil, fmt.Errorf("failed to open libsql database: code %d", int(rc))
	}

	var conn C.libsql_connection_t
	rc = C.libsql_connect(db, &conn, (**C.char)(unsafe.Pointer(&outErr)))
	if rc != 0 {
		if outErr != nil {
			return nil, fmt.Errorf("failed to connect libsql: %s", C.GoString(outErr))
		}
		return nil, fmt.Errorf("failed to connect libsql: code %d", int(rc))
	}

	// Attempt to load SQLean shared modules at runtime as a fallback if present.
	// Controlled by env LIBSQL_ENABLE_RUNTIME_SQLEAN (default: true when artifacts dir exists).
	func() {
		// discover dir
		sqdir := os.Getenv("LIBSQL_SQLEAN_DIR")
		if sqdir == "" {
			sqdir = "./build/artifacts/artifacts/sqlean"
		}
		enable := os.Getenv("LIBSQL_ENABLE_RUNTIME_SQLEAN")
		if enable == "" {
			if _, err := os.Stat(sqdir); err == nil {
				enable = "1"
			}
		}
		if enable == "1" || enable == "true" {
			files, err := os.ReadDir(sqdir)
			if err != nil {
				// no sqlean dir
				return
			}
			for _, f := range files {
				if f.IsDir() {
					continue
				}
				if filepath.Ext(f.Name()) != ".so" && filepath.Ext(f.Name()) != ".dll" && filepath.Ext(f.Name()) != ".dylib" {
					continue
				}
				p := filepath.Join(sqdir, f.Name())
				cpath := C.CString(p)
				var out *C.char

				// 1) Try libsql_load_extension with nil entrypoint (common case)
				res := C.libsql_load_extension(conn, cpath, nil, (**C.char)(unsafe.Pointer(&out)))
				if res == 0 {
					C.free(unsafe.Pointer(cpath))
					fmt.Printf("Loaded sqlean via libsql_load_extension: %s\n", p)
					continue
				}
				// if we have an error message from libsql, capture it (non-fatal for fallback)
				var libErr string
				if out != nil {
					libErr = C.GoString(out)
				}

				// 2) Try candidate entry points with libsql_load_extension
				candidates := []string{"sqlite3_extension_init", "sqlean_init", fmt.Sprintf("sqlite3_%s_init", stripExt(f.Name())), fmt.Sprintf("%s_init", stripExt(f.Name()))}
				loaded := false
				for _, ep := range candidates {
					centry := C.CString(ep)
					res2 := C.libsql_load_extension(conn, cpath, centry, (**C.char)(unsafe.Pointer(&out)))
					C.free(unsafe.Pointer(centry))
					if res2 == 0 {
						C.free(unsafe.Pointer(cpath))
						fmt.Printf("Loaded sqlean via libsql_load_extension entry '%s': %s\n", ep, p)
						loaded = true
						break
					}
				}
				if loaded {
					continue
				}

				// 3) Fallback: dlopen and call init symbol directly using C.call_extension_init
				for _, ep := range candidates {
					centry := C.CString(ep)
					var cerr *C.char
					res3 := C.call_extension_init(cpath, centry, (**C.char)(unsafe.Pointer(&cerr)))
					C.free(unsafe.Pointer(centry))
					if res3 == 0 {
						C.free(unsafe.Pointer(cpath))
						fmt.Printf("Loaded sqlean via dlopen+init '%s': %s\n", ep, p)
						loaded = true
						break
					}
					if cerr != nil {
						fmt.Printf("WARN: dlopen init '%s' failed for %s: %s\n", ep, p, C.GoString(cerr))
					}
				}
				// Free cpath if not already freed
				C.free(unsafe.Pointer(cpath))
				if !loaded {
					if libErr != "" {
						fmt.Printf("WARN: failed to load sqlean %s via libsql_load_extension: %s\n", p, libErr)
					} else {
						fmt.Printf("WARN: failed to load sqlean %s: no supported entrypoint found\n", p)
					}
				}
			}
		}
	}()

	return &Connection{conn: (*C.libsql_connection)(unsafe.Pointer(conn))}, nil
}

// Driver returns the driver
func (c *Connector) Driver() driver.Driver {
	return &Driver{}
}

// Driver implements database/sql/driver.Driver
type Driver struct{}

// Open creates a new connection (required by interface)
func (d *Driver) Open(dsn string) (driver.Conn, error) {
	connector := &Connector{dsn: dsn}
	return connector.Connect(context.Background())
}

// Connection implementation

// Prepare prepares a statement
func (c *Connection) Prepare(query string) (driver.Stmt, error) {
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))

	var stmt C.libsql_stmt_t
	result := C.libsql_prepare(c.conn, cQuery, &stmt, nil)

	if result != 0 {
		return nil, fmt.Errorf("failed to prepare statement: %d", int(result))
	}

	return &Statement{stmt: stmt, conn: c}, nil
}

// Close closes the connection
func (c *Connection) Close() error {
	if c.conn != nil {
		C.libsql_disconnect(c.conn)
		c.conn = nil
	}
	return nil
}

// Begin starts a transaction
func (c *Connection) Begin() (driver.Tx, error) {
	return &Transaction{conn: c}, nil
}

// Statement implementation

type Statement struct {
	stmt C.libsql_stmt_t
	conn *Connection
}

func (s *Statement) Close() error {
	if s.stmt != nil {
		C.libsql_free_stmt(s.stmt)
		s.stmt = nil
	}
	return nil
}

func (s *Statement) NumInput() int {
	return -1 // Unknown number of parameters
}

func (s *Statement) Exec(args []driver.Value) (driver.Result, error) {
	// Reset statement
	C.libsql_reset_stmt(s.stmt, nil)

	// Bind parameters
	for i, arg := range args {
		var result C.int
		switch v := arg.(type) {
		case int64:
			result = C.libsql_bind_int(s.stmt, C.int(i+1), C.longlong(v), nil)
		case float64:
			result = C.libsql_bind_float(s.stmt, C.int(i+1), C.double(v), nil)
		case string:
			cStr := C.CString(v)
			defer C.free(unsafe.Pointer(cStr))
			result = C.libsql_bind_string(s.stmt, C.int(i+1), cStr, nil)
		case []byte:
			result = C.libsql_bind_blob(s.stmt, C.int(i+1), (*C.uchar)(unsafe.Pointer(&v[0])), C.int(len(v)), nil)
		case nil:
			result = C.libsql_bind_null(s.stmt, C.int(i+1), nil)
		default:
			return nil, fmt.Errorf("unsupported parameter type: %T", v)
		}
		if result != 0 {
			return nil, fmt.Errorf("failed to bind parameter %d: %d", i+1, int(result))
		}
	}

	// Execute
	var outErr *C.char
	res := C.libsql_execute_stmt(s.stmt, (**C.char)(unsafe.Pointer(&outErr)))
	if res != 0 {
		if outErr != nil {
			return nil, fmt.Errorf("execution failed: %s", C.GoString(outErr))
		}
		return nil, fmt.Errorf("execution failed: code %d", int(res))
	}

	return &Result{stmt: s.stmt, conn: s.conn.conn}, nil
}

func (s *Statement) Query(args []driver.Value) (driver.Rows, error) {
	// Reset statement
	C.libsql_reset_stmt(s.stmt, nil)

	// Bind parameters (same as Exec)
	for i, arg := range args {
		var result C.int
		switch v := arg.(type) {
		case int64:
			result = C.libsql_bind_int(s.stmt, C.int(i+1), C.longlong(v), nil)
		case float64:
			result = C.libsql_bind_float(s.stmt, C.int(i+1), C.double(v), nil)
		case string:
			cStr := C.CString(v)
			defer C.free(unsafe.Pointer(cStr))
			result = C.libsql_bind_string(s.stmt, C.int(i+1), cStr, nil)
		case []byte:
			result = C.libsql_bind_blob(s.stmt, C.int(i+1), (*C.uchar)(unsafe.Pointer(&v[0])), C.int(len(v)), nil)
		case nil:
			result = C.libsql_bind_null(s.stmt, C.int(i+1), nil)
		default:
			return nil, fmt.Errorf("unsupported parameter type: %T", v)
		}
		if result != 0 {
			return nil, fmt.Errorf("failed to bind parameter %d: %d", i+1, int(result))
		}
	}

	// Execute query and get rows via libsql_query_stmt
	var rows C.libsql_rows_t
	var outErr *C.char
	rc := C.libsql_query_stmt(s.stmt, &rows, (**C.char)(unsafe.Pointer(&outErr)))
	if rc != 0 {
		if outErr != nil {
			return nil, fmt.Errorf("query failed: %s", C.GoString(outErr))
		}
		return nil, fmt.Errorf("query failed: code %d", int(rc))
	}

	return &Rows{rows: rows}, nil
}

// Result implementation
type Result struct {
	stmt C.libsql_stmt_t
	conn C.libsql_connection_t
}

func (r *Result) LastInsertId() (int64, error) {
	return int64(C.libsql_last_insert_rowid(r.conn)), nil
}

func (r *Result) RowsAffected() (int64, error) {
	return int64(C.libsql_changes(r.conn)), nil
}

// Rows implementation
type Rows struct {
	rows    C.libsql_rows_t
	columns []string
}

func (r *Rows) Columns() []string {
	if r.columns == nil {
		colCount := int(C.libsql_column_count(r.rows))
		r.columns = make([]string, colCount)
		for i := 0; i < colCount; i++ {
			var name *C.char
			C.libsql_column_name(r.rows, C.int(i), &name, nil)
			r.columns[i] = C.GoString(name)
		}
	}
	return r.columns
}

func (r *Rows) Close() error {
	C.libsql_free_rows(r.rows)
	return nil
}

func (r *Rows) Next(dest []driver.Value) error {
	var outRow C.libsql_row_t
	var outErr *C.char
	rc := C.libsql_next_row(r.rows, &outRow, (**C.char)(unsafe.Pointer(&outErr)))
	if rc != 0 {
		if outErr != nil {
			return fmt.Errorf("row error: %s", C.GoString(outErr))
		}
		return fmt.Errorf("row error: code %d", int(rc))
	}

	colCount := len(r.Columns())
	for i := 0; i < colCount; i++ {
		var ctype C.int
		C.libsql_column_type(r.rows, outRow, C.int(i), &ctype, nil)
		switch int(ctype) {
		case 1:
			var v C.longlong
			C.libsql_get_int(outRow, C.int(i), &v, nil)
			dest[i] = int64(v)
		case 2:
			var fv C.double
			C.libsql_get_float(outRow, C.int(i), &fv, nil)
			dest[i] = float64(fv)
		case 3:
			var s *C.char
			C.libsql_get_string(outRow, C.int(i), &s, nil)
			dest[i] = C.GoString(s)
		case 4:
			var b C.blob
			C.libsql_get_blob(outRow, C.int(i), &b, nil)
			dest[i] = C.GoBytes(unsafe.Pointer(b.ptr), C.int(b.len))
		default:
			dest[i] = nil
		}
	}

	return nil
}

// Transaction implementation
type Transaction struct {
	conn *Connection
}

func (t *Transaction) Commit() error {
	// libSQL doesn't have explicit transaction management in this wrapper
	// Transactions are implicit
	return nil
}

func (t *Transaction) Rollback() error {
	// libSQL doesn't have explicit transaction management in this wrapper
	return nil
}

// Helper functions for compatibility

// GetLibSQLPath returns the appropriate static library path for the current architecture
func GetLibSQLPath() string {
	switch runtime.GOARCH {
	case "amd64":
		return "libsql-amd64.a"
	case "arm64":
		return "libsql-arm64.a"
	default:
		return "libsql.a"
	}
}

// NewEmbeddedConnector creates a connector for embedded use (compatibility wrapper)
func NewEmbeddedConnector(dbPath string) *Connector {
	dsn := fmt.Sprintf("file:%s", dbPath)
	return &Connector{dsn: dsn}
}

// stripExt removes file extension from filename
func stripExt(name string) string {
	if idx := len(name) - len(filepath.Ext(name)); idx > 0 {
		return name[:idx]
	}
	return name
}

// WaitForSync is a no-op for embedded use
func (c *Connection) WaitForSync(ctx context.Context, timeout time.Duration) error {
	return nil // No sync needed for embedded
}

// CallExtensionInit is a debug helper that attempts to dlopen a module and call its init symbol.
// Returns result code and optional error message.
func CallExtensionInit(path string, initName string) (int, string) {
	cpath := C.CString(path)
	centry := C.CString(initName)
	defer C.free(unsafe.Pointer(cpath))
	defer C.free(unsafe.Pointer(centry))
	var cerr *C.char
	res := C.call_extension_init(cpath, centry, (**C.char)(unsafe.Pointer(&cerr)))
	msg := ""
	if cerr != nil {
		msg = C.GoString(cerr)
	}
	return int(res), msg
}

// DebugLoadExtension opens an embedded connection and attempts to call libsql_load_extension on the provided shared object path and optional entrypoint.
// Returns rc (int), message string (if any), and error for Go-level failures.
func DebugLoadExtension(dbPath string, soPath string, entry string) (int, string, error) {
	connector := &Connector{dsn: fmt.Sprintf("file:%s", dbPath)}
	drvConn, err := connector.Connect(context.Background())
	if err != nil {
		return -1, "", fmt.Errorf("connect failed: %w", err)
	}
	defer drvConn.Close()
	conn, ok := drvConn.(*Connection)
	if !ok {
		return -1, "", fmt.Errorf("unexpected connection type")
	}

	cpath := C.CString(soPath)
	defer C.free(unsafe.Pointer(cpath))
	var centry *C.char
	if entry != "" {
		centry = C.CString(entry)
		defer C.free(unsafe.Pointer(centry))
	} else {
		centry = nil
	}
	var out *C.char
	rc := C.libsql_load_extension(conn.conn, cpath, centry, (**C.char)(unsafe.Pointer(&out)))
	msg := ""
	if out != nil {
		msg = C.GoString(out)
	}
	return int(rc), msg, nil
}
