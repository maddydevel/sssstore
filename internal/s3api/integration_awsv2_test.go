package s3api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/sssstore/sssstore/internal/auth"
	appconfig "github.com/sssstore/sssstore/internal/config"
	"github.com/sssstore/sssstore/internal/storage"
)

func newTestS3Client(t *testing.T, srvURL, accessKey, secretKey string) *s3.Client {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		config.WithRegion("us-east-1"),
	)
	if err != nil {
		t.Fatalf("failed to load AWS config: %v", err)
	}

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(srvURL)
		o.UsePathStyle = true
	})
}

func TestAWSSDKv2Integration(t *testing.T) {
	tmp := t.TempDir()
	users := []appconfig.User{{Name: "int", AccessKey: "integration-key", SecretKey: "integration-secret", Policy: auth.PolicyAdmin}}
	h := New(storage.New(tmp), auth.NewSigV4Authenticator(users))
	ts := httptest.NewServer(h)
	defer ts.Close()

	client := newTestS3Client(t, ts.URL, "integration-key", "integration-secret")
	ctx := context.Background()
	bucket := "test-sdk-v2-bucket"

	// 1. CreateBucket
	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatalf("CreateBucket failed: %v", err)
	}

	// 2. PutObject
	key := "test-file.txt"
	content := "hello from aws sdk v2"
	putRes, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(content),
	})
	if err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}
	if putRes.ETag == nil {
		t.Fatalf("PutObject missing ETag")
	}

	etag := *putRes.ETag

	// 3. GetObject
	getRes, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	body, _ := io.ReadAll(getRes.Body)
	getRes.Body.Close()
	if string(body) != content {
		t.Fatalf("GetObject content mismatch: got %v", string(body))
	}

	// 4. GetObject Conditonals (If-Match success)
	getCondRes, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:  aws.String(bucket),
		Key:     aws.String(key),
		IfMatch: aws.String(etag),
	})
	if err != nil {
		t.Fatalf("GetObject If-Match failed: %v", err)
	}
	getCondRes.Body.Close()

	// 5. GetObject Conditionals (If-Match failure = 412 Precondition Failed)
	_, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:  aws.String(bucket),
		Key:     aws.String(key),
		IfMatch: aws.String(`"incorrect-etag"`),
	})
	if err == nil {
		t.Fatalf("Expected GetObject If-Match failure, but got success")
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && apiErr.ErrorCode() != "PreconditionFailed" {
		t.Fatalf("Expected PreconditionFailed, got %v", apiErr.ErrorCode())
	}

	// 6. GetObject Conditionals (If-None-Match failure = 304 Not Modified)
	_, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		IfNoneMatch: aws.String(etag),
	})
	if err == nil {
		t.Fatalf("Expected GetObject If-None-Match failure, but got success")
	}

	// For AWS SDK v2, a 304 Not Modified may be mapped to a different error code or just an error
	// Typically represented by HTTP status or custom Smithy Error
	// The standard behavior for S3 API is to return a 304 Not Modified, which the SDK translates to an error.

	// 7. GetObject Range
	getRangeRes, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  aws.String("bytes=0-4"),
	})
	if err != nil {
		t.Fatalf("GetObject Range failed: %v", err)
	}
	rangeBody, _ := io.ReadAll(getRangeRes.Body)
	getRangeRes.Body.Close()
	if string(rangeBody) != "hello" { // Length of "hello" is 5 bytes (0,1,2,3,4)
		t.Fatalf("GetObject range mismatch: got %v", string(rangeBody))
	}

	// 8. ListObjectsV2
	listRes, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatalf("ListObjectsV2 failed: %v", err)
	}
	if *listRes.KeyCount != 1 || *listRes.Contents[0].Key != key {
		t.Fatalf("ListObjectsV2 expected %s, got %v", key, listRes.Contents)
	}

	// 9. DeleteObject
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("DeleteObject failed: %v", err)
	}

	// 10. Verify Delete
	_, err = client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		t.Fatalf("Expected Object not found after delete")
	}
	if errors.As(err, &apiErr) {
		if apiErr.ErrorCode() != "NoSuchKey" {
			t.Fatalf("Expected NoSuchKey error, got: %s", apiErr.ErrorCode())
		}
	}
}

// Ensure multipart upload works with standard AWS SDK tools
func TestAWSSDKv2MultipartSetup(t *testing.T) {
	tmp := t.TempDir()
	users := []appconfig.User{{Name: "int", AccessKey: "integration-key", SecretKey: "integration-secret", Policy: auth.PolicyAdmin}}
	h := New(storage.New(tmp), auth.NewSigV4Authenticator(users))
	ts := httptest.NewServer(h)
	defer ts.Close()

	client := newTestS3Client(t, ts.URL, "integration-key", "integration-secret")
	ctx := context.Background()
	bucket := "mp-bucket"

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatalf("CreateBucket failed: %v", err)
	}

	key := "big-file.bin"
	createRes, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload failed: %v", err)
	}
	uploadId := *createRes.UploadId

	part1Res, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadId),
		PartNumber: aws.Int32(1),
		Body:       bytes.NewReader([]byte("part 1 data ")),
	})
	if err != nil {
		t.Fatalf("UploadPart 1 failed: %v", err)
	}

	part2Res, err := client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadId),
		PartNumber: aws.Int32(2),
		Body:       bytes.NewReader([]byte("and part 2 data")),
	})
	if err != nil {
		t.Fatalf("UploadPart 2 failed: %v", err)
	}

	_, err = client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadId),
		MultipartUpload: &s3types.CompletedMultipartUpload{
			Parts: []s3types.CompletedPart{
				{ETag: part1Res.ETag, PartNumber: aws.Int32(1)},
				{ETag: part2Res.ETag, PartNumber: aws.Int32(2)},
			},
		},
	})
	if err != nil {
		t.Fatalf("CompleteMultipartUpload failed: %v", err)
	}

	getRes, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("GetObject after MP failed: %v", err)
	}
	body, _ := io.ReadAll(getRes.Body)
	getRes.Body.Close()
	
	if string(body) != "part 1 data and part 2 data" {
		t.Fatalf("MP get mismatch: %s", string(body))
	}
}

func TestIAMPolicyEnforcement(t *testing.T) {
	tmp := t.TempDir()
	admin := appconfig.User{Name: "admin", AccessKey: "admin-key", SecretKey: "admin-secret", Policy: auth.PolicyAdmin}
	reader := appconfig.User{Name: "reader", AccessKey: "read-key", SecretKey: "read-secret", Policy: auth.PolicyReadOnly}
	
	h := New(storage.New(tmp), auth.NewSigV4Authenticator([]appconfig.User{admin, reader}))
	ts := httptest.NewServer(h)
	defer ts.Close()

	adminClient := newTestS3Client(t, ts.URL, "admin-key", "admin-secret")
	readClient := newTestS3Client(t, ts.URL, "read-key", "read-secret")
	ctx := context.Background()
	bucket := "iam-bucket"

	// Admin should successfully create
	_, err := adminClient.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatalf("Admin create bucket failed: %v", err)
	}

	// Reader should fail to PutObject
	_, err = readClient.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String("forbidden.txt"),
		Body:   strings.NewReader("bad"),
	})
	if err == nil {
		t.Fatal("Expected Reader to fail PutObject")
	}

	// But Admin should succeed
	_, err = adminClient.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String("allowed.txt"),
		Body:   strings.NewReader("good"),
	})
	if err != nil {
		t.Fatalf("Admin put object failed: %v", err)
	}

	// Reader should succeed to GetObject
	getRes, err := readClient.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String("allowed.txt"),
	})
	if err != nil {
		t.Fatalf("Reader get object failed: %v", err)
	}
	defer getRes.Body.Close()
}

