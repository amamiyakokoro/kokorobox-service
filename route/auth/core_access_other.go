//go:build !windows

package auth

import "net/http"

func RequireWindowsCoreAdministrator(next http.Handler) http.Handler {
	return next
}
