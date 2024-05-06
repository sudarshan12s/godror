// Copyright 2018, 2020 The Godror Authors
//
//
// SPDX-License-Identifier: UPL-1.0 OR Apache-2.0

package godror_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
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
	P.Username = ""
	P.Password.Reset()
	const hostName = "test.clouddb.com"
	const pno = 443
	tokenCtx := context.WithValue(context.Background(), "host", hostName)
	tokenCtx = context.WithValue(tokenCtx, "port", pno)
	cb := func(ctx context.Context, tok *dsn.AccessToken) error {

		if !strings.EqualFold(ctx.Value("host").(string), hostName) {
			t.Errorf("TestTokenAuthCallBack: hostName got %s, wanted %s", ctx.Value("host"), hostName)
		}
		newtoken := os.Getenv("GODROR_TEST_NEWTOKEN")
		newpvtkey := os.Getenv("GODROR_TEST_NEWPVTKEY")
		tok.Token = newtoken
		tok.PrivateKey = newpvtkey
		t.Log(" Token Passed in Callback", tok)
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
		TokenCBCtx:     tokenCtx,
	}
	P.ExternalAuth = true
	db := sql.OpenDB(godror.NewConnector(P))
	defer db.Close()

	// create OCI SessionPool
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestTokenAuthStandAlone(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(testContext("TokenAuthStandAlone"), 30*time.Second)
	defer cancel()
	P, err := godror.ParseDSN(testConStr)
	if err != nil {
		t.Fatal(err)
	}

	// Reset user , password
	P.Username = ""
	P.Password.Reset()

	P.Token = os.Getenv("GODROR_TEST_NEWTOKEN")
	P.PrivateKey = os.Getenv("GODROR_TEST_NEWPVTKEY")
	P.StandaloneConnection = true
	P.ExternalAuth = true
	t.Log("`" + P.StringWithPassword() + "`")
	db, err := sql.Open("godror", P.StringWithPassword())
	if err != nil {
		t.Fatal(err)
		// TBD check for token expiry
		//ORA-25708:
	}
	defer db.Close()

	// create OCI SessionPool
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
}
