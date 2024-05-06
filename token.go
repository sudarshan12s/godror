// Copyright 2024 The Godror Authors
//
//
// SPDX-License-Identifier: UPL-1.0 OR Apache-2.0

package godror

/*
#include <stdlib.h>
#include <stdio.h>
#include "dpiImpl.h"

int TokenCallbackHandlerDebug(void* context, dpiAccessToken *token);

*/
import "C"

import (
	"context"
	"github.com/godror/godror/dsn"
	"log"
	"sync"
	"unsafe"
)

// AccessToken Callback information.
type AccessTokenCB struct {
	ctx      context.Context
	callback func(context.Context, *dsn.AccessToken) error
	ID       uint64
}

// Cannot pass *AccessTokenCB to C, so pass an uint64 that points to this map entry
var (
	accessTokenMu sync.Mutex
	accessTokens  = make(map[uint64]*AccessTokenCB)
	accessTokenID uint64
)

// tokenCallbackHandler is the callback for C code on token expiry.
//
//export TokenCallbackHandler
func TokenCallbackHandler(ctx unsafe.Pointer, accessTokenC *C.dpiAccessToken) {
	log.Printf("CB %p %+v", ctx, accessTokenC)
	accessTokenMu.Lock()
	acessTokenCB := accessTokens[*((*uint64)(ctx))]
	accessTokenMu.Unlock()

	// Call user function which provides the new token and privateKey.
	var refreshAccessToken dsn.AccessToken
	if err := acessTokenCB.callback(acessTokenCB.ctx, &refreshAccessToken); err != nil {
		log.Printf("Token Generation Failed CB %p %+v", ctx, acessTokenCB)
	}

	token := refreshAccessToken.Token
	privateKey := refreshAccessToken.PrivateKey
	// TBD free these strings.
	accessTokenC.token = C.CString(token)
	accessTokenC.tokenLength = C.uint32_t(len(token))
	accessTokenC.privateKey = C.CString(privateKey)
	accessTokenC.privateKeyLength = C.uint32_t(len(privateKey))
}

// RegisterTokenCallback.
func RegisterTokenCallback(poolCreateParams *C.dpiPoolCreateParams,
	tokenGenFn func(context.Context, *dsn.AccessToken) error,
	tokenCtx context.Context) {

	// typedef int (*dpiAccessTokenCallback)(void* context, dpiAccessToken *accessToken);
	poolCreateParams.accessTokenCallback = C.dpiAccessTokenCallback(C.TokenCallbackHandlerDebug)

	// cannot pass &token to C, so pass indirectly
	aToken := AccessTokenCB{callback: tokenGenFn, ctx: tokenCtx}
	accessTokenMu.Lock()
	accessTokenID++
	aToken.ID = accessTokenID
	accessTokens[aToken.ID] = &aToken
	accessTokenMu.Unlock()
	tokenID := (*C.uint64_t)(C.malloc(8))
	*tokenID = C.uint64_t(accessTokenID)
	poolCreateParams.accessTokenCallbackContext = unsafe.Pointer(tokenID)
}
