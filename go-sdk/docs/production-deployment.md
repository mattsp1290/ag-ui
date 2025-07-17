# Production Deployment Guide

Comprehensive guide for deploying AG-UI Go SDK applications in production environments with enterprise-grade security, monitoring, and reliability.

## Table of Contents

- [Overview](#overview)
- [Infrastructure Requirements](#infrastructure-requirements)
- [Security Hardening](#security-hardening)
- [Configuration Management](#configuration-management)
- [Monitoring and Observability](#monitoring-and-observability)
- [High Availability Setup](#high-availability-setup)
- [Performance Optimization](#performance-optimization)
- [Backup and Recovery](#backup-and-recovery)
- [Deployment Strategies](#deployment-strategies)
- [Security Checklist](#security-checklist)

## Overview

This guide covers production deployment best practices for AG-UI Go SDK applications, ensuring security, scalability, and reliability in enterprise environments.

### Deployment Objectives

- **Security**: Comprehensive security hardening and threat protection
- **Reliability**: High availability with fault tolerance
- **Performance**: Optimized for production workloads
- **Observability**: Complete monitoring and alerting
- **Compliance**: Meet regulatory and security standards
- **Scalability**: Support for growth and load variations

## Infrastructure Requirements

### Minimum System Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| CPU | 2 cores | 4+ cores |
| RAM | 4 GB | 8+ GB |
| Storage | 20 GB SSD | 100+ GB SSD |
| Network | 1 Gbps | 10+ Gbps |
| OS | Linux kernel 4.18+ | Ubuntu 22.04 LTS, RHEL 9 |

### Recommended Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Load Balancer │    │   Web Gateway   │    │   Monitoring    │
│     (HAProxy)   │    │     (Nginx)     │    │  (Prometheus)   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
    ┌────────────────────────────┼────────────────────────────┐
    │                            │                            │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   AG-UI App 1   │    │   AG-UI App 2   │    │   AG-UI App N   │
│  (Primary Pod)  │    │ (Secondary Pod) │    │   (Scale Pod)   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
         ┌───────────────────────┼───────────────────────┐
         │                       │                       │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   PostgreSQL    │    │      Redis      │    │   Event Store   │
│   (Primary)     │    │     (Cache)     │    │   (Optional)    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Container Deployment (Docker/Kubernetes)

#### Dockerfile

```dockerfile
# Multi-stage build for security and efficiency
FROM golang:1.21-alpine AS builder

# Install security updates
RUN apk update && apk add --no-cache ca-certificates git

# Create non-root user
RUN adduser -D -s /bin/sh appuser

# Set working directory
WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build application
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always)" \
    -o ag-ui-server ./cmd/server

# Production image
FROM scratch

# Import CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy user info
COPY --from=builder /etc/passwd /etc/passwd

# Copy binary
COPY --from=builder /app/ag-ui-server /ag-ui-server

# Use non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD ["/ag-ui-server", "--health-check"]

# Run application
ENTRYPOINT ["/ag-ui-server"]
```

#### Kubernetes Deployment

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ag-ui-server
  namespace: ag-ui-production
  labels:
    app: ag-ui-server
    version: v1.0.0
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  selector:
    matchLabels:
      app: ag-ui-server
  template:
    metadata:
      labels:
        app: ag-ui-server
        version: v1.0.0
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
        prometheus.io/path: "/metrics"
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        fsGroup: 65534
      serviceAccountName: ag-ui-server
      containers:
      - name: ag-ui-server
        image: your-registry/ag-ui-server:v1.0.0
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8080
          name: http
          protocol: TCP
        - containerPort: 9090
          name: metrics
          protocol: TCP
        env:
        - name: ENVIRONMENT
          value: "production"
        - name: LOG_LEVEL
          value: "info"
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: ag-ui-secrets
              key: database-url
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: ag-ui-secrets
              key: jwt-secret
        - name: REDIS_URL
          valueFrom:
            secretKeyRef:
              name: ag-ui-secrets
              key: redis-url
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 3
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          capabilities:
            drop:
            - ALL
        volumeMounts:
        - name: tmp
          mountPath: /tmp
        - name: config
          mountPath: /etc/ag-ui
          readOnly: true
        - name: tls-certs
          mountPath: /etc/ssl/private
          readOnly: true
      volumes:
      - name: tmp
        emptyDir: {}
      - name: config
        configMap:
          name: ag-ui-config
      - name: tls-certs
        secret:
          secretName: ag-ui-tls
          defaultMode: 0400
      nodeSelector:
        node-type: worker
      tolerations:
      - key: "node-type"
        operator: "Equal"
        value: "worker"
        effect: "NoSchedule"
---
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: ag-ui-server
  namespace: ag-ui-production
  labels:
    app: ag-ui-server
spec:
  type: ClusterIP
  ports:
  - port: 80
    targetPort: 8080
    protocol: TCP
    name: http
  - port: 9090
    targetPort: 9090
    protocol: TCP
    name: metrics
  selector:
    app: ag-ui-server
---
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ag-ui-config
  namespace: ag-ui-production
data:
  config.yaml: |
    server:
      address: ":8080"
      read_timeout: "30s"
      write_timeout: "30s"
      idle_timeout: "120s"
      max_header_bytes: 1048576
    
    security:
      require_tls: true
      min_tls_version: "1.2"
      cors:
        allowed_origins:
          - "https://app.example.com"
          - "https://admin.example.com"
        allowed_methods: ["GET", "POST", "PUT", "DELETE"]
        allowed_headers: ["Authorization", "Content-Type"]
        allow_credentials: true
        max_age: "12h"
    
    monitoring:
      enabled: true
      prometheus_port: 9090
      metrics_path: "/metrics"
      otlp_endpoint: "jaeger:4317"
      trace_sample_rate: 0.1
    
    logging:
      level: "info"
      format: "json"
      output: "stdout"
```

## Security Hardening

### TLS Configuration

```yaml
# tls-secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: ag-ui-tls
  namespace: ag-ui-production
type: kubernetes.io/tls
data:
  tls.crt: <base64-encoded-certificate>
  tls.key: <base64-encoded-private-key>
```

```go
// TLS configuration in Go
func createTLSConfig() *tls.Config {
    return &tls.Config{
        MinVersion: tls.VersionTLS12,
        MaxVersion: tls.VersionTLS13,
        CipherSuites: []uint16{
            tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
            tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
            tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
            tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
        },
        PreferServerCipherSuites: true,
        CurvePreferences: []tls.CurveID{
            tls.CurveP256,
            tls.X25519,
        },
        NextProtos: []string{"h2", "http/1.1"},
    }
}
```

### Network Security

#### Ingress Controller with Security Headers

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ag-ui-ingress
  namespace: ag-ui-production
  annotations:
    kubernetes.io/ingress.class: "nginx"
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
    nginx.ingress.kubernetes.io/ssl-protocols: "TLSv1.2 TLSv1.3"
    nginx.ingress.kubernetes.io/ssl-ciphers: "ECDHE-ECDSA-AES256-GCM-SHA384,ECDHE-RSA-AES256-GCM-SHA384"
    nginx.ingress.kubernetes.io/configuration-snippet: |
      more_set_headers "X-Frame-Options: DENY";
      more_set_headers "X-Content-Type-Options: nosniff";
      more_set_headers "X-XSS-Protection: 1; mode=block";
      more_set_headers "Strict-Transport-Security: max-age=31536000; includeSubDomains";
      more_set_headers "Content-Security-Policy: default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'";
      more_set_headers "Referrer-Policy: strict-origin-when-cross-origin";
    nginx.ingress.kubernetes.io/rate-limit: "100"
    nginx.ingress.kubernetes.io/rate-limit-window: "1m"
spec:
  tls:
  - hosts:
    - api.example.com
    secretName: ag-ui-tls-secret
  rules:
  - host: api.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: ag-ui-server
            port:
              number: 80
```

#### Network Policies

```yaml
# network-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: ag-ui-network-policy
  namespace: ag-ui-production
spec:
  podSelector:
    matchLabels:
      app: ag-ui-server
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
  - from:
    - namespaceSelector:
        matchLabels:
          name: monitoring
    ports:
    - protocol: TCP
      port: 9090
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: database
    ports:
    - protocol: TCP
      port: 5432
  - to:
    - namespaceSelector:
        matchLabels:
          name: cache
    ports:
    - protocol: TCP
      port: 6379
  - to: []
    ports:
    - protocol: TCP
      port: 53
    - protocol: UDP
      port: 53
    - protocol: TCP
      port: 443
```

### Application Security

#### Authentication and Authorization

```go
// Production authentication configuration
func setupProductionAuth() *auth.Config {
    return &auth.Config{
        // JWT Configuration
        JWTConfig: &auth.JWTConfig{
            Secret:          os.Getenv("JWT_SECRET"),
            Algorithm:       "HS256",
            TokenExpiry:     15 * time.Minute,
            RefreshExpiry:   7 * 24 * time.Hour,
            Issuer:          "ag-ui-production",
            Audience:        "ag-ui-clients",
            RequiredClaims:  []string{"sub", "iat", "exp", "aud", "iss"},
        },
        
        // RBAC Configuration
        RBACConfig: &auth.RBACConfig{
            Enabled: true,
            Roles: map[string]*auth.Role{
                "admin": {
                    Name: "Administrator",
                    Permissions: []string{
                        "events:*",
                        "users:*",
                        "system:*",
                    },
                },
                "user": {
                    Name: "Regular User",
                    Permissions: []string{
                        "events:read",
                        "events:write",
                        "profile:read",
                        "profile:write",
                    },
                },
                "viewer": {
                    Name: "Read-only User",
                    Permissions: []string{
                        "events:read",
                        "profile:read",
                    },
                },
            },
        },
        
        // Rate Limiting
        RateLimit: &auth.RateLimitConfig{
            Enabled:         true,
            RequestsPerHour: 1000,
            BurstSize:       100,
            CleanupInterval: 1 * time.Hour,
        },
        
        // Session Management
        SessionConfig: &auth.SessionConfig{
            Secure:   true,
            HTTPOnly: true,
            SameSite: http.SameSiteStrictMode,
            MaxAge:   24 * time.Hour,
        },
    }
}
```

#### Input Validation and Sanitization

```go
// Production validation configuration
func setupProductionValidation() *validation.Config {
    return &validation.Config{
        StrictMode: true,
        Rules: []validation.Rule{
            // Content Security
            &validation.ContentSecurityRule{
                MaxContentLength:     10000,
                ProhibitedPatterns:   loadProhibitedPatterns(),
                SanitizeHTML:        true,
                AllowedHTMLTags:     []string{"p", "br", "strong", "em"},
                RequireContentType:  true,
            },
            
            // Input Sanitization
            &validation.InputSanitizationRule{
                StripJavaScript:     true,
                StripSQLInjection:   true,
                NormalizeUnicode:    true,
                MaxFieldLength:      1000,
                RequiredFields:      []string{"type", "content"},
            },
            
            // Rate Limiting per Event Type
            &validation.EventRateLimitRule{
                Limits: map[string]int{
                    "TEXT_MESSAGE_CONTENT": 100, // per minute
                    "TOOL_CALL_START":      10,  // per minute
                    "STATE_DELTA":          50,  // per minute
                },
            },
        },
        
        // Error Handling
        ErrorHandling: &validation.ErrorHandlingConfig{
            LogSensitiveData:    false,
            ReturnDetailedErrors: false,
            AlertOnSuspicious:   true,
            QuarantineMalicious: true,
        },
    }
}

func loadProhibitedPatterns() []string {
    return []string{
        `<script[^>]*>.*?</script>`,
        `javascript:`,
        `data:text/html`,
        `vbscript:`,
        `onload=`,
        `onerror=`,
        `onclick=`,
        // SQL injection patterns
        `(\w+)\s*(=|<|>|!=)\s*(\w+)?\s*(OR|AND)\s*(\w+)\s*(=|<|>|!=)`,
        `UNION\s+SELECT`,
        `DROP\s+TABLE`,
        `DELETE\s+FROM`,
        // Command injection patterns
        `[;&|]\s*(rm|cat|wget|curl|nc|telnet)`,
    }
}
```

## Configuration Management

### Environment-based Configuration

```go
// config/production.go
package config

import (
    "fmt"
    "os"
    "strconv"
    "time"
)

type ProductionConfig struct {
    Server     ServerConfig     `json:"server"`
    Database   DatabaseConfig   `json:"database"`
    Redis      RedisConfig      `json:"redis"`
    Security   SecurityConfig   `json:"security"`
    Monitoring MonitoringConfig `json:"monitoring"`
    Logging    LoggingConfig    `json:"logging"`
}

func LoadProductionConfig() (*ProductionConfig, error) {
    config := &ProductionConfig{
        Server: ServerConfig{
            Address:         getEnvOrDefault("SERVER_ADDRESS", ":8080"),
            ReadTimeout:     getDurationEnv("SERVER_READ_TIMEOUT", 30*time.Second),
            WriteTimeout:    getDurationEnv("SERVER_WRITE_TIMEOUT", 30*time.Second),
            IdleTimeout:     getDurationEnv("SERVER_IDLE_TIMEOUT", 120*time.Second),
            MaxHeaderBytes:  getIntEnv("SERVER_MAX_HEADER_BYTES", 1<<20),
            EnableHTTP2:     getBoolEnv("SERVER_ENABLE_HTTP2", true),
        },
        
        Database: DatabaseConfig{
            URL:             getRequiredEnv("DATABASE_URL"),
            MaxOpenConns:    getIntEnv("DATABASE_MAX_OPEN_CONNS", 25),
            MaxIdleConns:    getIntEnv("DATABASE_MAX_IDLE_CONNS", 5),
            ConnMaxLifetime: getDurationEnv("DATABASE_CONN_MAX_LIFETIME", 30*time.Minute),
            SSLMode:         getEnvOrDefault("DATABASE_SSL_MODE", "require"),
            MigrationsPath:  getEnvOrDefault("DATABASE_MIGRATIONS_PATH", "migrations"),
        },
        
        Redis: RedisConfig{
            URL:         getRequiredEnv("REDIS_URL"),
            Password:    os.Getenv("REDIS_PASSWORD"),
            MaxRetries:  getIntEnv("REDIS_MAX_RETRIES", 3),
            PoolSize:    getIntEnv("REDIS_POOL_SIZE", 10),
            IdleTimeout: getDurationEnv("REDIS_IDLE_TIMEOUT", 5*time.Minute),
        },
        
        Security: SecurityConfig{
            JWTSecret:       getRequiredEnv("JWT_SECRET"),
            EncryptionKey:   getRequiredEnv("ENCRYPTION_KEY"),
            RequireTLS:      getBoolEnv("SECURITY_REQUIRE_TLS", true),
            TrustProxy:      getBoolEnv("SECURITY_TRUST_PROXY", false),
            MaxRequestSize:  getIntEnv("SECURITY_MAX_REQUEST_SIZE", 10<<20), // 10MB
            RateLimitRPS:    getIntEnv("SECURITY_RATE_LIMIT_RPS", 100),
        },
        
        Monitoring: MonitoringConfig{
            Enabled:         getBoolEnv("MONITORING_ENABLED", true),
            PrometheusPort:  getIntEnv("MONITORING_PROMETHEUS_PORT", 9090),
            OTLPEndpoint:    getEnvOrDefault("MONITORING_OTLP_ENDPOINT", ""),
            TraceSampleRate: getFloatEnv("MONITORING_TRACE_SAMPLE_RATE", 0.1),
            MetricsPrefix:   getEnvOrDefault("MONITORING_METRICS_PREFIX", "ag_ui"),
        },
        
        Logging: LoggingConfig{
            Level:       getEnvOrDefault("LOG_LEVEL", "info"),
            Format:      getEnvOrDefault("LOG_FORMAT", "json"),
            Output:      getEnvOrDefault("LOG_OUTPUT", "stdout"),
            FileOptions: &LogFileOptions{
                Filename:   getEnvOrDefault("LOG_FILENAME", "/var/log/ag-ui/app.log"),
                MaxSize:    getIntEnv("LOG_MAX_SIZE", 100), // MB
                MaxAge:     getIntEnv("LOG_MAX_AGE", 30),   // days
                MaxBackups: getIntEnv("LOG_MAX_BACKUPS", 10),
                Compress:   getBoolEnv("LOG_COMPRESS", true),
            },
        },
    }
    
    // Validate required configuration
    if err := config.Validate(); err != nil {
        return nil, fmt.Errorf("configuration validation failed: %w", err)
    }
    
    return config, nil
}

func getRequiredEnv(key string) string {
    value := os.Getenv(key)
    if value == "" {
        panic(fmt.Sprintf("required environment variable %s is not set", key))
    }
    return value
}

func getEnvOrDefault(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
    if value := os.Getenv(key); value != "" {
        if intValue, err := strconv.Atoi(value); err == nil {
            return intValue
        }
    }
    return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
    if value := os.Getenv(key); value != "" {
        if boolValue, err := strconv.ParseBool(value); err == nil {
            return boolValue
        }
    }
    return defaultValue
}

func getFloatEnv(key string, defaultValue float64) float64 {
    if value := os.Getenv(key); value != "" {
        if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
            return floatValue
        }
    }
    return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
    if value := os.Getenv(key); value != "" {
        if duration, err := time.ParseDuration(value); err == nil {
            return duration
        }
    }
    return defaultValue
}
```

### Secrets Management

```yaml
# External Secrets Operator configuration
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: vault-secretstore
  namespace: ag-ui-production
spec:
  provider:
    vault:
      server: "https://vault.example.com"
      path: "secret"
      version: "v2"
      auth:
        kubernetes:
          mountPath: "kubernetes"
          role: "ag-ui-production"
---
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: ag-ui-secrets
  namespace: ag-ui-production
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: vault-secretstore
    kind: SecretStore
  target:
    name: ag-ui-secrets
    creationPolicy: Owner
  data:
  - secretKey: database-url
    remoteRef:
      key: ag-ui/production
      property: database_url
  - secretKey: jwt-secret
    remoteRef:
      key: ag-ui/production
      property: jwt_secret
  - secretKey: redis-url
    remoteRef:
      key: ag-ui/production
      property: redis_url
  - secretKey: encryption-key
    remoteRef:
      key: ag-ui/production
      property: encryption_key
```

## Monitoring and Observability

### Prometheus Metrics Configuration

```go
// metrics/prometheus.go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

type PrometheusMetrics struct {
    // Request metrics
    requestsTotal   *prometheus.CounterVec
    requestDuration *prometheus.HistogramVec
    requestSize     *prometheus.HistogramVec
    responseSize    *prometheus.HistogramVec
    
    // Event metrics
    eventsProcessed *prometheus.CounterVec
    eventErrors     *prometheus.CounterVec
    eventLatency    *prometheus.HistogramVec
    
    // System metrics
    goroutines      prometheus.Gauge
    memoryUsage     prometheus.Gauge
    cpuUsage        prometheus.Gauge
    
    // Business metrics
    activeUsers     prometheus.Gauge
    activeAgents    prometheus.Gauge
    messageRate     *prometheus.HistogramVec
}

func NewPrometheusMetrics() *PrometheusMetrics {
    m := &PrometheusMetrics{
        requestsTotal: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "ag_ui_requests_total",
                Help: "Total number of HTTP requests",
            },
            []string{"method", "endpoint", "status"},
        ),
        
        requestDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "ag_ui_request_duration_seconds",
                Help:    "HTTP request duration in seconds",
                Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
            },
            []string{"method", "endpoint"},
        ),
        
        eventsProcessed: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "ag_ui_events_processed_total",
                Help: "Total number of events processed",
            },
            []string{"event_type", "agent", "status"},
        ),
        
        eventLatency: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "ag_ui_event_processing_seconds",
                Help:    "Event processing time in seconds",
                Buckets: prometheus.ExponentialBuckets(0.0001, 2, 20),
            },
            []string{"event_type", "agent"},
        ),
        
        goroutines: prometheus.NewGauge(
            prometheus.GaugeOpts{
                Name: "ag_ui_goroutines",
                Help: "Number of goroutines",
            },
        ),
        
        activeUsers: prometheus.NewGauge(
            prometheus.GaugeOpts{
                Name: "ag_ui_active_users",
                Help: "Number of active users",
            },
        ),
    }
    
    // Register metrics
    prometheus.MustRegister(
        m.requestsTotal,
        m.requestDuration,
        m.eventsProcessed,
        m.eventLatency,
        m.goroutines,
        m.activeUsers,
    )
    
    return m
}

func (m *PrometheusMetrics) RecordRequest(method, endpoint, status string, duration float64) {
    m.requestsTotal.WithLabelValues(method, endpoint, status).Inc()
    m.requestDuration.WithLabelValues(method, endpoint).Observe(duration)
}

func (m *PrometheusMetrics) RecordEvent(eventType, agent, status string, duration float64) {
    m.eventsProcessed.WithLabelValues(eventType, agent, status).Inc()
    m.eventLatency.WithLabelValues(eventType, agent).Observe(duration)
}
```

### OpenTelemetry Configuration

```go
// monitoring/otel.go
package monitoring

import (
    "context"
    "fmt"
    
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func SetupOpenTelemetry(ctx context.Context, config *Config) (*sdktrace.TracerProvider, error) {
    // Create OTLP exporter
    exporter, err := otlptrace.New(ctx,
        otlptracegrpc.NewClient(
            otlptracegrpc.WithEndpoint(config.OTLPEndpoint),
            otlptracegrpc.WithInsecure(), // Use WithTLSCredentials() in production
        ),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
    }
    
    // Create resource
    res, err := resource.New(ctx,
        resource.WithAttributes(
            semconv.ServiceNameKey.String(config.ServiceName),
            semconv.ServiceVersionKey.String(config.ServiceVersion),
            semconv.DeploymentEnvironmentKey.String(config.Environment),
            semconv.ServiceInstanceIDKey.String(config.InstanceID),
        ),
        resource.WithFromEnv(),
        resource.WithProcess(),
        resource.WithOS(),
        resource.WithContainer(),
        resource.WithHost(),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create resource: %w", err)
    }
    
    // Create tracer provider
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(res),
        sdktrace.WithSampler(sdktrace.TraceIDRatioBased(config.TraceSampleRate)),
    )
    
    // Set global tracer provider
    otel.SetTracerProvider(tp)
    
    return tp, nil
}
```

### Alerting Rules

```yaml
# prometheus-rules.yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: ag-ui-alerts
  namespace: ag-ui-production
spec:
  groups:
  - name: ag-ui.rules
    interval: 30s
    rules:
    # High error rate
    - alert: HighErrorRate
      expr: |
        (
          sum(rate(ag_ui_requests_total{status=~"5.."}[5m])) /
          sum(rate(ag_ui_requests_total[5m]))
        ) * 100 > 5
      for: 2m
      labels:
        severity: critical
      annotations:
        summary: "High error rate detected"
        description: "Error rate is {{ $value }}% over the last 5 minutes"
    
    # High latency
    - alert: HighLatency
      expr: |
        histogram_quantile(0.95, 
          sum(rate(ag_ui_request_duration_seconds_bucket[5m])) by (le)
        ) > 0.5
      for: 5m
      labels:
        severity: warning
      annotations:
        summary: "High latency detected"
        description: "95th percentile latency is {{ $value }}s"
    
    # Memory usage
    - alert: HighMemoryUsage
      expr: |
        (
          container_memory_usage_bytes{pod=~"ag-ui-server-.*"} /
          container_spec_memory_limit_bytes{pod=~"ag-ui-server-.*"}
        ) * 100 > 80
      for: 10m
      labels:
        severity: warning
      annotations:
        summary: "High memory usage"
        description: "Memory usage is {{ $value }}%"
    
    # Pod restart rate
    - alert: HighPodRestartRate
      expr: |
        increase(kube_pod_container_status_restarts_total{pod=~"ag-ui-server-.*"}[15m]) > 3
      for: 0m
      labels:
        severity: critical
      annotations:
        summary: "High pod restart rate"
        description: "Pod {{ $labels.pod }} has restarted {{ $value }} times in 15 minutes"
    
    # Event processing failures
    - alert: EventProcessingFailures
      expr: |
        (
          sum(rate(ag_ui_events_processed_total{status="error"}[5m])) /
          sum(rate(ag_ui_events_processed_total[5m]))
        ) * 100 > 2
      for: 3m
      labels:
        severity: warning
      annotations:
        summary: "High event processing failure rate"
        description: "Event processing failure rate is {{ $value }}%"
```

## High Availability Setup

### Database Configuration

```yaml
# PostgreSQL with Patroni for high availability
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgresql-ha
  namespace: ag-ui-production
spec:
  serviceName: postgresql-ha
  replicas: 3
  selector:
    matchLabels:
      app: postgresql-ha
  template:
    metadata:
      labels:
        app: postgresql-ha
    spec:
      containers:
      - name: postgresql
        image: postgres:15-alpine
        env:
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgresql-secret
              key: password
        - name: POSTGRES_REPLICATION_USER
          value: replicator
        - name: POSTGRES_REPLICATION_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgresql-secret
              key: replication-password
        volumeMounts:
        - name: postgresql-data
          mountPath: /var/lib/postgresql/data
        - name: postgresql-config
          mountPath: /etc/postgresql
        resources:
          requests:
            memory: "1Gi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "1000m"
  volumeClaimTemplates:
  - metadata:
      name: postgresql-data
    spec:
      accessModes: ["ReadWriteOnce"]
      storageClassName: "fast-ssd"
      resources:
        requests:
          storage: 50Gi
```

### Redis Cluster

```yaml
# Redis Cluster for high availability
apiVersion: v1
kind: ConfigMap
metadata:
  name: redis-cluster-config
  namespace: ag-ui-production
data:
  redis.conf: |
    cluster-enabled yes
    cluster-config-file nodes.conf
    cluster-node-timeout 5000
    appendonly yes
    save 900 1
    save 300 10
    save 60 10000
    maxmemory 1gb
    maxmemory-policy allkeys-lru
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: redis-cluster
  namespace: ag-ui-production
spec:
  serviceName: redis-cluster
  replicas: 6
  selector:
    matchLabels:
      app: redis-cluster
  template:
    metadata:
      labels:
        app: redis-cluster
    spec:
      containers:
      - name: redis
        image: redis:7-alpine
        command:
        - redis-server
        - /etc/redis/redis.conf
        - --cluster-announce-hostname
        - $(hostname).redis-cluster.ag-ui-production.svc.cluster.local
        volumeMounts:
        - name: redis-data
          mountPath: /data
        - name: redis-config
          mountPath: /etc/redis
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
      volumes:
      - name: redis-config
        configMap:
          name: redis-cluster-config
  volumeClaimTemplates:
  - metadata:
      name: redis-data
    spec:
      accessModes: ["ReadWriteOnce"]
      storageClassName: "fast-ssd"
      resources:
        requests:
          storage: 10Gi
```

## Security Checklist

### Pre-deployment Security Checklist

- [ ] **TLS Configuration**
  - [ ] TLS 1.2+ enabled
  - [ ] Strong cipher suites configured
  - [ ] Certificate validity verified
  - [ ] HSTS headers enabled

- [ ] **Authentication & Authorization**
  - [ ] JWT secrets securely stored
  - [ ] Token expiration configured
  - [ ] RBAC policies defined
  - [ ] Multi-factor authentication enabled

- [ ] **Network Security**
  - [ ] Network policies configured
  - [ ] Ingress security headers set
  - [ ] Rate limiting enabled
  - [ ] CORS properly configured

- [ ] **Container Security**
  - [ ] Non-root user configured
  - [ ] Read-only root filesystem
  - [ ] Minimal base image used
  - [ ] Security contexts applied

- [ ] **Secrets Management**
  - [ ] No secrets in container images
  - [ ] Secrets encrypted at rest
  - [ ] Secret rotation configured
  - [ ] Access logging enabled

- [ ] **Input Validation**
  - [ ] All inputs validated
  - [ ] SQL injection protection
  - [ ] XSS protection enabled
  - [ ] File upload restrictions

- [ ] **Monitoring & Logging**
  - [ ] Security events logged
  - [ ] Log aggregation configured
  - [ ] Anomaly detection enabled
  - [ ] Incident response plan ready

- [ ] **Backup & Recovery**
  - [ ] Automated backups configured
  - [ ] Backup encryption enabled
  - [ ] Recovery procedures tested
  - [ ] Disaster recovery plan documented

### Post-deployment Security Verification

```bash
#!/bin/bash
# security-check.sh - Post-deployment security verification

echo "=== AG-UI Security Check ==="

# Check TLS configuration
echo "1. Checking TLS configuration..."
nmap --script ssl-enum-ciphers -p 443 api.example.com

# Check security headers
echo "2. Checking security headers..."
curl -I https://api.example.com | grep -E "(X-Frame-Options|X-Content-Type-Options|Strict-Transport-Security)"

# Check for exposed secrets
echo "3. Checking for exposed secrets..."
kubectl get secrets -n ag-ui-production -o json | jq '.items[].data | keys[]'

# Check RBAC configuration
echo "4. Checking RBAC configuration..."
kubectl auth can-i --list --as=system:serviceaccount:ag-ui-production:ag-ui-server

# Check network policies
echo "5. Checking network policies..."
kubectl get networkpolicy -n ag-ui-production

# Check pod security context
echo "6. Checking pod security context..."
kubectl get pods -n ag-ui-production -o jsonpath='{.items[*].spec.securityContext}'

# Check resource limits
echo "7. Checking resource limits..."
kubectl get pods -n ag-ui-production -o jsonpath='{.items[*].spec.containers[*].resources}'

echo "=== Security Check Complete ==="
```

This comprehensive production deployment guide provides enterprise-grade security, monitoring, and reliability patterns for AG-UI Go SDK applications in production environments.