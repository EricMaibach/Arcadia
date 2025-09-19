# Apache Tika Configuration Reference

This document provides a comprehensive reference for all configuration options available for the Apache Tika integration in the Arcadia Documents Module.

## Table of Contents

- [Overview](#overview)
- [Configuration Structure](#configuration-structure)
- [Core Configuration Options](#core-configuration-options)
- [Circuit Breaker Configuration](#circuit-breaker-configuration)
- [Processing Mode Configuration](#processing-mode-configuration)
- [Performance Configuration](#performance-configuration)
- [Security Configuration](#security-configuration)
- [Environment Variables](#environment-variables)
- [Configuration Validation](#configuration-validation)
- [Default Configurations](#default-configurations)
- [Configuration Examples](#configuration-examples)
- [Migration Guide](#migration-guide)

## Overview

The Tika integration provides extensive configuration options to customize behavior for different environments and use cases. Configuration can be provided through:

- **Go code**: Direct configuration using `TikaConfig` struct
- **YAML files**: Environment-specific configuration files
- **Environment variables**: Runtime configuration overrides
- **Command line**: Development and testing overrides

### Configuration Hierarchy

Configuration is applied in the following priority order (highest to lowest):

1. **Command line arguments** (development only)
2. **Environment variables**
3. **Configuration files** (YAML/JSON)
4. **Default values**

## Configuration Structure

### Complete TikaConfig Structure

```go
type TikaConfig struct {
    // Server connection settings
    ServerURL       string        `json:"server_url" yaml:"server_url"`
    Timeout         time.Duration `json:"timeout" yaml:"timeout"`
    MaxRetries      int           `json:"max_retries" yaml:"max_retries"`
    MaxConnections  int           `json:"max_connections" yaml:"max_connections"`
    IdleConnTimeout time.Duration `json:"idle_conn_timeout" yaml:"idle_conn_timeout"`

    // File processing settings
    MaxFileSize     int64  `json:"max_file_size" yaml:"max_file_size"`
    EnableFallback  bool   `json:"enable_fallback" yaml:"enable_fallback"`

    // Processing mode settings
    AcceptAllFormats   bool     `json:"accept_all_formats" yaml:"accept_all_formats"`
    OfficeExtensions   []string `json:"office_extensions" yaml:"office_extensions"`
    FallbackConfidence float64  `json:"fallback_confidence" yaml:"fallback_confidence"`
    OfficeConfidence   float64  `json:"office_confidence" yaml:"office_confidence"`

    // Supported formats
    OfficeFormats []string `json:"office_formats" yaml:"office_formats"`

    // Circuit breaker settings
    CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker" yaml:"circuit_breaker"`
}

type CircuitBreakerConfig struct {
    FailureThreshold    int           `json:"failure_threshold" yaml:"failure_threshold"`
    ResetTimeout        time.Duration `json:"reset_timeout" yaml:"reset_timeout"`
    HalfOpenMaxRequests int           `json:"half_open_max_requests" yaml:"half_open_max_requests"`
}
```

## Core Configuration Options

### ServerURL

**Type**: `string`
**Required**: Yes
**Default**: `"http://localhost:9998"`

The base URL of the Apache Tika server.

```yaml
# Examples
server_url: "http://localhost:9998"           # Local development
server_url: "http://tika.internal:9998"       # Internal service
server_url: "https://tika.company.com:9998"   # External HTTPS
server_url: "http://tika-lb:9998"             # Load balanced
```

**Environment Variable**: `TIKA_SERVER_URL`

### Timeout

**Type**: `time.Duration`
**Required**: No
**Default**: `30 * time.Second`
**Range**: `1s` - `600s`

HTTP request timeout for Tika server communication.

```yaml
# Examples
timeout: "30s"     # Development
timeout: "60s"     # Production
timeout: "2m"      # Large files
timeout: "5m"      # Maximum recommended
```

**Environment Variable**: `TIKA_TIMEOUT`

### MaxRetries

**Type**: `int`
**Required**: No
**Default**: `3`
**Range**: `0` - `10`

Maximum number of retry attempts for failed requests.

```yaml
# Examples
max_retries: 0     # No retries (fast fail)
max_retries: 3     # Standard retries
max_retries: 5     # Production reliability
max_retries: 10    # Maximum resilience
```

**Environment Variable**: `TIKA_MAX_RETRIES`

### MaxConnections

**Type**: `int`
**Required**: No
**Default**: `10`
**Range**: `1` - `100`

Maximum number of concurrent HTTP connections to Tika server.

```yaml
# Examples
max_connections: 5     # Development
max_connections: 10    # Default
max_connections: 25    # High throughput
max_connections: 50    # Maximum load
```

**Environment Variable**: `TIKA_MAX_CONNECTIONS`

### IdleConnTimeout

**Type**: `time.Duration`
**Required**: No
**Default**: `90 * time.Second`
**Range**: `30s` - `300s`

Timeout for idle HTTP connections before they are closed.

```yaml
# Examples
idle_conn_timeout: "60s"    # Short-lived connections
idle_conn_timeout: "90s"    # Default
idle_conn_timeout: "120s"   # Long-lived connections
idle_conn_timeout: "300s"   # Maximum keep-alive
```

**Environment Variable**: `TIKA_IDLE_CONN_TIMEOUT`

### MaxFileSize

**Type**: `int64`
**Required**: No
**Default**: `100 * 1024 * 1024` (100MB)
**Range**: `1MB` - `1GB`

Maximum file size that will be processed by Tika.

```yaml
# Examples
max_file_size: 10485760      # 10MB (development)
max_file_size: 52428800      # 50MB (small files)
max_file_size: 104857600     # 100MB (default)
max_file_size: 209715200     # 200MB (large files)
max_file_size: 1073741824    # 1GB (maximum)
```

**Environment Variable**: `TIKA_MAX_FILE_SIZE`

## Circuit Breaker Configuration

### FailureThreshold

**Type**: `int`
**Required**: No
**Default**: `5`
**Range**: `1` - `50`

Number of consecutive failures before opening the circuit breaker.

```yaml
circuit_breaker:
  failure_threshold: 3     # Sensitive (fast failure detection)
  failure_threshold: 5     # Default (balanced)
  failure_threshold: 10    # Tolerant (production)
  failure_threshold: 20    # Very tolerant (unstable networks)
```

**Environment Variable**: `TIKA_CB_FAILURE_THRESHOLD`

### ResetTimeout

**Type**: `time.Duration`
**Required**: No
**Default**: `60 * time.Second`
**Range**: `10s` - `300s`

Time to wait before attempting to close an open circuit breaker.

```yaml
circuit_breaker:
  reset_timeout: "30s"     # Quick recovery
  reset_timeout: "60s"     # Default
  reset_timeout: "120s"    # Conservative recovery
  reset_timeout: "300s"    # Slow recovery
```

**Environment Variable**: `TIKA_CB_RESET_TIMEOUT`

### HalfOpenMaxRequests

**Type**: `int`
**Required**: No
**Default**: `3`
**Range**: `1` - `10`

Maximum number of requests allowed in half-open state.

```yaml
circuit_breaker:
  half_open_max_requests: 1     # Minimal testing
  half_open_max_requests: 3     # Default
  half_open_max_requests: 5     # Thorough testing
  half_open_max_requests: 10    # Extensive testing
```

**Environment Variable**: `TIKA_CB_HALF_OPEN_MAX_REQUESTS`

## Processing Mode Configuration

### AcceptAllFormats

**Type**: `bool`
**Required**: No
**Default**: `false`

Enables fallback mode for processing any file format supported by Tika.

```yaml
# Office-only mode (high confidence)
accept_all_formats: false

# Fallback mode (universal processing)
accept_all_formats: true
```

**Environment Variable**: `TIKA_ACCEPT_ALL_FORMATS` (`true`/`false`)

### OfficeConfidence

**Type**: `float64`
**Required**: No
**Default**: `0.9`
**Range**: `0.0` - `1.0`

Confidence score for known Office document formats.

```yaml
# Examples
office_confidence: 0.8     # Conservative
office_confidence: 0.9     # Default (recommended)
office_confidence: 0.95    # Very confident
office_confidence: 1.0     # Maximum confidence
```

**Environment Variable**: `TIKA_OFFICE_CONFIDENCE`

### FallbackConfidence

**Type**: `float64`
**Required**: No
**Default**: `0.3`
**Range**: `0.0` - `1.0`

Confidence score for unknown file formats in fallback mode.

```yaml
# Examples
fallback_confidence: 0.1     # Very low confidence
fallback_confidence: 0.3     # Default (conservative)
fallback_confidence: 0.5     # Medium confidence
fallback_confidence: 0.7     # High confidence
```

**Environment Variable**: `TIKA_FALLBACK_CONFIDENCE`

### OfficeExtensions

**Type**: `[]string`
**Required**: No
**Default**: See below

List of file extensions that are treated as Office documents with high confidence.

```yaml
# Default extensions
office_extensions:
  - "doc"
  - "docx"
  - "xls"
  - "xlsx"
  - "ppt"
  - "pptx"
  - "odt"
  - "ods"
  - "odp"

# Extended support
office_extensions:
  - "doc"
  - "docx"
  - "dot"
  - "dotx"
  - "xls"
  - "xlsx"
  - "xlt"
  - "xltx"
  - "ppt"
  - "pptx"
  - "pot"
  - "potx"
  - "odt"
  - "ods"
  - "odp"
  - "odg"
  - "rtf"
  - "wpd"
```

**Environment Variable**: `TIKA_OFFICE_EXTENSIONS` (comma-separated)

### OfficeFormats

**Type**: `[]string`
**Required**: No
**Default**: See `OfficeExtensions`

Legacy configuration option. Use `OfficeExtensions` instead.

## Performance Configuration

### High Throughput Configuration

```yaml
# High throughput setup
server_url: "http://tika-cluster:9998"
timeout: "45s"
max_retries: 3
max_connections: 25
idle_conn_timeout: "120s"
max_file_size: 104857600  # 100MB

accept_all_formats: false
office_confidence: 0.9
fallback_confidence: 0.3

circuit_breaker:
  failure_threshold: 8
  reset_timeout: "90s"
  half_open_max_requests: 5
```

### Low Latency Configuration

```yaml
# Low latency setup
server_url: "http://localhost:9998"
timeout: "15s"
max_retries: 1
max_connections: 5
idle_conn_timeout: "60s"
max_file_size: 52428800  # 50MB

accept_all_formats: false
office_confidence: 0.95
fallback_confidence: 0.3

circuit_breaker:
  failure_threshold: 3
  reset_timeout: "30s"
  half_open_max_requests: 2
```

### Memory Optimized Configuration

```yaml
# Memory optimized setup
server_url: "http://tika:9998"
timeout: "30s"
max_retries: 2
max_connections: 3
idle_conn_timeout: "60s"
max_file_size: 26214400  # 25MB

accept_all_formats: false
office_confidence: 0.9
fallback_confidence: 0.3

circuit_breaker:
  failure_threshold: 5
  reset_timeout: "60s"
  half_open_max_requests: 2
```

## Security Configuration

### Restricted Processing Configuration

```yaml
# Security-focused setup
server_url: "https://secure-tika.internal:9998"
timeout: "30s"
max_retries: 2
max_connections: 10
idle_conn_timeout: "60s"
max_file_size: 52428800  # 50MB limit

# Office-only mode for security
accept_all_formats: false
office_confidence: 0.95

# Limited extensions
office_extensions:
  - "docx"
  - "xlsx"
  - "pptx"
  - "odt"
  - "ods"
  - "odp"

circuit_breaker:
  failure_threshold: 5
  reset_timeout: "60s"
  half_open_max_requests: 3
```

### Paranoid Security Configuration

```yaml
# Maximum security setup
server_url: "https://isolated-tika.secure:9998"
timeout: "20s"
max_retries: 1
max_connections: 5
idle_conn_timeout: "30s"
max_file_size: 10485760  # 10MB strict limit

# Only modern Office formats
accept_all_formats: false
office_confidence: 1.0

office_extensions:
  - "docx"
  - "xlsx"
  - "pptx"

circuit_breaker:
  failure_threshold: 3
  reset_timeout: "120s"
  half_open_max_requests: 1
```

## Environment Variables

### Complete Environment Variable Reference

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `TIKA_SERVER_URL` | string | `http://localhost:9998` | Tika server URL |
| `TIKA_TIMEOUT` | duration | `30s` | Request timeout |
| `TIKA_MAX_RETRIES` | int | `3` | Maximum retries |
| `TIKA_MAX_CONNECTIONS` | int | `10` | Connection pool size |
| `TIKA_IDLE_CONN_TIMEOUT` | duration | `90s` | Idle connection timeout |
| `TIKA_MAX_FILE_SIZE` | int64 | `104857600` | Maximum file size |
| `TIKA_ACCEPT_ALL_FORMATS` | bool | `false` | Enable fallback mode |
| `TIKA_OFFICE_CONFIDENCE` | float64 | `0.9` | Office format confidence |
| `TIKA_FALLBACK_CONFIDENCE` | float64 | `0.3` | Fallback confidence |
| `TIKA_OFFICE_EXTENSIONS` | string | `doc,docx,...` | Comma-separated extensions |
| `TIKA_CB_FAILURE_THRESHOLD` | int | `5` | Circuit breaker failure threshold |
| `TIKA_CB_RESET_TIMEOUT` | duration | `60s` | Circuit breaker reset timeout |
| `TIKA_CB_HALF_OPEN_MAX_REQUESTS` | int | `3` | Half-open max requests |

### Environment Variable Examples

```bash
# Development environment
export TIKA_SERVER_URL="http://localhost:9998"
export TIKA_TIMEOUT="30s"
export TIKA_MAX_FILE_SIZE="52428800"
export TIKA_ACCEPT_ALL_FORMATS="true"

# Production environment
export TIKA_SERVER_URL="http://tika-prod.internal:9998"
export TIKA_TIMEOUT="60s"
export TIKA_MAX_CONNECTIONS="25"
export TIKA_MAX_FILE_SIZE="209715200"
export TIKA_ACCEPT_ALL_FORMATS="false"
export TIKA_OFFICE_CONFIDENCE="0.95"

# High security environment
export TIKA_SERVER_URL="https://secure-tika:9998"
export TIKA_TIMEOUT="20s"
export TIKA_MAX_FILE_SIZE="10485760"
export TIKA_ACCEPT_ALL_FORMATS="false"
export TIKA_OFFICE_EXTENSIONS="docx,xlsx,pptx"
```

## Configuration Validation

### Validation Rules

The configuration system validates all parameters according to these rules:

```go
// Validation rules implemented in TikaConfig.Validate()
func (c *TikaConfig) Validate() error {
    // Server URL validation
    if c.ServerURL == "" {
        return ErrInvalidConfig{"server_url cannot be empty"}
    }

    // Timeout validation
    if c.Timeout <= 0 {
        return ErrInvalidConfig{"timeout must be positive"}
    }
    if c.Timeout > 10*time.Minute {
        return ErrInvalidConfig{"timeout cannot exceed 10 minutes"}
    }

    // Retry validation
    if c.MaxRetries < 0 {
        return ErrInvalidConfig{"max_retries cannot be negative"}
    }
    if c.MaxRetries > 10 {
        return ErrInvalidConfig{"max_retries cannot exceed 10"}
    }

    // File size validation
    if c.MaxFileSize <= 0 {
        return ErrInvalidConfig{"max_file_size must be positive"}
    }
    if c.MaxFileSize > 1024*1024*1024 { // 1GB
        return ErrInvalidConfig{"max_file_size cannot exceed 1GB"}
    }

    // Connection validation
    if c.MaxConnections <= 0 {
        return ErrInvalidConfig{"max_connections must be positive"}
    }
    if c.MaxConnections > 100 {
        return ErrInvalidConfig{"max_connections cannot exceed 100"}
    }

    // Confidence validation
    if c.FallbackConfidence < 0 || c.FallbackConfidence > 1 {
        return ErrInvalidConfig{"fallback_confidence must be between 0 and 1"}
    }
    if c.OfficeConfidence < 0 || c.OfficeConfidence > 1 {
        return ErrInvalidConfig{"office_confidence must be between 0 and 1"}
    }

    // Circuit breaker validation
    if c.CircuitBreaker.FailureThreshold <= 0 {
        return ErrInvalidConfig{"circuit_breaker.failure_threshold must be positive"}
    }
    if c.CircuitBreaker.ResetTimeout <= 0 {
        return ErrInvalidConfig{"circuit_breaker.reset_timeout must be positive"}
    }
    if c.CircuitBreaker.HalfOpenMaxRequests <= 0 {
        return ErrInvalidConfig{"circuit_breaker.half_open_max_requests must be positive"}
    }

    return nil
}
```

### Configuration Testing

```go
// Test configuration validity
func TestConfiguration() {
    config := &tika.TikaConfig{
        ServerURL:          "http://localhost:9998",
        Timeout:            30 * time.Second,
        MaxRetries:         3,
        MaxFileSize:        100 * 1024 * 1024,
        MaxConnections:     10,
        AcceptAllFormats:   false,
        OfficeConfidence:   0.9,
        FallbackConfidence: 0.3,
        CircuitBreaker: tika.CircuitBreakerConfig{
            FailureThreshold:    5,
            ResetTimeout:        60 * time.Second,
            HalfOpenMaxRequests: 3,
        },
    }

    if err := config.Validate(); err != nil {
        log.Fatalf("Configuration validation failed: %v", err)
    }

    fmt.Println("Configuration is valid")
}
```

## Default Configurations

### Development Default

```go
func DefaultDevelopmentConfig() *TikaConfig {
    return &TikaConfig{
        ServerURL:       "http://localhost:9998",
        Timeout:         30 * time.Second,
        MaxRetries:      2,
        MaxFileSize:     50 * 1024 * 1024,
        MaxConnections:  5,
        IdleConnTimeout: 60 * time.Second,

        AcceptAllFormats:   true,  // Enable for testing
        OfficeConfidence:   0.9,
        FallbackConfidence: 0.4,

        OfficeExtensions: []string{
            "doc", "docx", "xls", "xlsx", "ppt", "pptx",
            "odt", "ods", "odp",
        },

        CircuitBreaker: CircuitBreakerConfig{
            FailureThreshold:    3,
            ResetTimeout:        30 * time.Second,
            HalfOpenMaxRequests: 2,
        },
    }
}
```

### Production Default

```go
func DefaultProductionConfig() *TikaConfig {
    return &TikaConfig{
        ServerURL:       "http://tika.internal:9998",
        Timeout:         60 * time.Second,
        MaxRetries:      5,
        MaxFileSize:     200 * 1024 * 1024,
        MaxConnections:  25,
        IdleConnTimeout: 120 * time.Second,

        AcceptAllFormats:   false, // Office-only for production
        OfficeConfidence:   0.95,
        FallbackConfidence: 0.3,

        OfficeExtensions: []string{
            "doc", "docx", "xls", "xlsx", "ppt", "pptx",
            "odt", "ods", "odp", "rtf",
        },

        CircuitBreaker: CircuitBreakerConfig{
            FailureThreshold:    8,
            ResetTimeout:        120 * time.Second,
            HalfOpenMaxRequests: 5,
        },
    }
}
```

## Configuration Examples

### Multi-Environment Configuration

```yaml
# config/environments.yaml
environments:
  development:
    tika:
      server_url: "http://localhost:9998"
      timeout: "30s"
      max_retries: 2
      max_file_size: 52428800
      accept_all_formats: true
      office_confidence: 0.9
      fallback_confidence: 0.4

  testing:
    tika:
      server_url: "http://tika-test:9998"
      timeout: "45s"
      max_retries: 3
      max_file_size: 104857600
      accept_all_formats: true
      office_confidence: 0.9
      fallback_confidence: 0.3

  staging:
    tika:
      server_url: "http://tika-staging.internal:9998"
      timeout: "60s"
      max_retries: 4
      max_file_size: 157286400
      accept_all_formats: false
      office_confidence: 0.95
      fallback_confidence: 0.3

  production:
    tika:
      server_url: "http://tika-prod.internal:9998"
      timeout: "60s"
      max_retries: 5
      max_file_size: 209715200
      max_connections: 25
      accept_all_formats: false
      office_confidence: 0.95
      fallback_confidence: 0.3
      circuit_breaker:
        failure_threshold: 8
        reset_timeout: "120s"
        half_open_max_requests: 5
```

### Docker Compose Configuration

```yaml
# docker-compose.yml
version: '3.8'
services:
  arcadia:
    image: arcadia:latest
    environment:
      - TIKA_SERVER_URL=http://tika:9998
      - TIKA_TIMEOUT=60s
      - TIKA_MAX_CONNECTIONS=15
      - TIKA_MAX_FILE_SIZE=157286400
      - TIKA_ACCEPT_ALL_FORMATS=false
    depends_on:
      - tika

  tika:
    image: apache/tika:latest
    ports:
      - "9998:9998"
```

### Kubernetes ConfigMap

```yaml
# k8s/tika-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tika-config
  namespace: arcadia
data:
  tika.yaml: |
    server_url: "http://tika-service:9998"
    timeout: "60s"
    max_retries: 5
    max_file_size: 209715200
    max_connections: 25
    idle_conn_timeout: "120s"

    accept_all_formats: false
    office_confidence: 0.95
    fallback_confidence: 0.3

    office_extensions:
      - "doc"
      - "docx"
      - "xls"
      - "xlsx"
      - "ppt"
      - "pptx"
      - "odt"
      - "ods"
      - "odp"

    circuit_breaker:
      failure_threshold: 8
      reset_timeout: "120s"
      half_open_max_requests: 5
```

## Migration Guide

### From Non-Tika Configuration

If migrating from a setup without Tika, add the following to your configuration:

```yaml
# Add to existing config
tika:
  server_url: "http://localhost:9998"
  timeout: "30s"
  max_retries: 3
  max_file_size: 104857600
  accept_all_formats: false  # Start with Office-only
  office_confidence: 0.9
```

### Configuration Version Migration

#### From v1.0 to v1.1

```yaml
# Old configuration (v1.0)
tika:
  server_url: "http://localhost:9998"
  timeout: "30s"
  fallback_enabled: true  # DEPRECATED

# New configuration (v1.1)
tika:
  server_url: "http://localhost:9998"
  timeout: "30s"
  accept_all_formats: true  # NEW: replaces fallback_enabled
```

#### From v1.1 to v1.2

```yaml
# Old configuration (v1.1)
tika:
  circuit_breaker_enabled: true  # DEPRECATED
  circuit_breaker_threshold: 5   # DEPRECATED

# New configuration (v1.2)
tika:
  circuit_breaker:  # NEW: structured configuration
    failure_threshold: 5
    reset_timeout: "60s"
    half_open_max_requests: 3
```

### Validation After Migration

```go
// Validate migrated configuration
func ValidateMigratedConfig(config *TikaConfig) error {
    // Run standard validation
    if err := config.Validate(); err != nil {
        return fmt.Errorf("basic validation failed: %w", err)
    }

    // Additional migration-specific checks
    if config.ServerURL == "http://old-tika:9998" {
        return fmt.Errorf("old server URL detected, please update")
    }

    if len(config.OfficeExtensions) == 0 {
        return fmt.Errorf("office_extensions not configured, please add")
    }

    return nil
}
```

This comprehensive configuration reference covers all aspects of configuring the Apache Tika integration for different environments and use cases.