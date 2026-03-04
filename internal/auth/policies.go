package auth

import "errors"

const (
	PolicyAdmin      = "admin"
	PolicyReadWrite  = "read-write"
	PolicyReadOnly   = "read-only"
)

var ErrUnauthorized = errors.New("unauthorized action for given policy")

type Principal struct {
	AccessKey string
	Policy    string
}

func (p Principal) Authorize(action string) error {
	if p.Policy == PolicyAdmin {
		return nil // Admin can do anything
	}

	isReadAction := isRead(action)
	switch p.Policy {
	case PolicyReadWrite:
		// read-write cannot create/delete buckets or change versioning config
		if action == "s3:CreateBucket" || action == "s3:DeleteBucket" || action == "s3:PutBucketVersioning" {
			return ErrUnauthorized
		}
		return nil // allowed to do any object read/write
	case PolicyReadOnly:
		if isReadAction {
			return nil
		}
		return ErrUnauthorized
	default:
		return ErrUnauthorized
	}
}

func isRead(action string) bool {
	switch action {
	case "s3:ListAllMyBuckets",
		"s3:ListBucket",
		"s3:ListBucketVersions",
		"s3:HeadBucket",
		"s3:GetBucketVersioning",
		"s3:GetObject",
		"s3:GetObjectVersion":
		return true
	default:
		return false
	}
}
