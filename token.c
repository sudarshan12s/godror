#include <stdio.h>
#include "dpiImpl.h"

void TokenCallbackHandler(void *context, dpiAccessToken *access_token);

void TokenCallbackHandlerDebug(void *context, dpiAccessToken *acToken) {
	fprintf(stderr, "callback called\n");
	TokenCallbackHandler(context, acToken);
}
