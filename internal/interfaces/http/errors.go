package http

import (
	"log"

	"github.com/danielgtaylor/huma/v2"
)

// upstreamError logs the full upstream error server-side and returns a sanitised
// huma.StatusError that does not leak internal infrastructure (URLs, response bodies,
// stack traces) to the client.
func upstreamError(component string, err error) huma.StatusError {
	log.Printf("upstream error in %s: %v", component, err)
	return huma.Error502BadGateway("upstream service is currently unavailable")
}

// internalError logs the full error server-side and returns a generic 500 to the client.
func internalError(component string, err error) huma.StatusError {
	log.Printf("internal error in %s: %v", component, err)
	return huma.Error500InternalServerError("internal server error")
}
