# Apache Tika Performance Tuning Guide

This guide provides comprehensive performance optimization strategies for the Apache Tika integration in the Arcadia Documents Module.

## Table of Contents

- [Overview](#overview)
- [Performance Fundamentals](#performance-fundamentals)
- [Baseline Measurement](#baseline-measurement)
- [Tika Server Optimization](#tika-server-optimization)
- [Client Configuration Tuning](#client-configuration-tuning)
- [System-Level Optimization](#system-level-optimization)
- [Network Optimization](#network-optimization)
- [Memory Management](#memory-management)
- [Scaling Strategies](#scaling-strategies)
- [Monitoring and Metrics](#monitoring-and-metrics)
- [Performance Testing](#performance-testing)
- [Troubleshooting Performance Issues](#troubleshooting-performance-issues)
- [Best Practices](#best-practices)

## Overview

The Tika integration performance depends on multiple factors:

- **Tika Server Configuration**: JVM settings, thread pools, memory allocation
- **Client Configuration**: Connection pools, timeouts, retry logic
- **Document Characteristics**: File size, complexity, format
- **System Resources**: CPU, memory, network bandwidth
- **Network Topology**: Latency, bandwidth, reliability

### Performance Goals

| Metric | Target | Optimization Priority |
|--------|--------|----------------------|
| **Small Documents** (< 1MB) | 200+ docs/min | High |
| **Medium Documents** (1-10MB) | 50+ docs/min | Medium |
| **Large Documents** (10-100MB) | 10+ docs/min | Low |
| **Processing Latency** (95th percentile) | < 5 seconds | High |
| **Memory Usage** | < 8GB per Tika instance | Medium |
| **Error Rate** | < 1% | High |

## Performance Fundamentals

### Processing Pipeline

Understanding the processing pipeline helps identify bottlenecks:

```
Document Input → File Validation → Network Transfer → Tika Processing → Response Transfer → Result Processing
     ↓              ↓                  ↓                ↓                 ↓                ↓
   I/O Time      CPU Time         Network Time      CPU + Memory      Network Time     CPU Time
   (1-5ms)       (1-10ms)         (10-50ms)        (100ms-5s)        (10-50ms)       (1-10ms)
```

### Bottleneck Identification

Common bottlenecks and their symptoms:

| Bottleneck | Symptoms | Solutions |
|------------|----------|-----------|
| **CPU** | High CPU usage, slow processing | Scale servers, optimize JVM |
| **Memory** | High memory usage, GC pressure | Increase heap, optimize GC |
| **Network** | High latency, timeouts | Optimize connections, local deployment |
| **I/O** | Slow file access | SSD storage, file caching |
| **Tika Capacity** | Queue buildup, circuit breaker open | Scale Tika instances |

## Baseline Measurement

### Establishing Performance Baseline

Before optimization, establish current performance metrics:

```bash
#!/bin/bash
# scripts/performance-baseline.sh

echo "=== Tika Performance Baseline ==="

# 1. Single document processing
echo "Testing single document processing..."
for size in small medium large; do
    echo "Processing $size document..."
    time arcadia-docs process-file "test-data/$size-document.docx"
done

# 2. Batch processing
echo "Testing batch processing..."
time arcadia-docs process-batch test-data/office-docs/

# 3. Concurrent processing
echo "Testing concurrent processing..."
for i in {1..10}; do
    arcadia-docs process-file "test-data/medium-document.docx" &
done
wait

# 4. Resource usage
echo "Current resource usage:"
docker stats --no-stream tika
free -h
```

### Performance Metrics Collection

```go
// metrics/performance-collector.go
package metrics

import (
    "context"
    "time"
)

type PerformanceMetrics struct {
    // Processing metrics
    DocumentsPerMinute    float64
    AverageProcessingTime time.Duration
    P95ProcessingTime     time.Duration
    P99ProcessingTime     time.Duration

    // Resource metrics
    CPUUsage        float64
    MemoryUsage     int64
    NetworkLatency  time.Duration
    DiskIOWait      float64

    // Error metrics
    ErrorRate       float64
    TimeoutRate     float64
    CircuitBreaker  string
}

func CollectPerformanceMetrics(ctx context.Context) *PerformanceMetrics {
    // Implementation for collecting comprehensive metrics
    return &PerformanceMetrics{
        // ... metric collection logic
    }
}
```

### Benchmark Test Suite

```go
// benchmark/tika_test.go
package benchmark

import (
    "context"
    "testing"
    "time"
)

func BenchmarkTikaProcessing(b *testing.B) {
    ctx := context.Background()

    testCases := []struct {
        name     string
        filePath string
        fileSize int64
    }{
        {"Small_DOCX", "test-data/small.docx", 50 * 1024},
        {"Medium_DOCX", "test-data/medium.docx", 1024 * 1024},
        {"Large_DOCX", "test-data/large.docx", 10 * 1024 * 1024},
        {"Complex_XLSX", "test-data/complex.xlsx", 5 * 1024 * 1024},
        {"Image_Heavy_PPTX", "test-data/presentation.pptx", 20 * 1024 * 1024},
    }

    for _, tc := range testCases {
        b.Run(tc.name, func(b *testing.B) {
            b.ResetTimer()
            b.SetBytes(tc.fileSize)

            for i := 0; i < b.N; i++ {
                _, err := processor.Process(ctx, tc.filePath)
                if err != nil {
                    b.Fatalf("Processing failed: %v", err)
                }
            }
        })
    }
}

func BenchmarkConcurrentProcessing(b *testing.B) {
    ctx := context.Background()
    concurrency := []int{1, 5, 10, 20, 50}

    for _, c := range concurrency {
        b.Run(fmt.Sprintf("Concurrency_%d", c), func(b *testing.B) {
            b.SetParallelism(c)
            b.RunParallel(func(pb *testing.PB) {
                for pb.Next() {
                    _, err := processor.Process(ctx, "test-data/medium.docx")
                    if err != nil {
                        b.Fatalf("Processing failed: %v", err)
                    }
                }
            })
        })
    }
}
```

## Tika Server Optimization

### JVM Configuration

Optimize JVM settings for your workload:

```bash
# High-performance JVM configuration
JAVA_OPTS="-Xmx8g -Xms4g \
           -XX:+UseG1GC \
           -XX:MaxGCPauseMillis=200 \
           -XX:+UseStringDeduplication \
           -XX:+OptimizeStringConcat \
           -XX:+UseCompressedOops \
           -XX:+UseCompressedClassPointers \
           -Djava.awt.headless=true \
           -Dfile.encoding=UTF-8"

# Memory-optimized configuration
JAVA_OPTS="-Xmx4g -Xms2g \
           -XX:+UseG1GC \
           -XX:+UseStringDeduplication \
           -XX:MaxRAMPercentage=75 \
           -XX:+ExitOnOutOfMemoryError"

# Low-latency configuration
JAVA_OPTS="-Xmx6g -Xms6g \
           -XX:+UseG1GC \
           -XX:MaxGCPauseMillis=50 \
           -XX:+UnlockExperimentalVMOptions \
           -XX:+UseShenandoahGC"
```

### Tika Server Configuration

Optimize Tika server settings:

```xml
<!-- tika-config-performance.xml -->
<?xml version="1.0" encoding="UTF-8"?>
<properties>
  <parsers>
    <parser class="org.apache.tika.parser.DefaultParser">
      <!-- Exclude resource-intensive parsers -->
      <parser-exclude class="org.apache.tika.parser.executable.ExecutableParser"/>
      <parser-exclude class="org.apache.tika.parser.image.ImageParser"/>
      <parser-exclude class="org.apache.tika.parser.ocr.TesseractOCRParser"/>
    </parser>
  </parsers>

  <server>
    <params>
      <!-- Connection optimization -->
      <param name="maxConnections" type="int">500</param>
      <param name="maxForwardsConnections" type="int">500</param>

      <!-- Memory limits -->
      <param name="maxFileSize" type="long">209715200</param> <!-- 200MB -->
      <param name="maxEmbeddedResources" type="int">100</param>

      <!-- Timeout optimization -->
      <param name="serverReadTimeoutMillis" type="long">300000</param> <!-- 5 min -->
      <param name="serverParseTimeoutMillis" type="long">300000</param>
      <param name="maxIdleTime" type="long">300000</param>

      <!-- Performance tuning -->
      <param name="enableUnsecureFeatures" type="boolean">false</param>
      <param name="maxDocumentLength" type="int">100000000</param>

      <!-- Thread pool optimization -->
      <param name="numParsingThreads" type="int">20</param>
      <param name="taskPulseMillis" type="long">500</param>
      <param name="taskTimeoutMillis" type="long">300000</param>
    </params>
  </server>

  <!-- Content type detection optimization -->
  <detectors>
    <detector class="org.apache.tika.detect.DefaultDetector"/>
  </detectors>

  <!-- Service loader optimization -->
  <service-loader initializableProblemHandler="ignore"/>
</properties>
```

### Docker Configuration

Optimize Docker deployment:

```yaml
# docker-compose-performance.yml
version: '3.8'

services:
  tika:
    image: apache/tika:2.9.1
    environment:
      - JAVA_OPTS=-Xmx8g -Xms4g -XX:+UseG1GC -XX:MaxGCPauseMillis=200
    volumes:
      - ./tika-config-performance.xml:/opt/tika-config.xml:ro
      - /tmp:/tmp  # Fast temporary storage
    deploy:
      resources:
        limits:
          memory: 10G
          cpus: '4.0'
        reservations:
          memory: 4G
          cpus: '2.0'
    ulimits:
      nofile:
        soft: 65536
        hard: 65536
    sysctls:
      - net.core.somaxconn=1024
      - net.ipv4.tcp_keepalive_time=600
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9998/version"]
      interval: 15s
      timeout: 5s
      retries: 3
```

## Client Configuration Tuning

### Connection Pool Optimization

```go
// High-performance client configuration
config := &tika.TikaConfig{
    ServerURL:       "http://tika-cluster:9998",
    Timeout:         45 * time.Second,  // Balanced timeout
    MaxRetries:      3,                 // Limited retries
    MaxConnections:  50,                // Large connection pool
    IdleConnTimeout: 120 * time.Second, // Long-lived connections

    // File processing optimization
    MaxFileSize: 100 * 1024 * 1024, // 100MB reasonable limit

    // Circuit breaker tuning
    CircuitBreaker: tika.CircuitBreakerConfig{
        FailureThreshold:    10,               // Tolerant threshold
        ResetTimeout:        60 * time.Second, // Quick recovery
        HalfOpenMaxRequests: 5,                // Adequate testing
    },

    // Processing mode optimization
    AcceptAllFormats:   false, // Office-only for performance
    OfficeConfidence:   0.95,  // High confidence
    FallbackConfidence: 0.3,   // Conservative fallback
}
```

### HTTP Client Tuning

```go
// Custom HTTP client for performance
func NewPerformanceHTTPClient() *http.Client {
    transport := &http.Transport{
        MaxIdleConns:        100,              // Large idle pool
        MaxIdleConnsPerHost: 50,               // High per-host limit
        IdleConnTimeout:     120 * time.Second, // Long-lived connections
        DisableKeepAlives:   false,            // Enable keep-alive
        DisableCompression:  true,             // Disable compression for binary files

        // TCP optimization
        DialContext: (&net.Dialer{
            Timeout:   5 * time.Second,  // Quick connection establishment
            KeepAlive: 30 * time.Second, // TCP keep-alive
        }).DialContext,

        // TLS optimization (if using HTTPS)
        TLSHandshakeTimeout: 5 * time.Second,
        ExpectContinueTimeout: 1 * time.Second,

        // Response header timeout
        ResponseHeaderTimeout: 10 * time.Second,
    }

    return &http.Client{
        Transport: transport,
        Timeout:   60 * time.Second, // Overall timeout
    }
}
```

### Batch Processing Optimization

```go
// Optimized batch processing
type BatchProcessor struct {
    processor   *tika.TikaProcessor
    workerCount int
    batchSize   int
    semaphore   chan struct{}
}

func NewBatchProcessor(processor *tika.TikaProcessor, workers int) *BatchProcessor {
    return &BatchProcessor{
        processor:   processor,
        workerCount: workers,
        batchSize:   100,
        semaphore:   make(chan struct{}, workers),
    }
}

func (bp *BatchProcessor) ProcessBatch(ctx context.Context, filePaths []string) error {
    // Process in parallel batches
    for i := 0; i < len(filePaths); i += bp.batchSize {
        end := i + bp.batchSize
        if end > len(filePaths) {
            end = len(filePaths)
        }

        batch := filePaths[i:end]
        if err := bp.processBatch(ctx, batch); err != nil {
            return err
        }
    }

    return nil
}

func (bp *BatchProcessor) processBatch(ctx context.Context, batch []string) error {
    var wg sync.WaitGroup
    errChan := make(chan error, len(batch))

    for _, filePath := range batch {
        wg.Add(1)
        go func(path string) {
            defer wg.Done()

            // Acquire semaphore
            bp.semaphore <- struct{}{}
            defer func() { <-bp.semaphore }()

            // Process document
            _, err := bp.processor.Process(ctx, path)
            if err != nil {
                errChan <- fmt.Errorf("failed to process %s: %w", path, err)
            }
        }(filePath)
    }

    wg.Wait()
    close(errChan)

    // Collect errors
    var errors []error
    for err := range errChan {
        errors = append(errors, err)
    }

    if len(errors) > 0 {
        return fmt.Errorf("batch processing errors: %v", errors)
    }

    return nil
}
```

## System-Level Optimization

### Operating System Tuning

```bash
#!/bin/bash
# scripts/system-optimization.sh

echo "Applying system-level optimizations..."

# 1. Increase file descriptor limits
echo "fs.file-max = 2097152" >> /etc/sysctl.conf
echo "* soft nofile 65536" >> /etc/security/limits.conf
echo "* hard nofile 65536" >> /etc/security/limits.conf

# 2. TCP/IP optimization
cat >> /etc/sysctl.conf << 'EOF'
# Network optimization
net.core.rmem_max = 134217728
net.core.wmem_max = 134217728
net.ipv4.tcp_rmem = 4096 65536 134217728
net.ipv4.tcp_wmem = 4096 65536 134217728
net.core.netdev_max_backlog = 5000
net.core.somaxconn = 1024

# Connection optimization
net.ipv4.tcp_keepalive_time = 600
net.ipv4.tcp_keepalive_intvl = 60
net.ipv4.tcp_keepalive_probes = 3
net.ipv4.tcp_fin_timeout = 30
EOF

# 3. Memory optimization
cat >> /etc/sysctl.conf << 'EOF'
# Memory optimization
vm.swappiness = 10
vm.dirty_ratio = 15
vm.dirty_background_ratio = 5
vm.vfs_cache_pressure = 50
EOF

# 4. Apply settings
sysctl -p

echo "System optimization completed"
```

### Storage Optimization

```bash
#!/bin/bash
# scripts/storage-optimization.sh

# 1. Use SSD storage for temporary files
mkdir -p /ssd/tmp
chown tika:tika /ssd/tmp
echo "TMPDIR=/ssd/tmp" >> /etc/environment

# 2. Mount with optimal options
echo "/dev/sdb1 /opt/tika-data ext4 noatime,nodiratime,data=ordered 0 2" >> /etc/fstab

# 3. Configure file system
tune2fs -o journal_data_writeback /dev/sdb1

# 4. Optimize I/O scheduler
echo noop > /sys/block/sdb/queue/scheduler
```

### Container Optimization

```yaml
# Optimized container configuration
version: '3.8'

x-common-variables: &common-variables
  JAVA_OPTS: "-Xmx8g -Xms4g -XX:+UseG1GC -XX:MaxGCPauseMillis=200"
  TMPDIR: "/tmp"

services:
  tika:
    image: apache/tika:2.9.1
    environment:
      <<: *common-variables
    volumes:
      - type: tmpfs
        target: /tmp
        tmpfs:
          size: 2G
      - ./tika-config.xml:/opt/tika-config.xml:ro
    deploy:
      resources:
        limits:
          memory: 10G
          cpus: '4.0'
    ulimits:
      nofile: 65536
      memlock: -1
    sysctls:
      - net.core.somaxconn=1024
```

## Network Optimization

### Network Topology

Optimize network placement:

```
┌─────────────────────────────────────────────────────────────┐
│                     Optimal Topology                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐    Low Latency    ┌─────────────┐         │
│  │  Arcadia    │◄──────────────────►│    Tika     │         │
│  │  Instance   │     < 1ms RTT      │   Server    │         │
│  └─────────────┘                    └─────────────┘         │
│                                                             │
│  • Same data center/availability zone                      │
│  • High-bandwidth network (10Gbps+)                        │
│  • Dedicated network segment                               │
└─────────────────────────────────────────────────────────────┘
```

### Load Balancer Configuration

```nginx
# nginx-performance.conf
upstream tika_backend {
    least_conn;  # Distribute based on active connections

    server tika-1:9998 max_fails=3 fail_timeout=30s weight=1;
    server tika-2:9998 max_fails=3 fail_timeout=30s weight=1;
    server tika-3:9998 max_fails=3 fail_timeout=30s weight=1;

    keepalive 32;  # Connection pooling
}

server {
    listen 9998;

    location / {
        proxy_pass http://tika_backend;

        # Connection optimization
        proxy_http_version 1.1;
        proxy_set_header Connection "";

        # Timeout optimization
        proxy_connect_timeout 5s;
        proxy_send_timeout 300s;
        proxy_read_timeout 300s;

        # Buffer optimization
        proxy_buffering on;
        proxy_buffer_size 64k;
        proxy_buffers 8 64k;
        proxy_busy_buffers_size 128k;

        # Health checking
        proxy_next_upstream error timeout invalid_header http_500 http_502 http_503;
    }

    # Health check endpoint
    location /health {
        access_log off;
        proxy_pass http://tika_backend/version;
        proxy_connect_timeout 2s;
        proxy_read_timeout 2s;
    }
}
```

### Connection Pooling

```go
// Advanced connection pooling
type ConnectionPool struct {
    servers []string
    pools   map[string]*http.Client
    mutex   sync.RWMutex
}

func NewConnectionPool(servers []string) *ConnectionPool {
    pools := make(map[string]*http.Client)

    for _, server := range servers {
        transport := &http.Transport{
            MaxIdleConns:        50,
            MaxIdleConnsPerHost: 25,
            IdleConnTimeout:     120 * time.Second,
            DisableKeepAlives:   false,

            DialContext: (&net.Dialer{
                Timeout:   2 * time.Second,
                KeepAlive: 30 * time.Second,
            }).DialContext,
        }

        pools[server] = &http.Client{
            Transport: transport,
            Timeout:   60 * time.Second,
        }
    }

    return &ConnectionPool{
        servers: servers,
        pools:   pools,
    }
}

func (cp *ConnectionPool) GetClient(server string) *http.Client {
    cp.mutex.RLock()
    defer cp.mutex.RUnlock()
    return cp.pools[server]
}
```

## Memory Management

### Tika Server Memory

Optimize Tika server memory usage:

```bash
# Memory-optimized JVM settings
MEMORY_OPTS="-Xmx8g -Xms4g \
             -XX:+UseG1GC \
             -XX:MaxGCPauseMillis=200 \
             -XX:+UseStringDeduplication \
             -XX:MaxMetaspaceSize=512m \
             -XX:CompressedClassSpaceSize=256m"

# Garbage collection optimization
GC_OPTS="-XX:+UnlockExperimentalVMOptions \
         -XX:+UseCGroupMemoryLimitForHeap \
         -XX:+PrintGC \
         -XX:+PrintGCDetails \
         -XX:+PrintGCTimeStamps \
         -Xloggc:/opt/logs/gc.log"

# Memory monitoring
MONITORING_OPTS="-XX:+HeapDumpOnOutOfMemoryError \
                 -XX:HeapDumpPath=/opt/dumps/ \
                 -XX:+PrintStringDeduplicationStatistics"
```

### Client Memory Management

```go
// Memory-efficient processing
type MemoryManager struct {
    maxConcurrent int
    semaphore     chan struct{}
    memoryLimit   int64
    currentUsage  int64
    mutex         sync.Mutex
}

func NewMemoryManager(maxConcurrent int, memoryLimit int64) *MemoryManager {
    return &MemoryManager{
        maxConcurrent: maxConcurrent,
        semaphore:     make(chan struct{}, maxConcurrent),
        memoryLimit:   memoryLimit,
    }
}

func (mm *MemoryManager) AcquireMemory(size int64) bool {
    mm.mutex.Lock()
    defer mm.mutex.Unlock()

    if mm.currentUsage+size > mm.memoryLimit {
        return false // Memory limit exceeded
    }

    select {
    case mm.semaphore <- struct{}{}:
        mm.currentUsage += size
        return true
    default:
        return false // Too many concurrent operations
    }
}

func (mm *MemoryManager) ReleaseMemory(size int64) {
    mm.mutex.Lock()
    mm.currentUsage -= size
    mm.mutex.Unlock()

    <-mm.semaphore
}
```

### Memory Monitoring

```bash
#!/bin/bash
# scripts/memory-monitor.sh

echo "=== Memory Usage Monitoring ==="

# 1. System memory
echo "System Memory:"
free -h

# 2. Container memory (if using Docker)
echo "Container Memory:"
docker stats --no-stream --format "table {{.Container}}\t{{.MemUsage}}\t{{.MemPerc}}"

# 3. JVM memory (Tika server)
echo "JVM Memory:"
jstat -gc $(pgrep -f tika-server) 1s 5

# 4. Memory trends
echo "Memory Trends:"
sar -r 1 5

# 5. Memory alerts
total_mem=$(free | grep '^Mem:' | awk '{print $2}')
used_mem=$(free | grep '^Mem:' | awk '{print $3}')
mem_percent=$((used_mem * 100 / total_mem))

if [ $mem_percent -gt 80 ]; then
    echo "WARNING: Memory usage is ${mem_percent}%"
fi
```

## Scaling Strategies

### Horizontal Scaling

Deploy multiple Tika instances:

```yaml
# docker-compose-scale.yml
version: '3.8'

services:
  tika:
    image: apache/tika:2.9.1
    environment:
      - JAVA_OPTS=-Xmx4g -Xms2g -XX:+UseG1GC
    volumes:
      - ./tika-config.xml:/opt/tika-config.xml:ro
    deploy:
      replicas: 5  # Scale to 5 instances
      resources:
        limits:
          memory: 5G
          cpus: '2.0'
      update_config:
        parallelism: 1
        order: start-first
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9998/version"]
      interval: 30s

  nginx:
    image: nginx:alpine
    ports:
      - "9998:9998"
    volumes:
      - ./nginx-lb.conf:/etc/nginx/nginx.conf:ro
    depends_on:
      - tika
```

### Auto-Scaling

Kubernetes HPA configuration:

```yaml
# k8s/tika-hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: tika-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: tika-server
  minReplicas: 2
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
      - type: Percent
        value: 100
        periodSeconds: 60
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 10
        periodSeconds: 60
```

### Client-Side Load Balancing

```go
// Smart load balancer
type SmartLoadBalancer struct {
    servers []ServerInfo
    mutex   sync.RWMutex
    metrics map[string]*ServerMetrics
}

type ServerInfo struct {
    URL       string
    Weight    int
    Available bool
}

type ServerMetrics struct {
    ResponseTime  time.Duration
    ActiveConns   int32
    ErrorRate     float64
    LastHealthy   time.Time
}

func (slb *SmartLoadBalancer) SelectServer() string {
    slb.mutex.RLock()
    defer slb.mutex.RUnlock()

    var bestServer string
    var bestScore float64

    for _, server := range slb.servers {
        if !server.Available {
            continue
        }

        metrics := slb.metrics[server.URL]
        if metrics == nil {
            continue
        }

        // Calculate server score (lower is better)
        score := float64(metrics.ResponseTime.Milliseconds()) *
                (1 + metrics.ErrorRate) *
                (1 + float64(metrics.ActiveConns)/100)

        if bestServer == "" || score < bestScore {
            bestServer = server.URL
            bestScore = score
        }
    }

    return bestServer
}
```

## Monitoring and Metrics

### Performance Metrics

Key metrics to monitor:

```go
// Performance metrics structure
type TikaPerformanceMetrics struct {
    // Throughput metrics
    DocumentsPerSecond    prometheus.CounterVec
    ProcessingDuration    prometheus.HistogramVec
    QueueDepth           prometheus.GaugeVec

    // Resource metrics
    MemoryUsage          prometheus.GaugeVec
    CPUUsage             prometheus.GaugeVec
    ConnectionPoolUsage  prometheus.GaugeVec

    // Error metrics
    ErrorRate            prometheus.CounterVec
    TimeoutRate          prometheus.CounterVec
    CircuitBreakerState  prometheus.GaugeVec

    // Business metrics
    DocumentSizeBytes    prometheus.HistogramVec
    DocumentTypeCounter  prometheus.CounterVec
}

func NewTikaPerformanceMetrics() *TikaPerformanceMetrics {
    return &TikaPerformanceMetrics{
        DocumentsPerSecond: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "tika_documents_processed_total",
                Help: "Total number of documents processed",
            },
            []string{"instance", "document_type", "status"},
        ),

        ProcessingDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "tika_processing_duration_seconds",
                Help:    "Document processing duration",
                Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
            },
            []string{"instance", "document_type"},
        ),

        // ... other metrics
    }
}
```

### Grafana Dashboard

```json
{
  "dashboard": {
    "title": "Tika Performance Dashboard",
    "panels": [
      {
        "title": "Documents Processed per Second",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(tika_documents_processed_total[1m])",
            "legendFormat": "{{instance}} - {{document_type}}"
          }
        ]
      },
      {
        "title": "Processing Duration Distribution",
        "type": "heatmap",
        "targets": [
          {
            "expr": "rate(tika_processing_duration_seconds_bucket[5m])",
            "format": "heatmap"
          }
        ]
      },
      {
        "title": "Resource Utilization",
        "type": "graph",
        "targets": [
          {
            "expr": "tika_memory_usage_bytes / 1024 / 1024 / 1024",
            "legendFormat": "Memory (GB)"
          },
          {
            "expr": "tika_cpu_usage_percent",
            "legendFormat": "CPU %"
          }
        ]
      },
      {
        "title": "Error Rates",
        "type": "stat",
        "targets": [
          {
            "expr": "rate(tika_errors_total[5m]) / rate(tika_requests_total[5m]) * 100",
            "legendFormat": "Error Rate %"
          }
        ]
      }
    ]
  }
}
```

### Alerting Rules

```yaml
# alerts/tika-performance.yml
groups:
  - name: tika.performance
    rules:
      - alert: TikaHighLatency
        expr: histogram_quantile(0.95, rate(tika_processing_duration_seconds_bucket[5m])) > 30
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "Tika processing latency is high"
          description: "95th percentile latency is {{ $value }}s"

      - alert: TikaLowThroughput
        expr: rate(tika_documents_processed_total[5m]) < 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Tika throughput is low"
          description: "Processing rate is {{ $value }} docs/sec"

      - alert: TikaHighMemoryUsage
        expr: tika_memory_usage_bytes / tika_memory_limit_bytes > 0.9
        for: 3m
        labels:
          severity: critical
        annotations:
          summary: "Tika memory usage is critical"
          description: "Memory usage is {{ $value | humanizePercentage }}"

      - alert: TikaHighErrorRate
        expr: rate(tika_errors_total[5m]) / rate(tika_requests_total[5m]) > 0.05
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Tika error rate is high"
          description: "Error rate is {{ $value | humanizePercentage }}"
```

## Performance Testing

### Load Testing Framework

```go
// loadtest/framework.go
package loadtest

import (
    "context"
    "fmt"
    "sync"
    "time"
)

type LoadTestConfig struct {
    Concurrency      int
    Duration         time.Duration
    RampUpDuration   time.Duration
    RequestRate      int
    TestDocuments    []string
}

type LoadTestResult struct {
    TotalRequests    int64
    SuccessfulReqs   int64
    FailedRequests   int64
    AverageLatency   time.Duration
    P95Latency       time.Duration
    P99Latency       time.Duration
    MaxLatency       time.Duration
    ThroughputRPS    float64
    ErrorRate        float64
}

func RunLoadTest(config LoadTestConfig, processor TikaProcessor) (*LoadTestResult, error) {
    ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
    defer cancel()

    results := make(chan TestResult, config.Concurrency*100)
    var wg sync.WaitGroup

    // Start workers
    for i := 0; i < config.Concurrency; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            runWorker(ctx, workerID, config, processor, results)
        }(i)
    }

    // Collect results
    go func() {
        wg.Wait()
        close(results)
    }()

    return collectResults(results), nil
}

func runWorker(ctx context.Context, workerID int, config LoadTestConfig,
               processor TikaProcessor, results chan<- TestResult) {

    ticker := time.NewTicker(time.Second / time.Duration(config.RequestRate))
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // Select random test document
            docPath := config.TestDocuments[rand.Intn(len(config.TestDocuments))]

            start := time.Now()
            _, err := processor.Process(ctx, docPath)
            duration := time.Since(start)

            results <- TestResult{
                WorkerID:   workerID,
                Duration:   duration,
                Success:    err == nil,
                Error:      err,
                Timestamp:  start,
            }
        }
    }
}
```

### Stress Testing

```bash
#!/bin/bash
# scripts/stress-test.sh

echo "=== Tika Stress Testing ==="

# Test parameters
CONCURRENT_USERS=(1 5 10 20 50 100)
TEST_DURATION="300s"  # 5 minutes
RAMP_UP="60s"

for users in "${CONCURRENT_USERS[@]}"; do
    echo "Testing with $users concurrent users..."

    # Start load test
    ./loadtest \
        --concurrency=$users \
        --duration=$TEST_DURATION \
        --ramp-up=$RAMP_UP \
        --test-docs=test-data/office-docs/ \
        --output=results/stress-test-${users}users.json

    # Wait between tests
    sleep 30

    echo "Completed test with $users users"
done

echo "Stress testing completed. Results in results/ directory."
```

### Performance Regression Testing

```go
// regression/performance_test.go
package regression

import (
    "testing"
    "time"
)

func TestPerformanceRegression(t *testing.T) {
    // Baseline performance expectations
    baselines := map[string]PerformanceBaseline{
        "small_docx": {
            MaxDuration:    2 * time.Second,
            MinThroughput: 100, // docs/minute
        },
        "medium_xlsx": {
            MaxDuration:    5 * time.Second,
            MinThroughput: 50,
        },
        "large_pptx": {
            MaxDuration:    15 * time.Second,
            MinThroughput: 10,
        },
    }

    for testName, baseline := range baselines {
        t.Run(testName, func(t *testing.T) {
            // Run performance test
            result := runPerformanceTest(testName)

            // Check duration
            if result.AverageDuration > baseline.MaxDuration {
                t.Errorf("Performance regression: average duration %v exceeds baseline %v",
                    result.AverageDuration, baseline.MaxDuration)
            }

            // Check throughput
            if result.Throughput < baseline.MinThroughput {
                t.Errorf("Performance regression: throughput %v below baseline %v",
                    result.Throughput, baseline.MinThroughput)
            }
        })
    }
}
```

## Troubleshooting Performance Issues

### Performance Debugging

```bash
#!/bin/bash
# scripts/performance-debug.sh

echo "=== Performance Debugging ==="

# 1. Check system resources
echo "System Resources:"
top -b -n1 | head -20
free -h
df -h

# 2. Check network latency
echo "Network Latency:"
ping -c 5 tika-server

# 3. Check Tika server health
echo "Tika Server Health:"
curl -w "@curl-format.txt" -o /dev/null -s "http://tika-server:9998/version"

# 4. Check JVM performance
echo "JVM Performance:"
jstat -gc $(pgrep -f tika-server)

# 5. Check connection pools
echo "Connection Pool Status:"
netstat -an | grep :9998 | wc -l

# 6. Check error rates
echo "Recent Errors:"
grep ERROR /var/log/tika/tika.log | tail -10
```

### Common Performance Issues

#### Issue 1: High Memory Usage

**Symptoms:**
- OutOfMemoryError
- Frequent GC pauses
- Slow processing

**Diagnosis:**
```bash
# Check memory usage
jstat -gc $(pgrep tika) 1s 10
jmap -histo $(pgrep tika) | head -20

# Check heap dump
jmap -dump:format=b,file=heap.hprof $(pgrep tika)
```

**Solutions:**
```bash
# Increase heap size
JAVA_OPTS="-Xmx12g -Xms6g"

# Optimize GC
JAVA_OPTS="$JAVA_OPTS -XX:+UseG1GC -XX:MaxGCPauseMillis=200"

# Limit file size
max_file_size: 50485760  # 50MB
```

#### Issue 2: High CPU Usage

**Symptoms:**
- CPU usage > 90%
- Slow processing
- High system load

**Diagnosis:**
```bash
# Check CPU usage
top -p $(pgrep tika)
perf top -p $(pgrep tika)

# Profile CPU usage
java -jar async-profiler.jar -e cpu -d 60 -f profile.html $(pgrep tika)
```

**Solutions:**
```bash
# Reduce concurrency
max_connections: 10

# Optimize JVM
JAVA_OPTS="-XX:+UseG1GC -XX:ParallelGCThreads=4"

# Scale horizontally
docker-compose up --scale tika=3
```

#### Issue 3: Network Bottlenecks

**Symptoms:**
- High network latency
- Connection timeouts
- Slow throughput

**Diagnosis:**
```bash
# Check network stats
ss -s
iftop -i eth0

# Check connection pool
netstat -an | grep :9998
```

**Solutions:**
```bash
# Optimize connection pool
max_connections: 25
idle_conn_timeout: "120s"

# Use local deployment
# Deploy Tika server on same host/subnet

# Optimize network
# Use faster network interface
# Reduce network hops
```

## Best Practices

### Configuration Best Practices

1. **Start Conservative**
   ```yaml
   # Initial production config
   tika:
     max_connections: 10
     timeout: "30s"
     max_file_size: 52428800  # 50MB
   ```

2. **Monitor and Adjust**
   ```bash
   # Monitor key metrics
   # Adjust based on actual usage
   # Gradual increases
   ```

3. **Environment-Specific Tuning**
   ```yaml
   # Development: Fast feedback
   # Staging: Match production
   # Production: Optimized for load
   ```

### Performance Best Practices

1. **Resource Allocation**
   - CPU: 2-4 cores per Tika instance
   - Memory: 4-8GB heap per instance
   - Storage: SSD for temporary files

2. **Scaling Strategy**
   - Horizontal scaling preferred
   - Load balancing for distribution
   - Auto-scaling for peak loads

3. **Monitoring Strategy**
   - Comprehensive metrics collection
   - Proactive alerting
   - Regular performance reviews

### Operational Best Practices

1. **Capacity Planning**
   ```bash
   # Calculate requirements
   # Plan for peak loads
   # Monitor growth trends
   ```

2. **Performance Testing**
   ```bash
   # Regular load testing
   # Performance regression testing
   # Stress testing
   ```

3. **Optimization Cycle**
   ```bash
   # Measure baseline
   # Identify bottlenecks
   # Apply optimizations
   # Measure improvements
   # Repeat
   ```

This comprehensive performance tuning guide provides the foundation for optimizing Apache Tika integration performance across different environments and scales.