// Package publish provides publishing functionality for Releaser.
package publish

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"

	"github.com/oarkflow/releaser/internal/artifact"
	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/tmpl"
)

// S3Publisher publishes to AWS S3 or compatible storage
type S3Publisher struct {
	config  config.Blob
	tmplCtx *tmpl.Context
}

// NewS3Publisher creates a new S3 publisher
func NewS3Publisher(cfg config.Blob, tmplCtx *tmpl.Context) *S3Publisher {
	return &S3Publisher{
		config:  cfg,
		tmplCtx: tmplCtx,
	}
}

// Publish uploads artifacts to S3
func (p *S3Publisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")

	if accessKey == "" || secretKey == "" {
		return fmt.Errorf("AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are required")
	}

	bucket := p.config.Bucket
	if bucket == "" {
		return fmt.Errorf("S3 bucket is required")
	}

	region := p.config.Region
	if region == "" {
		region = os.Getenv("AWS_REGION")
		if region == "" {
			region = "us-east-1"
		}
	}

	endpoint := p.config.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://s3.%s.amazonaws.com", region)
	}

	log.Info("Uploading to S3", "bucket", bucket, "region", region)

	for _, a := range artifacts {
		if err := p.uploadFile(ctx, a, bucket, region, endpoint, accessKey, secretKey); err != nil {
			return fmt.Errorf("failed to upload %s: %w", a.Name, err)
		}
	}

	log.Info("Uploaded to S3")
	return nil
}

// uploadFile uploads a single file to S3 using REST API with AWS Signature v4
func (p *S3Publisher) uploadFile(ctx context.Context, a artifact.Artifact, bucket, region, endpoint, accessKey, secretKey string) error {
	log.Debug("Uploading to S3", "name", a.Name)

	file, err := os.Open(a.Path)
	if err != nil {
		return err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	// Determine object key
	key := a.Name
	if p.config.Directory != "" {
		dir, _ := p.tmplCtx.Apply(p.config.Directory)
		key = filepath.Join(dir, a.Name)
	}

	// Build request
	url := fmt.Sprintf("%s/%s/%s", endpoint, bucket, key)
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(content))
	if err != nil {
		return err
	}

	// Set headers
	now := time.Now().UTC()
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("x-amz-date", now.Format("20060102T150405Z"))
	req.Header.Set("x-amz-content-sha256", sha256Hex(content))

	if p.config.ACL != "" {
		req.Header.Set("x-amz-acl", p.config.ACL)
	}

	// Sign request
	signV4(req, accessKey, secretKey, region, "s3", content)

	// Send request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("S3 upload failed: %s", respBody)
	}

	log.Debug("Uploaded to S3", "key", key)
	return nil
}

// signV4 signs an HTTP request with AWS Signature Version 4
func signV4(req *http.Request, accessKey, secretKey, region, service string, payload []byte) {
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	// Create canonical request
	method := req.Method
	canonicalURI := req.URL.Path
	canonicalQueryString := req.URL.RawQuery

	// Signed headers
	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"

	// Canonical headers
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		req.Header.Get("Content-Type"),
		req.Host,
		req.Header.Get("x-amz-content-sha256"),
		amzDate)

	payloadHash := sha256Hex(payload)

	canonicalRequest := strings.Join([]string{
		method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	// Create string to sign
	algorithm := "AWS4-HMAC-SHA256"
	credentialScope := fmt.Sprintf("%s/%s/%s/aws4_request", dateStamp, region, service)

	stringToSign := strings.Join([]string{
		algorithm,
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	// Calculate signature
	kDate := hmacSHA256([]byte("AWS4"+secretKey), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	// Add authorization header
	authHeader := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, accessKey, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)
}

// sha256Hex returns the hex-encoded SHA256 hash
func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// hmacSHA256 computes HMAC-SHA256
func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// GCSPublisher publishes to Google Cloud Storage
type GCSPublisher struct {
	config  config.Blob
	tmplCtx *tmpl.Context
}

// NewGCSPublisher creates a new GCS publisher
func NewGCSPublisher(cfg config.Blob, tmplCtx *tmpl.Context) *GCSPublisher {
	return &GCSPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
	}
}

// Publish uploads artifacts to GCS
func (p *GCSPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	// GCS requires OAuth2 or service account authentication
	// For simplicity, we'll use the JSON API with bearer token

	token := os.Getenv("GCS_ACCESS_TOKEN")
	if token == "" {
		// Try to get token from gcloud
		return fmt.Errorf("GCS_ACCESS_TOKEN is required (run 'gcloud auth print-access-token')")
	}

	bucket := p.config.Bucket
	if bucket == "" {
		return fmt.Errorf("GCS bucket is required")
	}

	log.Info("Uploading to GCS", "bucket", bucket)

	for _, a := range artifacts {
		if err := p.uploadFile(ctx, a, bucket, token); err != nil {
			return fmt.Errorf("failed to upload %s: %w", a.Name, err)
		}
	}

	log.Info("Uploaded to GCS")
	return nil
}

// uploadFile uploads a single file to GCS
func (p *GCSPublisher) uploadFile(ctx context.Context, a artifact.Artifact, bucket, token string) error {
	log.Debug("Uploading to GCS", "name", a.Name)

	file, err := os.Open(a.Path)
	if err != nil {
		return err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	// Determine object name
	objectName := a.Name
	if p.config.Directory != "" {
		dir, _ := p.tmplCtx.Apply(p.config.Directory)
		objectName = dir + "/" + a.Name
	}

	// Upload using JSON API
	url := fmt.Sprintf("https://storage.googleapis.com/upload/storage/v1/b/%s/o?uploadType=media&name=%s",
		bucket, objectName)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(content))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(content)))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GCS upload failed: %s", respBody)
	}

	log.Debug("Uploaded to GCS", "object", objectName)
	return nil
}

// AzureBlobPublisher publishes to Azure Blob Storage
type AzureBlobPublisher struct {
	config  config.Blob
	tmplCtx *tmpl.Context
}

// NewAzureBlobPublisher creates a new Azure Blob publisher
func NewAzureBlobPublisher(cfg config.Blob, tmplCtx *tmpl.Context) *AzureBlobPublisher {
	return &AzureBlobPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
	}
}

// Publish uploads artifacts to Azure Blob Storage
func (p *AzureBlobPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	accountName := os.Getenv("AZURE_STORAGE_ACCOUNT")
	accountKey := os.Getenv("AZURE_STORAGE_KEY")

	if accountName == "" || accountKey == "" {
		return fmt.Errorf("AZURE_STORAGE_ACCOUNT and AZURE_STORAGE_KEY are required")
	}

	container := p.config.Bucket
	if container == "" {
		return fmt.Errorf("Azure container (bucket) is required")
	}

	log.Info("Uploading to Azure Blob Storage", "container", container)

	for _, a := range artifacts {
		if err := p.uploadFile(ctx, a, container, accountName, accountKey); err != nil {
			return fmt.Errorf("failed to upload %s: %w", a.Name, err)
		}
	}

	log.Info("Uploaded to Azure Blob Storage")
	return nil
}

// uploadFile uploads a single file to Azure Blob Storage
func (p *AzureBlobPublisher) uploadFile(ctx context.Context, a artifact.Artifact, container, accountName, accountKey string) error {
	log.Debug("Uploading to Azure", "name", a.Name)

	file, err := os.Open(a.Path)
	if err != nil {
		return err
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	// Determine blob name
	blobName := a.Name
	if p.config.Directory != "" {
		dir, _ := p.tmplCtx.Apply(p.config.Directory)
		blobName = dir + "/" + a.Name
	}

	// Build request URL
	url := fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s",
		accountName, container, blobName)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(content))
	if err != nil {
		return err
	}

	// Set required headers
	now := time.Now().UTC()
	req.Header.Set("x-ms-date", now.Format(http.TimeFormat))
	req.Header.Set("x-ms-version", "2020-10-02")
	req.Header.Set("x-ms-blob-type", "BlockBlob")
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(content)))

	// Sign request (simplified - production should use proper SharedKey auth)
	signAzure(req, accountName, accountKey, container, blobName)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Azure upload failed: %s", respBody)
	}

	log.Debug("Uploaded to Azure", "blob", blobName)
	return nil
}

// signAzure signs an Azure Blob Storage request
func signAzure(req *http.Request, accountName, accountKey, container, blobName string) {
	// This is a simplified version - production code should use proper SharedKey authentication
	// For now, we'll just set up the headers that would be signed

	// In a full implementation, you would:
	// 1. Build the StringToSign from headers
	// 2. HMAC-SHA256 with the decoded account key
	// 3. Base64 encode the signature
	// 4. Set Authorization header

	// For a quick working solution, users should use Azure CLI or SDK
	// This placeholder allows the structure to be in place

	canonicalizedHeaders := getCanonicalizedHeaders(req)
	canonicalizedResource := fmt.Sprintf("/%s/%s/%s", accountName, container, blobName)

	stringToSign := strings.Join([]string{
		req.Method,
		req.Header.Get("Content-Encoding"),
		req.Header.Get("Content-Language"),
		req.Header.Get("Content-Length"),
		req.Header.Get("Content-MD5"),
		req.Header.Get("Content-Type"),
		req.Header.Get("Date"),
		req.Header.Get("If-Modified-Since"),
		req.Header.Get("If-Match"),
		req.Header.Get("If-None-Match"),
		req.Header.Get("If-Unmodified-Since"),
		req.Header.Get("Range"),
		canonicalizedHeaders,
		canonicalizedResource,
	}, "\n")

	_ = stringToSign // Would be used for actual signing
}

// getCanonicalizedHeaders returns canonicalized x-ms headers
func getCanonicalizedHeaders(req *http.Request) string {
	var headers []string
	for key, values := range req.Header {
		lowerKey := strings.ToLower(key)
		if strings.HasPrefix(lowerKey, "x-ms-") {
			headers = append(headers, fmt.Sprintf("%s:%s", lowerKey, strings.Join(values, ",")))
		}
	}
	sort.Strings(headers)
	return strings.Join(headers, "\n")
}

// MinioPublisher publishes to MinIO (S3-compatible)
type MinioPublisher struct {
	config  config.Blob
	tmplCtx *tmpl.Context
}

// NewMinioPublisher creates a new MinIO publisher
func NewMinioPublisher(cfg config.Blob, tmplCtx *tmpl.Context) *MinioPublisher {
	return &MinioPublisher{
		config:  cfg,
		tmplCtx: tmplCtx,
	}
}

// Publish uploads artifacts to MinIO
func (p *MinioPublisher) Publish(ctx context.Context, artifacts []artifact.Artifact) error {
	// MinIO is S3-compatible, so we reuse S3 publisher with custom endpoint
	s3 := NewS3Publisher(p.config, p.tmplCtx)
	return s3.Publish(ctx, artifacts)
}
