package customlibsql

/*
#cgo CFLAGS: -I${SRCDIR}/../../../build/artifacts/artifacts
#cgo linux,amd64 LDFLAGS: -L${SRCDIR}/../../../build/artifacts/artifacts -lsql-amd64 -lm -ldl
#cgo linux,arm64 LDFLAGS: -L${SRCDIR}/../../../build/artifacts/artifacts -lsql-arm64 -lm -ldl

#include "libsql.h"
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
	C.libsql_reset_stmt(s.stmt)

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

	return &Result{stmt: s.stmt}, nil
}

func (s *Statement) Query(args []driver.Value) (driver.Rows, error) {
	// Reset statement
	C.libsql_reset_stmt(s.stmt)

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

// WaitForSync is a no-op for embedded use
func (c *Connection) WaitForSync(ctx context.Context, timeout time.Duration) error {
	return nil // No sync needed for embedded
}
