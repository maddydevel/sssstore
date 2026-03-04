package s3api

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sssstore/sssstore/internal/auth"
	"github.com/sssstore/sssstore/internal/storage"
)

type Handler struct {
	store *storage.Store
	auth  auth.Authenticator
}

func New(store *storage.Store, a auth.Authenticator) http.Handler {
	return &Handler{store: store, auth: a}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var principal auth.Principal
	if h.auth != nil {
		p, err := h.auth.Authenticate(r)
		if err != nil {
			h.writeError(w, http.StatusForbidden, "AccessDenied", err.Error())
			return
		}
		principal = p
	}

	if r.URL.Path == "/" {
		if r.Method == http.MethodGet {
			if h.authorize(w, principal, "s3:ListAllMyBuckets") {
				h.listBuckets(w)
			}
			return
		}
		h.writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "unsupported method")
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		h.writeError(w, http.StatusBadRequest, "InvalidURI", "invalid path")
		return
	}
	bucket := parts[0]
	key := strings.Join(parts[1:], "/")

	if key == "" {
		h.routeBucket(w, r, bucket, principal)
		return
	}
	h.routeObject(w, r, bucket, key, principal)
}

func (h *Handler) authorize(w http.ResponseWriter, p auth.Principal, action string) bool {
	if h.auth == nil {
		return true // auth is bypassed entirely
	}
	if err := p.Authorize(action); err != nil {
		h.writeError(w, http.StatusForbidden, "AccessDenied", err.Error())
		return false
	}
	return true
}

func (h *Handler) routeBucket(w http.ResponseWriter, r *http.Request, bucket string, p auth.Principal) {
	switch r.Method {
	case http.MethodPut:
		if _, ok := r.URL.Query()["versioning"]; ok {
			if h.authorize(w, p, "s3:PutBucketVersioning") {
				h.putBucketVersioning(w, r, bucket)
			}
			return
		}
		if h.authorize(w, p, "s3:CreateBucket") {
			h.createBucket(w, bucket)
		}
	case http.MethodDelete:
		if h.authorize(w, p, "s3:DeleteBucket") {
			h.deleteBucket(w, bucket)
		}
	case http.MethodHead:
		if h.authorize(w, p, "s3:HeadBucket") {
			h.headBucket(w, bucket)
		}
	case http.MethodGet:
		if _, ok := r.URL.Query()["versioning"]; ok {
			if h.authorize(w, p, "s3:GetBucketVersioning") {
				h.getBucketVersioning(w, bucket)
			}
			return
		}
		if _, ok := r.URL.Query()["versions"]; ok {
			if h.authorize(w, p, "s3:ListBucketVersions") {
				h.listObjectVersions(w, bucket, r.URL.Query().Get("prefix"), r.URL.Query().Get("max-keys"))
			}
			return
		}
		if r.URL.Query().Get("list-type") == "2" {
			if h.authorize(w, p, "s3:ListBucket") {
				h.listObjectsV2(w, bucket, r.URL.Query().Get("prefix"), r.URL.Query().Get("max-keys"), r.URL.Query().Get("continuation-token"))
			}
			return
		}
		h.writeError(w, http.StatusNotImplemented, "NotImplemented", "only list-type=2, versioning, and versions are supported for bucket GET")
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "unsupported method")
	}
}

func (h *Handler) routeObject(w http.ResponseWriter, r *http.Request, bucket, key string, p auth.Principal) {
	q := r.URL.Query()
	switch r.Method {
	case http.MethodPost:
		if _, ok := q["uploads"]; ok {
			if h.authorize(w, p, "s3:PutObject") {
				h.createMultipartUpload(w, bucket, key)
			}
			return
		}
		if q.Get("uploadId") != "" {
			if h.authorize(w, p, "s3:PutObject") {
				h.completeMultipartUpload(w, bucket, key, q.Get("uploadId"))
			}
			return
		}
		h.writeError(w, http.StatusNotImplemented, "NotImplemented", "unsupported POST operation")
	case http.MethodPut:
		if q.Get("uploadId") != "" && q.Get("partNumber") != "" {
			if h.authorize(w, p, "s3:PutObject") {
				h.uploadPart(w, r, bucket, key, q.Get("uploadId"), q.Get("partNumber"))
			}
			return
		}
		if h.authorize(w, p, "s3:PutObject") {
			h.putObject(w, r, bucket, key)
		}
	case http.MethodGet:
		if h.authorize(w, p, "s3:GetObject") {
			h.getObject(w, r, bucket, key)
		}
	case http.MethodHead:
		if q.Get("versionId") != "" {
			if h.authorize(w, p, "s3:GetObjectVersion") {
				h.headObject(w, bucket, key, q.Get("versionId"))
			}
			return
		}
		if h.authorize(w, p, "s3:GetObject") {
			h.headObject(w, bucket, key, "")
		}
	case http.MethodDelete:
		if q.Get("uploadId") != "" {
			if h.authorize(w, p, "s3:AbortMultipartUpload") {
				h.abortMultipartUpload(w, bucket, q.Get("uploadId"))
			}
			return
		}
		if q.Get("versionId") != "" {
			if h.authorize(w, p, "s3:DeleteObjectVersion") {
				h.deleteObject(w, bucket, key, q.Get("versionId"))
			}
			return
		}
		if h.authorize(w, p, "s3:DeleteObject") {
			h.deleteObject(w, bucket, key, "")
		}
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "unsupported method")
	}
}

type InitiateMultipartUploadResult struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	XMLNS    string   `xml:"xmlns,attr"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	UploadID string   `xml:"UploadId"`
}

type CompleteMultipartUploadResult struct {
	XMLName xml.Name `xml:"CompleteMultipartUploadResult"`
	XMLNS   string   `xml:"xmlns,attr"`
	Bucket  string   `xml:"Bucket"`
	Key     string   `xml:"Key"`
	ETag    string   `xml:"ETag"`
}

func (h *Handler) createMultipartUpload(w http.ResponseWriter, bucket, key string) {
	uploadID, err := h.store.CreateMultipartUpload(bucket, key)
	if errors.Is(err, storage.ErrBucketNotFound) {
		h.writeError(w, http.StatusNotFound, "NoSuchBucket", err.Error())
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	h.writeXML(w, http.StatusOK, InitiateMultipartUploadResult{XMLNS: "http://s3.amazonaws.com/doc/2006-03-01/", Bucket: bucket, Key: key, UploadID: uploadID})
}

func (h *Handler) uploadPart(w http.ResponseWriter, r *http.Request, bucket, key, uploadID, partNumberRaw string) {
	pn, err := strconv.Atoi(partNumberRaw)
	if err != nil || pn < 1 {
		h.writeError(w, http.StatusBadRequest, "InvalidPart", "invalid part number")
		return
	}
	part, err := h.store.UploadPart(bucket, key, uploadID, pn, r.Body)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "InvalidRequest", err.Error())
		return
	}
	w.Header().Set("ETag", part.ETag)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) completeMultipartUpload(w http.ResponseWriter, bucket, key, uploadID string) {
	info, err := h.store.CompleteMultipartUpload(bucket, key, uploadID)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "InvalidRequest", err.Error())
		return
	}
	h.writeXML(w, http.StatusOK, CompleteMultipartUploadResult{XMLNS: "http://s3.amazonaws.com/doc/2006-03-01/", Bucket: bucket, Key: key, ETag: info.ETag})
}

func (h *Handler) abortMultipartUpload(w http.ResponseWriter, bucket, uploadID string) {
	if err := h.store.AbortMultipartUpload(bucket, uploadID); err != nil {
		h.writeError(w, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// rest unchanged
type ListAllMyBucketsResult struct {
	XMLName xml.Name     `xml:"ListAllMyBucketsResult"`
	XMLNS   string       `xml:"xmlns,attr"`
	Buckets BucketsField `xml:"Buckets"`
}

type BucketsField struct {
	Bucket []BucketXML `xml:"Bucket"`
}

type BucketXML struct {
	Name         string `xml:"Name"`
	CreationDate string `xml:"CreationDate"`
}

func (h *Handler) listBuckets(w http.ResponseWriter) { /*...*/
	buckets, err := h.store.ListBuckets()
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	res := ListAllMyBucketsResult{XMLNS: "http://s3.amazonaws.com/doc/2006-03-01/"}
	for _, b := range buckets {
		res.Buckets.Bucket = append(res.Buckets.Bucket, BucketXML{Name: b.Name, CreationDate: b.CreationDate.Format(time.RFC3339)})
	}
	h.writeXML(w, http.StatusOK, res)
}

func (h *Handler) createBucket(w http.ResponseWriter, bucket string) {
	if err := h.store.CreateBucket(bucket); err != nil {
		h.writeError(w, http.StatusBadRequest, "InvalidBucketName", err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}
func (h *Handler) deleteBucket(w http.ResponseWriter, bucket string) {
	err := h.store.DeleteBucket(bucket)
	switch {
	case errors.Is(err, storage.ErrBucketNotFound):
		h.writeError(w, http.StatusNotFound, "NoSuchBucket", err.Error())
	case errors.Is(err, storage.ErrBucketNotEmpty):
		h.writeError(w, http.StatusConflict, "BucketNotEmpty", err.Error())
	case err != nil:
		h.writeError(w, http.StatusInternalServerError, "InternalError", err.Error())
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}
func (h *Handler) headBucket(w http.ResponseWriter, bucket string) {
	exists, err := h.store.BucketExists(bucket)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	if !exists {
		h.writeError(w, http.StatusNotFound, "NoSuchBucket", storage.ErrBucketNotFound.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

type VersioningConfiguration struct {
	XMLName xml.Name `xml:"VersioningConfiguration"`
	Status  string   `xml:"Status,omitempty"`
}

func (h *Handler) getBucketVersioning(w http.ResponseWriter, bucket string) {
	enabled, err := h.store.GetBucketVersioning(bucket)
	if errors.Is(err, storage.ErrBucketNotFound) {
		h.writeError(w, http.StatusNotFound, "NoSuchBucket", err.Error())
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	status := ""
	if enabled {
		status = "Enabled"
	}
	h.writeXML(w, http.StatusOK, VersioningConfiguration{Status: status})
}

func (h *Handler) putBucketVersioning(w http.ResponseWriter, r *http.Request, bucket string) {
	var cfg VersioningConfiguration
	if err := xml.NewDecoder(r.Body).Decode(&cfg); err != nil {
		h.writeError(w, http.StatusBadRequest, "MalformedXML", err.Error())
		return
	}
	enabled := strings.EqualFold(cfg.Status, "Enabled")
	if err := h.store.SetBucketVersioning(bucket, enabled); err != nil {
		if errors.Is(err, storage.ErrBucketNotFound) {
			h.writeError(w, http.StatusNotFound, "NoSuchBucket", err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

type ListVersionsResult struct {
	XMLName  xml.Name                    `xml:"ListVersionsResult"`
	Name     string                      `xml:"Name"`
	Prefix   string                      `xml:"Prefix"`
	Versions []storage.ObjectVersionInfo `xml:"Version"`
}

func (h *Handler) listObjectVersions(w http.ResponseWriter, bucket, prefix, maxKeysRaw string) {
	maxKeys, _ := strconv.Atoi(maxKeysRaw)
	versions, err := h.store.ListObjectVersions(bucket, prefix, maxKeys)
	if errors.Is(err, storage.ErrBucketNotFound) {
		h.writeError(w, http.StatusNotFound, "NoSuchBucket", err.Error())
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	h.writeXML(w, http.StatusOK, ListVersionsResult{Name: bucket, Prefix: prefix, Versions: versions})
}

type ListBucketResult struct {
	XMLName           xml.Name      `xml:"ListBucketResult"`
	XMLNS             string        `xml:"xmlns,attr"`
	Name              string        `xml:"Name"`
	Prefix            string        `xml:"Prefix"`
	KeyCount          int           `xml:"KeyCount"`
	MaxKeys           int           `xml:"MaxKeys"`
	IsTruncated       bool          `xml:"IsTruncated"`
	ContinuationToken string        `xml:"ContinuationToken,omitempty"`
	NextToken         string        `xml:"NextContinuationToken,omitempty"`
	Contents          []ObjectEntry `xml:"Contents"`
}
type ObjectEntry struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag,omitempty"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

func (h *Handler) listObjectsV2(w http.ResponseWriter, bucket, prefix, maxKeysRaw, continuationToken string) {
	maxKeys, _ := strconv.Atoi(maxKeysRaw)
	objs, nextToken, truncated, err := h.store.ListObjectsV2(bucket, prefix, continuationToken, maxKeys)
	if errors.Is(err, storage.ErrBucketNotFound) {
		h.writeError(w, http.StatusNotFound, "NoSuchBucket", err.Error())
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	if maxKeys <= 0 {
		maxKeys = 1000
	}
	result := ListBucketResult{XMLNS: "http://s3.amazonaws.com/doc/2006-03-01/", Name: bucket, Prefix: prefix, MaxKeys: maxKeys, IsTruncated: truncated, ContinuationToken: continuationToken, NextToken: nextToken}
	for _, obj := range objs {
		result.Contents = append(result.Contents, ObjectEntry{Key: obj.Key, LastModified: obj.LastModified.Format(time.RFC3339), ETag: obj.ETag, Size: obj.Size, StorageClass: "STANDARD"})
	}
	result.KeyCount = len(result.Contents)
	h.writeXML(w, http.StatusOK, result)
}
func (h *Handler) putObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	info, err := h.store.PutObject(bucket, key, r.Body)
	if errors.Is(err, storage.ErrBucketNotFound) {
		h.writeError(w, http.StatusNotFound, "NoSuchBucket", err.Error())
		return
	}
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "InvalidRequest", err.Error())
		return
	}
	if info.ETag != "" {
		w.Header().Set("ETag", info.ETag)
	}
	w.WriteHeader(http.StatusOK)
}
func parseRange(hdr string, size int64) (start, end int64, ok bool) {
	if !strings.HasPrefix(hdr, "bytes=") {
		return 0, 0, false
	}
	parts := strings.Split(strings.TrimPrefix(hdr, "bytes="), "-")
	if len(parts) != 2 {
		return 0, 0, false
	}
	s, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || s < 0 {
		return 0, 0, false
	}
	if parts[1] == "" {
		return s, size - 1, s < size
	}
	e, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || e < s || e >= size {
		return 0, 0, false
	}
	return s, e, true
}
func checkPreconditions(w http.ResponseWriter, r *http.Request, info storage.ObjectInfo) bool {
	if match := r.Header.Get("If-Match"); match != "" {
		if info.ETag != match {
			w.WriteHeader(http.StatusPreconditionFailed)
			return false
		}
	}
	if noneMatch := r.Header.Get("If-None-Match"); noneMatch != "" {
		if info.ETag == noneMatch || noneMatch == "*" {
			w.WriteHeader(http.StatusNotModified)
			return false
		}
	}
	if unmodSince := r.Header.Get("If-Unmodified-Since"); unmodSince != "" {
		if t, err := http.ParseTime(unmodSince); err == nil {
			if info.LastModified.After(t) {
				w.WriteHeader(http.StatusPreconditionFailed)
				return false
			}
		}
	}
	if modSince := r.Header.Get("If-Modified-Since"); modSince != "" {
		if t, err := http.ParseTime(modSince); err == nil {
			if !info.LastModified.After(t) {
				w.WriteHeader(http.StatusNotModified)
				return false
			}
		}
	}
	return true
}

func (h *Handler) getObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	rc, info, err := h.store.GetObjectVersion(bucket, key, r.URL.Query().Get("versionId"))
	if errors.Is(err, storage.ErrObjectNotFound) {
		h.writeError(w, http.StatusNotFound, "NoSuchKey", err.Error())
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	defer rc.Close()
	w.Header().Set("Last-Modified", info.LastModified.Format(http.TimeFormat))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size))
	if info.ETag != "" {
		w.Header().Set("ETag", info.ETag)
	}

	if !checkPreconditions(w, r, info) {
		return
	}

	if rg := r.Header.Get("Range"); rg != "" {
		if rs, ok := rc.(io.ReadSeeker); ok {
			start, end, valid := parseRange(rg, info.Size)
			if !valid {
				w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", info.Size))
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			_, _ = rs.Seek(start, io.SeekStart)
			length := end - start + 1
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, info.Size))
			w.Header().Set("Content-Length", fmt.Sprintf("%d", length))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = io.CopyN(w, rs, length)
			return
		}
	}
	_, _ = io.Copy(w, rc)
}
func (h *Handler) headObject(w http.ResponseWriter, bucket, key, versionID string) {
	rc, info, err := h.store.GetObjectVersion(bucket, key, versionID)
	if rc != nil {
		_ = rc.Close()
	}
	if errors.Is(err, storage.ErrObjectNotFound) {
		h.writeError(w, http.StatusNotFound, "NoSuchKey", err.Error())
		return
	}
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.Header().Set("Last-Modified", info.LastModified.Format(http.TimeFormat))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size))
	if info.ETag != "" {
		w.Header().Set("ETag", info.ETag)
	}
	w.WriteHeader(http.StatusOK)
}
func (h *Handler) deleteObject(w http.ResponseWriter, bucket, key, versionID string) {
	err := h.store.DeleteObjectVersion(bucket, key, versionID)
	if err != nil && !errors.Is(err, storage.ErrObjectNotFound) {
		h.writeError(w, http.StatusInternalServerError, "InternalError", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type ErrorResponse struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, msg string) {
	h.writeXML(w, status, ErrorResponse{Code: code, Message: msg})
}
func (h *Handler) writeXML(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)
	_ = xml.NewEncoder(w).Encode(payload)
}
