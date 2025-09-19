# Migration Guide to Apache Tika Integration

This guide provides step-by-step instructions for migrating existing Arcadia Documents Module deployments to enable Apache Tika integration for Office document processing.

## Table of Contents

- [Overview](#overview)
- [Pre-Migration Assessment](#pre-migration-assessment)
- [Migration Planning](#migration-planning)
- [Prerequisites](#prerequisites)
- [Migration Steps](#migration-steps)
- [Configuration Migration](#configuration-migration)
- [Testing and Validation](#testing-and-validation)
- [Rollback Procedures](#rollback-procedures)
- [Post-Migration Optimization](#post-migration-optimization)
- [Troubleshooting](#troubleshooting)
- [FAQ](#faq)

## Overview

The Apache Tika integration adds comprehensive Office document processing capabilities to the Arcadia Documents Module. This migration enables:

- **Office Document Support**: Processing of .docx, .xlsx, .pptx, and other Office formats
- **Enhanced Text Extraction**: Superior quality text extraction from Office documents
- **Metadata Extraction**: Rich metadata from Office document properties
- **Fallback Processing**: Optional universal file format support
- **Improved Reliability**: Circuit breaker protection and health monitoring

### Migration Impact

| Area | Impact | Mitigation |
|------|--------|------------|
| **Performance** | Initial overhead for Tika server | Pre-warm servers, optimize configuration |
| **Dependencies** | New Apache Tika server dependency | Deploy redundant servers, health checks |
| **Configuration** | Additional configuration parameters | Use default values, gradual customization |
| **Storage** | No impact on existing documents | Existing documents remain accessible |
| **Processing** | Enhanced processing for Office documents | Gradual rollout, processor selection |

## Pre-Migration Assessment

### Current Environment Assessment

Before beginning migration, assess your current environment:

```bash
#!/bin/bash
# scripts/pre-migration-assessment.sh

echo "=== Arcadia Documents Module Pre-Migration Assessment ==="

# 1. Check current document types
echo "Current document types in system:"
find /path/to/documents -type f | grep -E '\.(doc|docx|xls|xlsx|ppt|pptx|odt|ods|odp)$' | wc -l
echo "Office documents found"

# 2. Check current processing load
echo "Current processing statistics:"
# Add your metrics collection here

# 3. Check available resources
echo "System resources:"
free -h
df -h
nproc

# 4. Check network connectivity for Tika
echo "Network assessment:"
# This will be used later for Tika server connectivity

# 5. Current configuration review
echo "Current configuration:"
# Review existing Documents Module configuration
```

### Document Analysis

Analyze your existing document corpus:

```go
// tools/document-analysis.go
package main

import (
    "context"
    "fmt"
    "log"
    "path/filepath"
    "strings"
)

func analyzeDocuments(documentsPath string) {
    officeExtensions := map[string]int{
        ".doc":  0, ".docx": 0,
        ".xls":  0, ".xlsx": 0,
        ".ppt":  0, ".pptx": 0,
        ".odt":  0, ".ods":  0, ".odp": 0,
    }

    totalFiles := 0
    totalSize := int64(0)

    err := filepath.Walk(documentsPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        if !info.IsDir() {
            totalFiles++
            totalSize += info.Size()

            ext := strings.ToLower(filepath.Ext(path))
            if _, exists := officeExtensions[ext]; exists {
                officeExtensions[ext]++
            }
        }

        return nil
    })

    if err != nil {
        log.Printf("Error analyzing documents: %v", err)
        return
    }

    fmt.Printf("Document Analysis Results:\n")
    fmt.Printf("Total files: %d\n", totalFiles)
    fmt.Printf("Total size: %d MB\n", totalSize/(1024*1024))
    fmt.Printf("\nOffice document breakdown:\n")

    officeTotal := 0
    for ext, count := range officeExtensions {
        if count > 0 {
            fmt.Printf("  %s: %d files\n", ext, count)
            officeTotal += count
        }
    }

    fmt.Printf("\nOffice documents: %d (%.1f%% of total)\n",
        officeTotal, float64(officeTotal)/float64(totalFiles)*100)

    if officeTotal > 0 {
        fmt.Printf("\nRecommendation: Tika integration will benefit your deployment\n")
    } else {
        fmt.Printf("\nRecommendation: Consider future Office document requirements\n")
    }
}
```

## Migration Planning

### Migration Strategy Options

#### Option 1: Immediate Full Migration (Recommended for Small Deployments)

- **Timeline**: 1-2 hours
- **Downtime**: 10-30 minutes
- **Suitable for**: < 1000 documents, development/staging environments

#### Option 2: Gradual Migration (Recommended for Large Deployments)

- **Timeline**: 1-2 weeks
- **Downtime**: Minimal
- **Suitable for**: > 1000 documents, production environments

#### Option 3: Parallel Migration

- **Timeline**: 2-4 weeks
- **Downtime**: None
- **Suitable for**: Critical production environments

### Migration Timeline

```
Week 1: Planning and Preparation
├── Day 1-2: Environment assessment
├── Day 3-4: Tika server setup and testing
├── Day 5-6: Configuration preparation
└── Day 7: Migration readiness review

Week 2: Migration Execution
├── Day 1-2: Staging environment migration
├── Day 3-4: Testing and validation
├── Day 5-6: Production migration
└── Day 7: Post-migration optimization
```

## Prerequisites

### Infrastructure Requirements

1. **Apache Tika Server**
   ```bash
   # Minimum requirements
   CPU: 2 cores
   Memory: 4GB RAM
   Storage: 10GB available space
   Network: Stable connection to Arcadia instances
   ```

2. **Network Connectivity**
   ```bash
   # Test connectivity
   telnet tika-server 9998
   curl http://tika-server:9998/version
   ```

3. **Backup and Recovery**
   ```bash
   # Backup current configuration
   cp -r /etc/arcadia/documents/ /backup/pre-tika-migration/

   # Backup current document database
   mysqldump documents_db > /backup/documents_db_pre_tika.sql
   ```

### Software Prerequisites

1. **Arcadia Version Compatibility**
   - Minimum version: Documents Module v1.0.0+
   - Recommended: Latest stable version

2. **Docker (if using containerized deployment)**
   ```bash
   docker --version  # Should be 20.10+
   docker-compose --version  # Should be 1.29+
   ```

## Migration Steps

### Step 1: Deploy Apache Tika Server

#### Option A: Docker Deployment (Recommended)

```bash
# 1. Create Tika deployment directory
mkdir -p /opt/arcadia-tika
cd /opt/arcadia-tika

# 2. Create docker-compose.yml
cat > docker-compose.yml << 'EOF'
version: '3.8'
services:
  tika:
    image: apache/tika:latest
    container_name: arcadia-tika
    ports:
      - "9998:9998"
    environment:
      - JAVA_OPTS=-Xmx2g -Xms1g
    volumes:
      - ./tika-config.xml:/opt/tika-config.xml:ro
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9998/version"]
      interval: 30s
      timeout: 10s
      retries: 3
    restart: unless-stopped
EOF

# 3. Create basic Tika configuration
cat > tika-config.xml << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<properties>
  <parsers>
    <parser class="org.apache.tika.parser.DefaultParser">
      <parser-exclude class="org.apache.tika.parser.executable.ExecutableParser"/>
    </parser>
  </parsers>
  <server>
    <params>
      <param name="maxConnections" type="int">100</param>
      <param name="maxFileSize" type="long">104857600</param>
    </params>
  </server>
</properties>
EOF

# 4. Start Tika server
docker-compose up -d

# 5. Verify deployment
curl http://localhost:9998/version
```

#### Option B: Manual Installation

```bash
# 1. Download Tika server
cd /opt
wget https://archive.apache.org/dist/tika/tika-server-2.9.1.jar

# 2. Create service user
useradd -r -s /bin/false tika

# 3. Create systemd service
cat > /etc/systemd/system/tika.service << 'EOF'
[Unit]
Description=Apache Tika Server
After=network.target

[Service]
Type=simple
User=tika
ExecStart=/usr/bin/java -jar /opt/tika-server-2.9.1.jar --host=0.0.0.0 --port=9998
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# 4. Start service
systemctl daemon-reload
systemctl enable tika
systemctl start tika

# 5. Verify
curl http://localhost:9998/version
```

### Step 2: Update Arcadia Configuration

#### Minimal Configuration Update

```yaml
# config/documents.yaml - Add Tika section
tika:
  server_url: "http://localhost:9998"
  timeout: "30s"
  max_retries: 3
  max_file_size: 104857600  # 100MB
  accept_all_formats: false  # Start with Office-only mode
  office_confidence: 0.9
```

#### Production Configuration Update

```yaml
# config/documents.yaml - Production settings
tika:
  server_url: "http://tika.internal:9998"
  timeout: "60s"
  max_retries: 5
  max_file_size: 209715200  # 200MB
  max_connections: 25
  idle_conn_timeout: "120s"

  # Office-only mode for stability
  accept_all_formats: false
  office_confidence: 0.95
  fallback_confidence: 0.3

  # Production circuit breaker settings
  circuit_breaker:
    failure_threshold: 8
    reset_timeout: "120s"
    half_open_max_requests: 5
```

### Step 3: Restart Arcadia Services

#### Graceful Restart Procedure

```bash
#!/bin/bash
# scripts/graceful-restart.sh

echo "Starting graceful restart for Tika integration..."

# 1. Verify Tika server is running
if ! curl -f http://localhost:9998/version > /dev/null 2>&1; then
    echo "ERROR: Tika server is not responding"
    exit 1
fi

# 2. Create configuration backup
cp /etc/arcadia/documents.yaml /backup/documents.yaml.pre-tika

# 3. Update configuration
cp /staging/documents.yaml /etc/arcadia/documents.yaml

# 4. Validate configuration
arcadia-docs validate-config /etc/arcadia/documents.yaml
if [ $? -ne 0 ]; then
    echo "ERROR: Configuration validation failed"
    cp /backup/documents.yaml.pre-tika /etc/arcadia/documents.yaml
    exit 1
fi

# 5. Restart services
systemctl reload arcadia-documents
sleep 10

# 6. Verify service health
arcadia-docs health-check
if [ $? -eq 0 ]; then
    echo "✓ Migration successful"
else
    echo "✗ Migration failed, rolling back"
    cp /backup/documents.yaml.pre-tika /etc/arcadia/documents.yaml
    systemctl reload arcadia-documents
    exit 1
fi
```

### Step 4: Verify Integration

#### Integration Test

```go
// test/tika-integration-test.go
package test

import (
    "context"
    "testing"
    "arcadia/modules/documents"
)

func TestTikaIntegration(t *testing.T) {
    // Initialize Documents Module with Tika
    deps := documents.Dependencies{
        DB:                database,
        EmbeddingProvider: embeddingProvider,
        Logger:           logger,
        Config:           documents.DefaultDocumentsConfig(),
    }

    module, err := documents.NewDocumentsModule(deps)
    if err != nil {
        t.Fatalf("Failed to initialize module: %v", err)
    }

    err = module.Start(context.Background())
    if err != nil {
        t.Fatalf("Failed to start module: %v", err)
    }

    // Test Office document processing
    testFiles := []string{
        "test-data/sample.docx",
        "test-data/sample.xlsx",
        "test-data/sample.pptx",
    }

    for _, file := range testFiles {
        t.Run(fmt.Sprintf("Process_%s", filepath.Base(file)), func(t *testing.T) {
            result, err := module.ProcessFile(context.Background(), file)
            if err != nil {
                t.Errorf("Failed to process %s: %v", file, err)
                return
            }

            // Verify Tika was used
            processorType, ok := result.Metadata["processor_type"]
            if !ok || processorType != "tika" {
                t.Errorf("Expected Tika processor, got %v", processorType)
            }

            // Verify content extraction
            if len(result.Document.Content) == 0 {
                t.Errorf("No content extracted from %s", file)
            }

            t.Logf("✓ Successfully processed %s (%d chars)",
                file, len(result.Document.Content))
        })
    }
}
```

## Configuration Migration

### From Non-Tika Setup

If migrating from a setup without Tika support:

```yaml
# BEFORE: Basic Documents Module configuration
documents:
  chunking:
    max_chunk_size: 512
    chunk_overlap: 50
  search:
    max_document_size: 51200
  processing:
    max_workers: 4
    batch_size: 100

# AFTER: Add Tika integration
documents:
  chunking:
    max_chunk_size: 512
    chunk_overlap: 50
  search:
    max_document_size: 51200
  processing:
    max_workers: 4
    batch_size: 100

  # NEW: Tika integration
  tika:
    server_url: "http://localhost:9998"
    timeout: "30s"
    max_retries: 3
    max_file_size: 104857600
    accept_all_formats: false
    office_confidence: 0.9
```

### Environment-Specific Migration

#### Development Environment

```yaml
# development.yaml
tika:
  server_url: "http://localhost:9998"
  timeout: "30s"
  max_retries: 2
  max_file_size: 52428800  # 50MB
  accept_all_formats: true  # Enable for testing variety
  office_confidence: 0.9
  fallback_confidence: 0.4
```

#### Staging Environment

```yaml
# staging.yaml
tika:
  server_url: "http://tika-staging:9998"
  timeout: "45s"
  max_retries: 3
  max_file_size: 104857600  # 100MB
  accept_all_formats: false  # Match production
  office_confidence: 0.9
  fallback_confidence: 0.3
```

#### Production Environment

```yaml
# production.yaml
tika:
  server_url: "http://tika-prod.internal:9998"
  timeout: "60s"
  max_retries: 5
  max_file_size: 209715200  # 200MB
  max_connections: 25
  accept_all_formats: false
  office_confidence: 0.95
  circuit_breaker:
    failure_threshold: 8
    reset_timeout: "120s"
    half_open_max_requests: 5
```

### Configuration Validation

```bash
#!/bin/bash
# scripts/validate-tika-config.sh

CONFIG_FILE="$1"
if [ -z "$CONFIG_FILE" ]; then
    echo "Usage: $0 <config_file>"
    exit 1
fi

echo "Validating Tika configuration in $CONFIG_FILE"

# 1. Check required fields
required_fields=("server_url" "timeout" "max_file_size")
for field in "${required_fields[@]}"; do
    if ! grep -q "^[[:space:]]*${field}:" "$CONFIG_FILE"; then
        echo "ERROR: Missing required field: $field"
        exit 1
    fi
done

# 2. Extract server URL
server_url=$(grep "server_url:" "$CONFIG_FILE" | cut -d'"' -f2)
echo "Testing connectivity to: $server_url"

# 3. Test Tika server connectivity
if curl -f -s --max-time 10 "$server_url/version" > /dev/null; then
    echo "✓ Tika server is accessible"
else
    echo "✗ Cannot connect to Tika server at $server_url"
    exit 1
fi

# 4. Validate configuration syntax
if command -v yq >/dev/null 2>&1; then
    yq eval '.tika' "$CONFIG_FILE" > /dev/null
    if [ $? -eq 0 ]; then
        echo "✓ Configuration syntax is valid"
    else
        echo "✗ Configuration syntax error"
        exit 1
    fi
fi

echo "Configuration validation successful"
```

## Testing and Validation

### Pre-Migration Testing

```bash
#!/bin/bash
# scripts/pre-migration-test.sh

echo "=== Pre-Migration Testing ==="

# 1. Test current document processing
echo "Testing current document processing..."
arcadia-docs process-file test-data/sample.txt
if [ $? -eq 0 ]; then
    echo "✓ Current processing works"
else
    echo "✗ Current processing failed"
    exit 1
fi

# 2. Test Tika server independently
echo "Testing Tika server..."
curl -X POST \
     -H "Content-Type: application/octet-stream" \
     --data-binary @test-data/sample.docx \
     http://localhost:9998/tika

if [ $? -eq 0 ]; then
    echo "✓ Tika server works independently"
else
    echo "✗ Tika server test failed"
    exit 1
fi
```

### Post-Migration Testing

```bash
#!/bin/bash
# scripts/post-migration-test.sh

echo "=== Post-Migration Testing ==="

# 1. Test basic functionality
echo "Testing basic document processing..."
arcadia-docs process-file test-data/sample.txt
if [ $? -eq 0 ]; then
    echo "✓ Basic processing still works"
else
    echo "✗ Basic processing broken"
    exit 1
fi

# 2. Test Office document processing
office_files=("sample.docx" "sample.xlsx" "sample.pptx")
for file in "${office_files[@]}"; do
    echo "Testing $file processing..."
    result=$(arcadia-docs process-file "test-data/$file")
    if echo "$result" | grep -q "processor_type.*tika"; then
        echo "✓ $file processed with Tika"
    else
        echo "✗ $file not processed with Tika"
        exit 1
    fi
done

# 3. Test health endpoints
echo "Testing health endpoints..."
curl -f http://localhost:8080/health/documents
curl -f http://localhost:9998/version

# 4. Performance baseline
echo "Establishing performance baseline..."
time arcadia-docs process-batch test-data/office-docs/
```

### Load Testing

```bash
#!/bin/bash
# scripts/load-test.sh

echo "=== Load Testing Tika Integration ==="

# Generate test load
for i in {1..50}; do
    arcadia-docs process-file "test-data/sample.docx" &
done

# Monitor system resources
echo "Monitoring system resources..."
top -p $(pgrep -d, tika) -n 5

# Wait for completion
wait

echo "Load testing completed"
```

## Rollback Procedures

### Automatic Rollback

```bash
#!/bin/bash
# scripts/auto-rollback.sh

echo "Initiating automatic rollback..."

# 1. Stop current services
systemctl stop arcadia-documents

# 2. Restore previous configuration
if [ -f /backup/documents.yaml.pre-tika ]; then
    cp /backup/documents.yaml.pre-tika /etc/arcadia/documents.yaml
    echo "✓ Configuration restored"
else
    echo "✗ Backup configuration not found"
    exit 1
fi

# 3. Remove Tika-specific configuration
sed -i '/^[[:space:]]*tika:/,/^[[:space:]]*[^[:space:]]/d' /etc/arcadia/documents.yaml

# 4. Restart services
systemctl start arcadia-documents
sleep 10

# 5. Verify rollback
arcadia-docs health-check
if [ $? -eq 0 ]; then
    echo "✓ Rollback successful"

    # Optional: Stop Tika server
    docker-compose -f /opt/arcadia-tika/docker-compose.yml down
else
    echo "✗ Rollback failed"
    exit 1
fi
```

### Manual Rollback Steps

1. **Stop Arcadia Services**
   ```bash
   systemctl stop arcadia-documents
   ```

2. **Restore Configuration**
   ```bash
   cp /backup/documents.yaml.pre-tika /etc/arcadia/documents.yaml
   ```

3. **Remove Tika Configuration**
   ```bash
   # Edit /etc/arcadia/documents.yaml and remove tika section
   ```

4. **Restart Services**
   ```bash
   systemctl start arcadia-documents
   ```

5. **Verify Operation**
   ```bash
   arcadia-docs health-check
   arcadia-docs process-file test-data/sample.txt
   ```

### Rollback Validation

```bash
#!/bin/bash
# scripts/validate-rollback.sh

echo "Validating rollback..."

# 1. Check configuration doesn't contain Tika
if grep -q "tika:" /etc/arcadia/documents.yaml; then
    echo "✗ Tika configuration still present"
    exit 1
fi

# 2. Test basic functionality
arcadia-docs process-file test-data/sample.txt
if [ $? -eq 0 ]; then
    echo "✓ Basic functionality restored"
else
    echo "✗ Basic functionality not working"
    exit 1
fi

# 3. Verify no Tika dependencies
if netstat -an | grep :9998 | grep ESTABLISHED; then
    echo "⚠ Warning: Still connected to Tika server"
fi

echo "Rollback validation completed"
```

## Post-Migration Optimization

### Performance Tuning

After successful migration, optimize performance:

```yaml
# Optimized configuration after migration
tika:
  # Tune based on actual load
  max_connections: 15        # Start conservative
  timeout: "45s"             # Balanced timeout
  max_file_size: 157286400   # 150MB reasonable limit

  # Monitor and adjust circuit breaker
  circuit_breaker:
    failure_threshold: 5     # Start sensitive
    reset_timeout: "60s"     # Quick recovery
    half_open_max_requests: 3

# Monitor these metrics and adjust:
# - Processing latency
# - Error rates
# - Memory usage
# - Connection pool utilization
```

### Monitoring Setup

```yaml
# monitoring/tika-alerts.yml
groups:
  - name: tika
    rules:
      - alert: TikaServerDown
        expr: up{job="tika-server"} == 0
        for: 30s
        labels:
          severity: critical
        annotations:
          summary: "Tika server is down"

      - alert: TikaHighLatency
        expr: histogram_quantile(0.95, rate(tika_processing_duration_seconds_bucket[5m])) > 30
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "Tika processing latency is high"

      - alert: TikaCircuitBreakerOpen
        expr: tika_circuit_breaker_state == 2
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "Tika circuit breaker is open"
```

### Gradual Feature Enablement

```go
// gradual-enablement.go
func enableFallbackMode(config *tika.TikaConfig) {
    // Week 1: Office-only mode
    config.AcceptAllFormats = false
    config.OfficeConfidence = 0.95

    // Week 2: Enable fallback with low confidence
    config.AcceptAllFormats = true
    config.FallbackConfidence = 0.2

    // Week 3+: Increase confidence based on results
    config.FallbackConfidence = 0.3
}
```

## Troubleshooting

### Common Migration Issues

#### Issue 1: Tika Server Connection Failed

**Symptoms:**
```
Error: tika server unavailable: connection refused
```

**Solutions:**
1. Verify Tika server is running:
   ```bash
   curl http://localhost:9998/version
   docker-compose ps tika
   ```

2. Check network connectivity:
   ```bash
   telnet localhost 9998
   ```

3. Review firewall rules:
   ```bash
   iptables -L | grep 9998
   ```

#### Issue 2: Configuration Validation Failed

**Symptoms:**
```
Error: invalid tika config: server_url cannot be empty
```

**Solutions:**
1. Validate YAML syntax:
   ```bash
   yq eval . config/documents.yaml
   ```

2. Check required fields:
   ```bash
   grep -E "(server_url|timeout|max_file_size)" config/documents.yaml
   ```

#### Issue 3: High Memory Usage

**Symptoms:**
- Increased memory consumption
- OOM kills
- Slow processing

**Solutions:**
1. Reduce file size limits:
   ```yaml
   tika:
     max_file_size: 52428800  # 50MB
   ```

2. Limit connections:
   ```yaml
   tika:
     max_connections: 5
   ```

3. Monitor Tika server memory:
   ```bash
   docker stats tika
   ```

#### Issue 4: Document Processing Errors

**Symptoms:**
```
Error: failed to process document with processor tika
```

**Solutions:**
1. Check document validity:
   ```bash
   file document.docx
   ```

2. Test with Tika directly:
   ```bash
   curl -X POST --data-binary @document.docx http://localhost:9998/tika
   ```

3. Review processor logs:
   ```bash
   docker-compose logs tika
   ```

### Migration Checklist

```
□ Pre-migration assessment completed
□ Tika server deployed and tested
□ Configuration updated and validated
□ Backup created and verified
□ Migration executed successfully
□ Post-migration testing completed
□ Performance baseline established
□ Monitoring and alerting configured
□ Documentation updated
□ Team trained on new features
```

## FAQ

### Q: Can I run without Tika server temporarily?

**A:** Yes, the Documents Module will work without Tika. Office documents will either be skipped or processed by alternative processors if configured.

### Q: Will existing documents need reprocessing?

**A:** No, existing documents remain unchanged. Only new documents will use Tika processing. You can optionally reprocess existing Office documents to benefit from improved extraction.

### Q: What happens if Tika server goes down?

**A:** The circuit breaker will detect failures and prevent hanging. Office documents will be skipped until Tika server is restored. Other document types continue processing normally.

### Q: Can I migrate back from Tika?

**A:** Yes, removing the Tika configuration will disable Tika processing. Follow the rollback procedures in this guide.

### Q: How do I scale Tika for high load?

**A:** Deploy multiple Tika servers behind a load balancer and update the `server_url` to point to the load balancer.

### Q: What Office formats are supported?

**A:** Tika supports .doc, .docx, .xls, .xlsx, .ppt, .pptx, .odt, .ods, .odp, .rtf, and many others. See the format documentation for complete list.

### Q: Can I use Tika for non-Office documents?

**A:** Yes, enable fallback mode (`accept_all_formats: true`) to process any format Tika supports, including HTML, XML, and more.

This comprehensive migration guide should enable a smooth transition to Tika integration for any existing Arcadia deployment.