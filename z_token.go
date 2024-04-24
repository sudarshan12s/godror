// Copyright 2018, 2020 The Godror Authors
//
//
// SPDX-License-Identifier: UPL-1.0 OR Apache-2.0

package godror_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	godror "github.com/godror/godror"
	"github.com/godror/godror/dsn"
)

func TestTokenAuthCallBack(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(testContext("TokenAuthCallBack"), 30*time.Second)
	defer cancel()
	P, err := godror.ParseConnString(testConStr)
	if err != nil {
		t.Fatal(err)
	}

	// Reset user and passwd
	P.Username = nil
	P.Password.Reset()
	P.homeogeneous = true
	tokenCtx := context.WithValue(context.Background(), "host", "test.clouddb.com")
	cb := func(ctx context.Context, tok *dsn.AccessToken) error {
		fmt.Println("inside token expiry calback")
		fmt.Println(" context passed ", ctx.Value("foo"))
		newtoken := os.Getenv("GODROR_TEST_NEWTOKEN")
		newpvtkey := os.Getenv("GODROR_TEST_NEWPVTKEY")
		tok.Token = newtoken
		tok.PrivateKey = newpvtkey
		fmt.Println(tok)
		return nil
	}

	P.Token = os.Getenv("GODROR_TEST_TOKEN")
	P.PrivateKey = os.Getenv("GODROR_TEST_PVTKEY")
	P.PoolParams = godror.PoolParams{
		MinSessions: 0, MaxSessions: 10, SessionIncrement: 1,
		WaitTimeout:    5 * time.Second,
		MaxLifeTime:    5 * time.Minute,
		SessionTimeout: 1 * time.Minute,
		TokenCB:        cb,
		TokenCBCtx:     ctx,
	}
	P.ExternalAuth = true
	db := sql.OpenDB(godror.NewConnector(P))
	defer db.Close()

	// create OCI SessionPool
	if err := db.Ping(); err != nil {
		fmt.Println(" ping failed  ")
		//log.Fatal(err)
	}
}

func TestTokenAuthStandAlone(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(testContext("TokenAuthStandAlone"), 30*time.Second)
	defer cancel()
	P, err := godror.ParseConnString(testConStr)
	if err != nil {
		t.Fatal(err)
	}

	// Reset user , password
	P.Username = nil
	P.Password.Reset()
	P.homeogeneous = true

	P.Token = os.Getenv("GODROR_TEST_TOKEN")
	P.PrivateKey = os.Getenv("GODROR_TEST_PVTKEY")
	P.StandaloneConnection = true
	P.ExternalAuth = true
	db := sql.Open("godror", P.string())
	defer db.Close()

	// create OCI SessionPool
	if err := db.Ping(); err != nil {
		fmt.Println(" ping failed  ")
		//log.Fatal(err)
	}
}
