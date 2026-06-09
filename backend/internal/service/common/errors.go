package common

import (
	"errors"

	"connectrpc.com/connect"
)

// AsConnectErr passes through connect errors and wraps others as Internal.
func AsConnectErr(err error) error {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return err
	}
	return connect.NewError(connect.CodeInternal, err)
}

// Deref returns the pointed-to string, or "" if the pointer is nil.
func Deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
