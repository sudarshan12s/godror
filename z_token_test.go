// Copyright 2024 The Godror Authors
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

func isTokenEnvConfigred(t *testing.T) {
	if os.Getenv("GODROR_TEST_EXPIRED_TOKEN") == nil ||
		os.Getenv("GODROR_TEST_EXPIRED_PVTKEY") == nil ||
		os.Getenv("GODROR_TEST_NEWPVTKEY") == nil ||
		os.Getenv("GODROR_TEST_NEWTOKEN") == nil {
		t.Skip("skipping TestTokenAuthStandAlone test")
	}
}

// - standalone=0
//   - Creates a homogeneous pool with externalAuth = 1.
//     An Expired token is passed during create pool, registering a callback
//     which provides a refresh token.
//     When a Ping is done, callback is called as token is expired and the provided
//     new token key and privateKey provided in the callback are used to perform
//     the ping.

func TestTokenAuthCallBack(t *testing.T) {
	isTokenEnvConfigred(t)
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

	P.Token = os.Getenv("GODROR_TEST_EXPIRED_TOKEN")
	P.PrivateKey = os.Getenv("GODROR_TEST_EXPIRED_PVTKEY")
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

	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
}

// - standalone=1
//   - Creates a standAlone connection with externalAuth = 1, valid token data.

func TestTokenAuthStandAlone(t *testing.T) {
	isTokenEnvConfigred(t)
	ctx, cancel := context.WithTimeout(testContext("TokenAuthStandAlone"), 30*time.Second)
	defer cancel()
	P, err := godror.ParseDSN(testConStr)
	if err != nil {
		t.Fatal(err)
	}

	// Reset user , password
	P.Username = ""
	P.Password.Reset()

	P.Token = os.Getenv("GODROR_TEST_EXPIRED_TOKEN")
	P.PrivateKey = os.Getenv("GODROR_TEST_EXPIRED_NEWPVTKEY")
	P.StandaloneConnection = true
	P.ExternalAuth = true
	t.Log("`" + P.StringWithPassword() + "`")
	db1, err := sql.Open("godror", P.StringWithPassword())
	if err != nil {
		t.Fatal(err)
	}
	defer db1.Close()
	if err := db1.PingContext(ctx); err != nil {
		// expecting token expiry error
		if !strings.Contains(err.Error(), "ORA-25708:") {
			t.Fatal(err)
		}
	}

	P.Token = os.Getenv("GODROR_TEST_NEWTOKEN")
	P.PrivateKey = os.Getenv("GODROR_TEST_NEWPVTKEY")
	t.Log("`" + P.StringWithPassword() + "`")
	db2, err := sql.Open("godror", P.StringWithPassword())
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()

	if err := db2.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
}
