package customlibsql

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../lib
#cgo LDFLAGS: -L${SRCDIR}/../../../../lib -lsql -lm -ldl
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/../../../../lib -lsql-amd64 -lm -ldl
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/../../../../lib -lsql-arm64 -lm -ldl

#include <libsql.h>
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"runtime"
	"time"
	"unsafe"
)

// Connection represents a libSQL database connection
type Connection struct {
	conn *C.libsql_connection
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

	var conn *C.libsql_connection
	result := C.libsql_open(cFilename, &conn)

	if result != 0 {
		return nil, fmt.Errorf("failed to open libsql connection: %d", int(result))
	}

	return &Connection{conn: conn}, nil
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

	var stmt *C.libsql_stmt
	result := C.libsql_prepare(c.conn, cQuery, &stmt)

	if result != 0 {
		return nil, fmt.Errorf("failed to prepare statement: %d", int(result))
	}

	return &Statement{stmt: stmt, conn: c}, nil
}

// Close closes the connection
func (c *Connection) Close() error {
	if c.conn != nil {
		C.libsql_close(c.conn)
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
	stmt *C.libsql_stmt
	conn *Connection
}

func (s *Statement) Close() error {
	if s.stmt != nil {
		C.libsql_finalize(s.stmt)
		s.stmt = nil
	}
	return nil
}

func (s *Statement) NumInput() int {
	return -1 // Unknown number of parameters
}

func (s *Statement) Exec(args []driver.Value) (driver.Result, error) {
	// Reset statement
	C.libsql_reset(s.stmt)

	// Bind parameters
	for i, arg := range args {
		var result C.int
		switch v := arg.(type) {
		case int64:
			result = C.libsql_bind_int64(s.stmt, C.int(i+1), C.longlong(v))
		case float64:
			result = C.libsql_bind_double(s.stmt, C.int(i+1), C.double(v))
		case string:
			cStr := C.CString(v)
			defer C.free(unsafe.Pointer(cStr))
			result = C.libsql_bind_text(s.stmt, C.int(i+1), cStr, C.int(len(v)))
		case []byte:
			result = C.libsql_bind_blob(s.stmt, C.int(i+1), (*C.uchar)(unsafe.Pointer(&v[0])), C.int(len(v)))
		case nil:
			result = C.libsql_bind_null(s.stmt, C.int(i+1))
		default:
			return nil, fmt.Errorf("unsupported parameter type: %T", v)
		}
		if result != 0 {
			return nil, fmt.Errorf("failed to bind parameter %d: %d", i+1, int(result))
		}
	}

	// Execute
	result := C.libsql_step(s.stmt)
	if result != 100 { // SQLITE_ROW
		if result == 101 { // SQLITE_DONE
			return &Result{stmt: s.stmt}, nil
		}
		return nil, fmt.Errorf("execution failed: %d", int(result))
	}

	return &Result{stmt: s.stmt}, nil
}

func (s *Statement) Query(args []driver.Value) (driver.Rows, error) {
	// Reset statement
	C.libsql_reset(s.stmt)

	// Bind parameters (same as Exec)
	for i, arg := range args {
		var result C.int
		switch v := arg.(type) {
		case int64:
			result = C.libsql_bind_int64(s.stmt, C.int(i+1), C.longlong(v))
		case float64:
			result = C.libsql_bind_double(s.stmt, C.int(i+1), C.double(v))
		case string:
			cStr := C.CString(v)
			defer C.free(unsafe.Pointer(cStr))
			result = C.libsql_bind_text(s.stmt, C.int(i+1), cStr, C.int(len(v)))
		case []byte:
			result = C.libsql_bind_blob(s.stmt, C.int(i+1), (*C.uchar)(unsafe.Pointer(&v[0])), C.int(len(v)))
		case nil:
			result = C.libsql_bind_null(s.stmt, C.int(i+1))
		default:
			return nil, fmt.Errorf("unsupported parameter type: %T", v)
		}
		if result != 0 {
			return nil, fmt.Errorf("failed to bind parameter %d: %d", i+1, int(result))
		}
	}

	return &Rows{stmt: s.stmt}, nil
}

// Result implementation
type Result struct {
	stmt *C.libsql_stmt
}

func (r *Result) LastInsertId() (int64, error) {
	return int64(C.libsql_last_insert_rowid(r.stmt)), nil
}

func (r *Result) RowsAffected() (int64, error) {
	return int64(C.libsql_changes(r.stmt)), nil
}

// Rows implementation
type Rows struct {
	stmt    *C.libsql_stmt
	columns []string
}

func (r *Rows) Columns() []string {
	if r.columns == nil {
		// Get column count
		colCount := int(C.libsql_column_count(r.stmt))
		r.columns = make([]string, colCount)

		for i := 0; i < colCount; i++ {
			colName := C.libsql_column_name(r.stmt, C.int(i))
			r.columns[i] = C.GoString(colName)
		}
	}
	return r.columns
}

func (r *Rows) Close() error {
	return nil // Statement will be finalized by Statement.Close()
}

func (r *Rows) Next(dest []driver.Value) error {
	result := C.libsql_step(r.stmt)

	if result == 101 { // SQLITE_DONE
		return fmt.Errorf("no more rows")
	}

	if result != 100 { // SQLITE_ROW
		return fmt.Errorf("step failed: %d", int(result))
	}

	colCount := len(r.Columns())
	for i := 0; i < colCount; i++ {
		colType := C.libsql_column_type(r.stmt, C.int(i))

		switch colType {
		case 1: // SQLITE_INTEGER
			dest[i] = int64(C.libsql_column_int64(r.stmt, C.int(i)))
		case 2: // SQLITE_FLOAT
			dest[i] = float64(C.libsql_column_double(r.stmt, C.int(i)))
		case 3: // SQLITE_TEXT
			text := C.libsql_column_text(r.stmt, C.int(i))
			dest[i] = C.GoString(text)
		case 4: // SQLITE_BLOB
			blob := C.libsql_column_blob(r.stmt, C.int(i))
			size := C.libsql_column_bytes(r.stmt, C.int(i))
			dest[i] = C.GoBytes(unsafe.Pointer(blob), size)
		case 5: // SQLITE_NULL
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

// WaitForSync is a no-op for embedded use
func (c *Connection) WaitForSync(ctx context.Context, timeout time.Duration) error {
	return nil // No sync needed for embedded
}
