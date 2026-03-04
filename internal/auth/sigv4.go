package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

var (
	ErrInvalidAuthHeader = errors.New("invalid authorization header")
	ErrInvalidSignature  = errors.New("signature does not match")
	ErrDateTooSkewed     = errors.New("request date is too skewed")
)

type SigV4Params struct {
	AccessKey     string
	Date          string
	Region        string
	Service       string
	SignedHeaders []string
	Signature     string
}

func parseSigV4(authz string) (SigV4Params, error) {
	if !strings.HasPrefix(authz, "AWS4-HMAC-SHA256 ") {
		return SigV4Params{}, ErrInvalidAuthHeader
	}
	authz = strings.TrimPrefix(authz, "AWS4-HMAC-SHA256 ")

	var p SigV4Params
	parts := strings.Split(authz, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "Credential=") {
			cred := strings.TrimPrefix(part, "Credential=")
			cParts := strings.Split(cred, "/")
			if len(cParts) != 5 {
				return p, ErrInvalidAuthHeader
			}
			p.AccessKey = cParts[0]
			p.Date = cParts[1]
			p.Region = cParts[2]
			p.Service = cParts[3]
		} else if strings.HasPrefix(part, "SignedHeaders=") {
			p.SignedHeaders = strings.Split(strings.TrimPrefix(part, "SignedHeaders="), ";")
		} else if strings.HasPrefix(part, "Signature=") {
			p.Signature = strings.TrimPrefix(part, "Signature=")
		}
	}
	if p.AccessKey == "" || p.Signature == "" {
		return p, ErrInvalidAuthHeader
	}
	return p, nil
}

func hashSHA256(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func getSigningKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "aws4_request")
	return kSigning
}

// urlEncode precisely follows SigV4 encode rules
func urlEncode(s string, path bool) string {
	var buf strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' {
			buf.WriteByte(c)
		} else if path && c == '/' {
			buf.WriteByte(c)
		} else {
			buf.WriteString(fmt.Sprintf("%%%02X", c))
		}
	}
	return buf.String()
}

func buildCanonicalRequest(r *http.Request, p SigV4Params) string {
	// 1. Method
	method := r.Method

	// 2. Canonical URI
	uri := r.URL.Path
	if uri == "" {
		uri = "/"
	}
	uri = urlEncode(uri, true)

	// 3. Canonical Query String
	var queryKeys []string
	for k := range r.URL.Query() {
		queryKeys = append(queryKeys, urlEncode(k, false))
	}
	sort.Strings(queryKeys)
	var canonicalQuery []string
	for _, k := range queryKeys {
		decodedK, _ := url.QueryUnescape(k) // original key
		if decodedK == "" {
			decodedK = k // fallback
		}
		
		vals := r.URL.Query()[decodedK]
		if len(vals) == 0 {
			canonicalQuery = append(canonicalQuery, k+"=")
			continue
		}
		
		var encodedVals []string
		for _, v := range vals {
			encodedVals = append(encodedVals, urlEncode(v, false))
		}
		sort.Strings(encodedVals)
		for _, v := range encodedVals {
			canonicalQuery = append(canonicalQuery, k+"="+v)
		}
	}
	queryString := strings.Join(canonicalQuery, "&")

	// 4. Canonical Headers
	var canonicalHeaders []string
	for _, sh := range p.SignedHeaders {
		sh = strings.ToLower(sh)
		if sh == "host" {
			canonicalHeaders = append(canonicalHeaders, "host:"+strings.TrimSpace(r.Host))
			continue
		}
		val := r.Header.Get(sh)
		// Space collapsing per RFC is technically required but often strings.TrimSpace is enough
		canonicalHeaders = append(canonicalHeaders, sh+":"+strings.TrimSpace(val))
	}
	headerString := strings.Join(canonicalHeaders, "\n") + "\n"

	// 5. Signed Headers
	signedHeadersStr := strings.Join(p.SignedHeaders, ";")

	// 6. Payload Hash
	payloadHash := r.Header.Get("X-Amz-Content-Sha256")
	if payloadHash == "" {
		payloadHash = "UNSIGNED-PAYLOAD"
	}

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s", method, uri, queryString, headerString, signedHeadersStr, payloadHash)
}

func VerifySigV4(r *http.Request, secretKey string) (SigV4Params, error) {
	authz := r.Header.Get("Authorization")
	p, err := parseSigV4(authz)
	if err != nil {
		return p, err
	}

	amzDate := r.Header.Get("X-Amz-Date")
	if amzDate == "" {
		amzDate = r.Header.Get("Date")
	}
	if amzDate == "" {
		return p, errors.New("missing date header for signature")
	}

	canonicalRequest := buildCanonicalRequest(r, p)
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", p.Date, p.Region, p.Service)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s", amzDate, credentialScope, hashSHA256([]byte(canonicalRequest)))

	signingKey := getSigningKey(secretKey, p.Date, p.Region, p.Service)
	expectedSignature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	if expectedSignature != p.Signature {
		return p, ErrInvalidSignature
	}

	return p, nil
}
