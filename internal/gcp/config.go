package gcp

type Config struct {
	// Enabled to indicate if GCP integration is enabled
	Enabled bool
	// Base64EncodedJsonKey base 64 encoded GCP json credentials
	Base64EncodedJsonKey string
	// GCSBucketName name of bucket in GCS
	GCSBucketName string
}
