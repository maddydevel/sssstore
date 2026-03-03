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

	"github.com/sssstore/sssstore/internal/storage"
)

type Handler struct {
	store *storage.Store
}

func New(store *storage.Store) http.Handler {
	return &Handler{store: store}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		if r.Method == http.MethodGet {
			h.listBuckets(w, r)
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
		h.routeBucket(w, r, bucket)
		return
	}
	h.routeObject(w, r, bucket, key)
}

func (h *Handler) routeBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	switch r.Method {
	case http.MethodPut:
		h.createBucket(w, bucket)
	case http.MethodDelete:
		h.deleteBucket(w, bucket)
	case http.MethodGet:
		if r.URL.Query().Get("list-type") == "2" {
			h.listObjectsV2(w, bucket, r.URL.Query().Get("prefix"), r.URL.Query().Get("max-keys"))
			return
		}
		h.writeError(w, http.StatusNotImplemented, "NotImplemented", "only list-type=2 is supported for bucket GET")
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "unsupported method")
	}
}

func (h *Handler) routeObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	switch r.Method {
	case http.MethodPut:
		h.putObject(w, r, bucket, key)
	case http.MethodGet:
		h.getObject(w, bucket, key)
	case http.MethodHead:
		h.headObject(w, bucket, key)
	case http.MethodDelete:
		h.deleteObject(w, bucket, key)
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "MethodNotAllowed", "unsupported method")
	}
}

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

func (h *Handler) listBuckets(w http.ResponseWriter, _ *http.Request) {
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

type ListBucketResult struct {
	XMLName     xml.Name      `xml:"ListBucketResult"`
	XMLNS       string        `xml:"xmlns,attr"`
	Name        string        `xml:"Name"`
	Prefix      string        `xml:"Prefix"`
	KeyCount    int           `xml:"KeyCount"`
	MaxKeys     int           `xml:"MaxKeys"`
	IsTruncated bool          `xml:"IsTruncated"`
	Contents    []ObjectEntry `xml:"Contents"`
}

type ObjectEntry struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag,omitempty"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

func (h *Handler) listObjectsV2(w http.ResponseWriter, bucket, prefix, maxKeysRaw string) {
	maxKeys, _ := strconv.Atoi(maxKeysRaw)
	objs, err := h.store.ListObjectsV2(bucket, prefix, maxKeys)
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
	result := ListBucketResult{XMLNS: "http://s3.amazonaws.com/doc/2006-03-01/", Name: bucket, Prefix: prefix, MaxKeys: maxKeys}
	for _, obj := range objs {
		result.Contents = append(result.Contents, ObjectEntry{Key: obj.Key, LastModified: obj.LastModified.Format(time.RFC3339), Size: obj.Size, StorageClass: "STANDARD"})
	}
	result.KeyCount = len(result.Contents)
	h.writeXML(w, http.StatusOK, result)
}

func (h *Handler) putObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	_, err := h.store.PutObject(bucket, key, r.Body)
	if errors.Is(err, storage.ErrBucketNotFound) {
		h.writeError(w, http.StatusNotFound, "NoSuchBucket", err.Error())
		return
	}
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "InvalidRequest", err.Error())
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) getObject(w http.ResponseWriter, bucket, key string) {
	rc, info, err := h.store.GetObject(bucket, key)
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
	_, _ = io.Copy(w, rc)
}

func (h *Handler) headObject(w http.ResponseWriter, bucket, key string) {
	rc, info, err := h.store.GetObject(bucket, key)
	if rc != nil {
		rc.Close()
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
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) deleteObject(w http.ResponseWriter, bucket, key string) {
	err := h.store.DeleteObject(bucket, key)
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
