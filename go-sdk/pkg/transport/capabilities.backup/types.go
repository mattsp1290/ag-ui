package capabilities

// CompressionType represents supported compression algorithms.
type CompressionType string

const (
	CompressionNone   CompressionType = "none"
	CompressionGzip   CompressionType = "gzip"
	CompressionZstd   CompressionType = "zstd"
	CompressionSnappy CompressionType = "snappy"
	CompressionBrotli CompressionType = "brotli"
)

// SecurityFeature represents supported security features.
type SecurityFeature string

const (
	SecurityTLS      SecurityFeature = "tls"
	SecurityMTLS     SecurityFeature = "mtls"
	SecurityJWT      SecurityFeature = "jwt"
	SecurityAPIKey   SecurityFeature = "api-key"
	SecurityOAuth2   SecurityFeature = "oauth2"
	SecurityCustom   SecurityFeature = "custom"
)