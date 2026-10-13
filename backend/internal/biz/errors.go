package biz

import (
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

/*
Kratos encodes a *errors.Error into HTTP by using its Code as the status code,
and the body is {code, reason, message, metadata}. The frontend's ApiError reads
exactly those fields, so constructing the right error here is all that is needed
to produce a correct response — the transport layer does not map anything.

Reasons are stable, uppercase identifiers a caller could branch on; messages are
the Chinese text shown to the user.
*/

func errBadRequest(message string) *errors.Error {
	return errors.BadRequest("INVALID_ARGUMENT", message)
}

func errUnauthorized(message string) *errors.Error {
	return errors.Unauthorized("UNAUTHORIZED", message)
}

func errForbidden(message string) *errors.Error {
	return errors.Forbidden("FORBIDDEN", message)
}

func errNotFoundError(message string) *errors.Error {
	return errors.NotFound("NOT_FOUND", message)
}

func errConflict(message string) *errors.Error {
	return errors.Conflict("CONFLICT", message)
}

func errTooManyRequests(message string) *errors.Error {
	return errors.New(429, "TOO_MANY_REQUESTS", message)
}

// internal logs the underlying cause and returns a message that does not leak
// server internals to the client.
func internal(message string, cause error) *errors.Error {
	log.Errorf("biz: %s: %v", message, cause)
	return errors.InternalServer("INTERNAL_ERROR", message)
}
