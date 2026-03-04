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

func AccessKeyFromRequest(r *http.Request) string {
	authz := r.Header.Get("Authorization")
	if authz == "" {
		return ""
	}
	m := credPattern.FindStringSubmatch(authz)
	if len(m) != 2 {
		return ""
	}
	return m[1]
}

func (a *StaticAuthenticator) Authenticate(r *http.Request) error {
	key := AccessKeyFromRequest(r)
	if key == "" {
		return ErrAccessDenied
	}
	if _, ok := a.allowed[key]; !ok {
		return ErrAccessDenied
	}
	return nil
}
