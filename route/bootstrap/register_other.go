//go:build !darwin

package bootstrap

import "github.com/go-chi/chi/v5"

func Register(_ chi.Router, _ string) {}
