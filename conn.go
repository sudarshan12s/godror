// Copyright 2017 Tamás Gulácsi
//
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package goracle

/*
#include <stdlib.h>
#include "dpiImpl.h"
*/
import "C"

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/pkg/errors"
)

const getConnection = "--GET_CONNECTION--"
const wrapResultset = "--WRAP_RESULTSET--"

// The maximum capacity is limited to (2^32 / sizeof(dpiData))-1 to remain compatible
// with 32-bit platforms. The size of a `C.dpiData` is 32 Byte on a 64-bit system, `C.dpiSubscrMessageTable` is 40 bytes.
// So this is 2^25.
// See https://github.com/go-goracle/goracle/issues/73#issuecomment-401281714
const maxArraySize = (1<<30)/C.sizeof_dpiSubscrMessageTable - 1

var _ = driver.Conn((*conn)(nil))
var _ = driver.ConnBeginTx((*conn)(nil))
var _ = driver.ConnPrepareContext((*conn)(nil))
var _ = driver.Pinger((*conn)(nil))

type conn struct {
	sync.RWMutex
	dpiConn        *C.dpiConn
	connParams     ConnectionParams
	inTransaction  bool
	Client, Server VersionInfo
	*drv
}

func (c *conn) getError() error {
	if c == nil || c.drv == nil {
		return driver.ErrBadConn
	}
	return c.drv.getError()
}

func (c *conn) Break() error {
	c.RLock()
	defer c.RUnlock()
	if Log != nil {
		Log("msg", "Break", "dpiConn", c.dpiConn)
	}
	if C.dpiConn_breakExecution(c.dpiConn) == C.DPI_FAILURE {
		return errors.Wrap(c.getError(), "Break")
	}
	return nil
}

// Ping checks the connection's state.
//
// WARNING: as database/sql calls database/sql/driver.Open when it needs
// a new connection, but does not provide this Context,
// if the Open stalls (unreachable / firewalled host), the
// database/sql.Ping may return way after the Context.Deadline!
func (c *conn) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.RLock()
	defer c.RUnlock()
	done := make(chan error, 1)
	go func() {
		defer close(done)
		failure := C.dpiConn_ping(c.dpiConn) == C.DPI_FAILURE
		if failure {
			done <- maybeBadConn(errors.Wrap(c.getError(), "Ping"))
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		// select again to avoid race condition if both are done
		select {
		case err := <-done:
			return err
		default:
			_ = c.Break()
			return driver.ErrBadConn
		}
	}
}

// Prepare returns a prepared statement, bound to this connection.
func (c *conn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

// Close invalidates and potentially stops any current
// prepared statements and transactions, marking this
// connection as no longer in use.
//
// Because the sql package maintains a free pool of
// connections and only calls Close when there's a surplus of
// idle connections, it shouldn't be necessary for drivers to
// do their own connection caching.
func (c *conn) Close() error {
	if c == nil {
		return nil
	}
	c.Lock()
	defer c.Unlock()
	c.setTraceTag(TraceTag{})
	dpiConn := c.dpiConn
	c.dpiConn = nil
	if dpiConn == nil {
		return nil
	}
	// Just to be sure, break anything in progress.
	done := make(chan struct{})
	go func() {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			C.dpiConn_breakExecution(dpiConn)
		}
	}()
	rc := C.dpiConn_release(dpiConn)
	close(done)
	var err error
	if rc == C.DPI_FAILURE {
		err = errors.Wrap(c.getError(), "Close")
	}
	return err
}

// Begin starts and returns a new transaction.
//
// Deprecated: Drivers should implement ConnBeginTx instead (or additionally).
func (c *conn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

// BeginTx starts and returns a new transaction.
// If the context is canceled by the user the sql package will
// call Tx.Rollback before discarding and closing the connection.
//
// This must check opts.Isolation to determine if there is a set
// isolation level. If the driver does not support a non-default
// level and one is set or if there is a non-default isolation level
// that is not supported, an error must be returned.
//
// This must also check opts.ReadOnly to determine if the read-only
// value is true to either set the read-only transaction property if supported
// or return an error if it is not supported.
func (c *conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var todo []string
	if opts.ReadOnly {
		todo = append(todo, "READ ONLY")
	} else {
		todo = append(todo, "READ WRITE")
	}
	switch level := sql.IsolationLevel(opts.Isolation); level {
	case sql.LevelDefault:
	case sql.LevelReadCommitted:
		todo = append(todo, "ISOLATION LEVEL READ COMMITTED")
	case sql.LevelSerializable:
		todo = append(todo, "ISOLATION LEVEL SERIALIZABLE")
	default:
		return nil, errors.Errorf("%v isolation level is not supported", sql.IsolationLevel(opts.Isolation))
	}

	for _, qry := range todo {
		qry = "SET TRANSACTION " + qry
		stmt, err := c.PrepareContext(ctx, qry)
		if err == nil {
			//fmt.Println(qry)
			_, err = stmt.Exec(nil) //nolint:typecheck,megacheck
			stmt.Close()
		}
		if err != nil {
			return nil, errors.Wrap(err, qry)
		}
	}

	c.RLock()
	inTran := c.inTransaction
	c.RUnlock()
	if inTran {
		return nil, errors.New("already in transaction")
	}
	c.Lock()
	c.inTransaction = true
	c.Unlock()
	if tt, ok := ctx.Value(traceTagCtxKey).(TraceTag); ok {
		c.Lock()
		c.setTraceTag(tt)
		c.Unlock()
	}
	return c, nil
}

// PrepareContext returns a prepared statement, bound to this connection.
// context is for the preparation of the statement,
// it must not store the context within the statement itself.
func (c *conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if tt, ok := ctx.Value(traceTagCtxKey).(TraceTag); ok {
		c.Lock()
		c.setTraceTag(tt)
		c.Unlock()
	}
	if query == getConnection {
		if Log != nil {
			Log("msg", "PrepareContext", "shortcut", query)
		}
		return &statement{conn: c, query: query}, nil
	}

	cSQL := C.CString(query)
	defer func() {
		C.free(unsafe.Pointer(cSQL))
	}()
	c.RLock()
	defer c.RUnlock()
	var dpiStmt *C.dpiStmt
	if C.dpiConn_prepareStmt(c.dpiConn, 0, cSQL, C.uint32_t(len(query)), nil, 0,
		(**C.dpiStmt)(unsafe.Pointer(&dpiStmt)),
	) == C.DPI_FAILURE {
		return nil, maybeBadConn(errors.Wrap(c.getError(), "Prepare: "+query))
	}
	return &statement{conn: c, dpiStmt: dpiStmt, query: query}, nil
}
func (c *conn) Commit() error {
	return c.endTran(true)
}
func (c *conn) Rollback() error {
	return c.endTran(false)
}
func (c *conn) endTran(isCommit bool) error {
	c.Lock()
	c.inTransaction = false

	var err error
	//msg := "Commit"
	if isCommit {
		if C.dpiConn_commit(c.dpiConn) == C.DPI_FAILURE {
			err = errors.Wrap(c.getError(), "Commit")
		}
	} else {
		//msg = "Rollback"
		if C.dpiConn_rollback(c.dpiConn) == C.DPI_FAILURE {
			err = errors.Wrap(c.getError(), "Rollback")
		}
	}
	c.Unlock()
	//fmt.Printf("%p.%s\n", c, msg)
	return err
}

type varInfo struct {
	IsPLSArray        bool
	Typ               C.dpiOracleTypeNum
	NatTyp            C.dpiNativeTypeNum
	SliceLen, BufSize int
	ObjectType        *C.dpiObjectType
}

func (c *conn) newVar(vi varInfo) (*C.dpiVar, []C.dpiData, error) {
	if c == nil || c.dpiConn == nil {
		return nil, nil, errors.New("connection is nil")
	}
	isArray := C.int(0)
	if vi.IsPLSArray {
		isArray = 1
	}
	if vi.SliceLen < 1 {
		vi.SliceLen = 1
	}
	var dataArr *C.dpiData
	var v *C.dpiVar
	if Log != nil {
		Log("C", "dpiConn_newVar", "conn", c.dpiConn, "typ", int(vi.Typ), "natTyp", int(vi.NatTyp), "sliceLen", vi.SliceLen, "bufSize", vi.BufSize, "isArray", isArray, "objType", vi.ObjectType, "v", v)
	}
	if C.dpiConn_newVar(
		c.dpiConn, vi.Typ, vi.NatTyp, C.uint32_t(vi.SliceLen),
		C.uint32_t(vi.BufSize), 1,
		isArray, vi.ObjectType,
		&v, &dataArr,
	) == C.DPI_FAILURE {
		return nil, nil, errors.Wrapf(c.getError(), "newVar(typ=%d, natTyp=%d, sliceLen=%d, bufSize=%d)", vi.Typ, vi.NatTyp, vi.SliceLen, vi.BufSize)
	}
	// https://github.com/golang/go/wiki/cgo#Turning_C_arrays_into_Go_slices
	/*
		var theCArray *C.YourType = C.getTheArray()
		length := C.getTheArrayLength()
		slice := (*[maxArraySize]C.YourType)(unsafe.Pointer(theCArray))[:length:length]
	*/
	data := ((*[maxArraySize]C.dpiData)(unsafe.Pointer(dataArr)))[:vi.SliceLen:vi.SliceLen]
	return v, data, nil
}

var _ = driver.Tx((*conn)(nil))

func (c *conn) ServerVersion() (VersionInfo, error) {
	return c.Server, nil
}

func (c *conn) init() error {
	if c.Server.Version != 0 {
		return nil
	}
	var err error
	if c.Client, err = c.drv.ClientVersion(); err != nil {
		return err
	}
	var v C.dpiVersionInfo
	var release *C.char
	var releaseLen C.uint32_t
	if C.dpiConn_getServerVersion(c.dpiConn, &release, &releaseLen, &v) == C.DPI_FAILURE {
		return errors.Wrap(c.getError(), "getServerVersion")
	}
	c.Server.set(&v)
	c.Server.ServerRelease = C.GoStringN(release, C.int(releaseLen))
	return nil
}

func (c *conn) setCallTimeout(ctx context.Context) {
	if c.Client.Version < 18 {
		return
	}
	var ms C.uint32_t
	if dl, ok := ctx.Deadline(); ok {
		ms = C.uint32_t(time.Until(dl) / time.Millisecond)
	}
	// force it to be 0 (disabled)
	C.dpiConn_setCallTimeout(c.dpiConn, ms)
}

func maybeBadConn(err error) error {
	if err == nil {
		return nil
	}
	if cd, ok := errors.Cause(err).(interface {
		Code() int
	}); ok {
		// Yes, this is copied from rana/ora, but I've put it there, so it's mine. @tgulacsi
		switch cd.Code() {
		case 0:
			if strings.Contains(err.Error(), " DPI-1002: ") {
				return driver.ErrBadConn
			}
			// cases by experience:
			// ORA-12170: TNS:Connect timeout occurred
			// ORA-12528: TNS:listener: all appropriate instances are blocking new connections
			// ORA-12545: Connect failed because target host or object does not exist
			// ORA-24315: illegal attribute type
			// ORA-28547: connection to server failed, probable Oracle Net admin error
		case 12170, 12528, 12545, 24315, 28547:

			//cases from https://github.com/oracle/odpi/blob/master/src/dpiError.c#L61-L94
		case 22: // invalid session ID; access denied
			fallthrough
		case 28: // your session has been killed
			fallthrough
		case 31: // your session has been marked for kill
			fallthrough
		case 45: // your session has been terminated with no replay
			fallthrough
		case 378: // buffer pools cannot be created as specified
			fallthrough
		case 602: // internal programming exception
			fallthrough
		case 603: // ORACLE server session terminated by fatal error
			fallthrough
		case 609: // could not attach to incoming connection
			fallthrough
		case 1012: // not logged on
			fallthrough
		case 1041: // internal error. hostdef extension doesn't exist
			fallthrough
		case 1043: // user side memory corruption
			fallthrough
		case 1089: // immediate shutdown or close in progress
			fallthrough
		case 1092: // ORACLE instance terminated. Disconnection forced
			fallthrough
		case 2396: // exceeded maximum idle time, please connect again
			fallthrough
		case 3113: // end-of-file on communication channel
			fallthrough
		case 3114: // not connected to ORACLE
			fallthrough
		case 3122: // attempt to close ORACLE-side window on user side
			fallthrough
		case 3135: // connection lost contact
			fallthrough
		case 3136: // inbound connection timed out
			fallthrough
		case 12153: // TNS:not connected
			fallthrough
		case 12537: // TNS:connection closed
			fallthrough
		case 12547: // TNS:lost contact
			fallthrough
		case 12570: // TNS:packet reader failure
			fallthrough
		case 12583: // TNS:no reader
			fallthrough
		case 27146: // post/wait initialization failed
			fallthrough
		case 28511: // lost RPC connection
			fallthrough
		case 56600: // an illegal OCI function call was issued
			return driver.ErrBadConn
		}
	}
	return err
}

func (c *conn) setTraceTag(tt TraceTag) error {
	if c.dpiConn == nil {
		return nil
	}
	//fmt.Fprintf(os.Stderr, "setTraceTag %s\n", tt)
	var err error
	for nm, v := range map[string]*string{
		"action":     &tt.Action,
		"module":     &tt.Module,
		"info":       &tt.ClientInfo,
		"identifier": &tt.ClientIdentifier,
		"op":         &tt.DbOp,
	} {
		var s *C.char
		if *v != "" {
			s = C.CString(*v)
		}
		var rc C.int
		switch nm {
		case "action":
			rc = C.dpiConn_setAction(c.dpiConn, s, C.uint32_t(len(*v)))
		case "module":
			rc = C.dpiConn_setModule(c.dpiConn, s, C.uint32_t(len(*v)))
		case "info":
			rc = C.dpiConn_setClientInfo(c.dpiConn, s, C.uint32_t(len(*v)))
		case "identifier":
			rc = C.dpiConn_setClientIdentifier(c.dpiConn, s, C.uint32_t(len(*v)))
		case "op":
			rc = C.dpiConn_setDbOp(c.dpiConn, s, C.uint32_t(len(*v)))
		}
		if rc == C.DPI_FAILURE && err == nil {
			err = errors.Wrap(c.getError(), nm)
		}
		if s != nil {
			C.free(unsafe.Pointer(s))
		}
	}
	return err
}

const traceTagCtxKey = ctxKey("tracetag")

// ContextWithTraceTag returns a context with the specified TraceTag, which will
// be set on the session used.
func ContextWithTraceTag(ctx context.Context, tt TraceTag) context.Context {
	return context.WithValue(ctx, traceTagCtxKey, tt)
}

// TraceTag holds tracing information for the session. It can be set on the session
// with ContextWithTraceTag.
type TraceTag struct {
	// ClientIdentifier - specifies an end user based on the logon ID, such as HR.HR
	ClientIdentifier string
	// ClientInfo - client-specific info
	ClientInfo string
	// DbOp - database operation
	DbOp string
	// Module - specifies a functional block, such as Accounts Receivable or General Ledger, of an application
	Module string
	// Action - specifies an action, such as an INSERT or UPDATE operation, in a module
	Action string
}
