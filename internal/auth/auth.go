package auth

import (
	"errors"
	"net/http"
	"regexp"

	"github.com/sssstore/sssstore/internal/config"
)

var (
	ErrAccessDenied = errors.New("access denied")
	credPattern     = regexp.MustCompile(`Credential=([^/\s,]+)`)
)

type Authenticator interface {
	Authenticate(r *http.Request) (Principal, error)
}

type SigV4Authenticator struct {
	users map[string]config.User
}

func NewSigV4Authenticator(users []config.User) *SigV4Authenticator {
	m := make(map[string]config.User, len(users))
	for _, u := range users {
		m[u.AccessKey] = u
	}
	return &SigV4Authenticator{users: m}
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

func (a *SigV4Authenticator) Authenticate(r *http.Request) (Principal, error) {
	key := AccessKeyFromRequest(r)
	if key == "" {
		return Principal{}, ErrAccessDenied
	}
	u, ok := a.users[key]
	if !ok {
		return Principal{}, ErrAccessDenied
	}
	
	// Perform robust AWS Signature (SigV4) Cryptographic Verification
	_, err := VerifySigV4(r, u.SecretKey)
	if err != nil {
		return Principal{}, ErrAccessDenied
	}

	return Principal{
		AccessKey: u.AccessKey,
		Policy:    u.Policy,
	}, nil
}

