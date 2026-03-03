package auth

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
)

var (
	ErrAccessDenied = errors.New("access denied")
	credPattern     = regexp.MustCompile(`Credential=([^/\s,]+)`) // AWS4-HMAC-SHA256 Credential=<access-key>/...
)

type Authenticator interface {
	Authenticate(r *http.Request) error
}

type StaticAuthenticator struct {
	allowed map[string]struct{}
}

func NewStaticAuthenticator(keys []string) *StaticAuthenticator {
	m := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		if strings.TrimSpace(k) == "" {
			continue
		}
		m[k] = struct{}{}
	}
	return &StaticAuthenticator{allowed: m}
}

func (a *StaticAuthenticator) Authenticate(r *http.Request) error {
	authz := r.Header.Get("Authorization")
	if authz == "" {
		return ErrAccessDenied
	}
	m := credPattern.FindStringSubmatch(authz)
	if len(m) != 2 {
		return ErrAccessDenied
	}
	if _, ok := a.allowed[m[1]]; !ok {
		return ErrAccessDenied
	}
	return nil
}
