# Apache Tika Deployment Guide

This guide provides comprehensive deployment instructions for Apache Tika integration with the Arcadia Documents Module across different environments.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Development Environment](#development-environment)
- [Docker Deployment](#docker-deployment)
- [Production Deployment](#production-deployment)
- [Kubernetes Deployment](#kubernetes-deployment)
- [Environment-Specific Configurations](#environment-specific-configurations)
- [Monitoring and Health Checks](#monitoring-and-health-checks)
- [Backup and Disaster Recovery](#backup-and-disaster-recovery)
- [Scaling Considerations](#scaling-considerations)
- [Security Best Practices](#security-best-practices)
- [Maintenance and Updates](#maintenance-and-updates)
- [Troubleshooting](#troubleshooting)

## Overview

The Apache Tika deployment provides document processing capabilities for the Arcadia Documents Module. This guide covers:

- **Development**: Local development setup with Docker
- **Staging**: Testing environment with monitoring
- **Production**: High-availability production deployment
- **Scaling**: Horizontal scaling and load balancing

### Deployment Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Load Balancer                          │
│                      (nginx/HAProxy)                           │
└─────────────────────────┬───────────────────────────────────────┘
                          │
    ┌─────────────────────┼─────────────────────┐
    │                     │                     │
    ▼                     ▼                     ▼
┌──────────┐         ┌──────────┐         ┌──────────┐
│ Arcadia  │         │ Arcadia  │         │ Arcadia  │
│Instance 1│         │Instance 2│         │Instance 3│
└─────┬────┘         └─────┬────┘         └─────┬────┘
      │                    │                    │
      └─────────┬──────────┴──────────┬─────────┘
                │                     │
                ▼                     ▼
      ┌─────────────────┐   ┌─────────────────┐
      │  Tika Server 1  │   │  Tika Server 2  │
      │   (Primary)     │   │   (Backup)      │
      └─────────────────┘   └─────────────────┘
```

## Prerequisites

### System Requirements

- **CPU**: 2+ cores (4+ recommended for production)
- **Memory**: 4GB+ (8GB+ recommended for production)
- **Storage**: 10GB+ available space
- **Network**: Stable network connection between Arcadia and Tika

### Software Requirements

- **Docker**: Version 20.10+ (for containerized deployment)
- **Java**: OpenJDK 11+ (for manual deployment)
- **Apache Tika**: Version 2.0+ (latest stable recommended)

### Network Requirements

- **Port 9998**: Default Tika server port (configurable)
- **Firewall**: Allow communication between Arcadia instances and Tika servers
- **DNS**: Proper hostname resolution for clustered deployments

## Development Environment

### Quick Start with Docker

```bash
# 1. Create development directory
mkdir arcadia-tika-dev
cd arcadia-tika-dev

# 2. Create docker-compose file
cat > docker-compose.yml << 'EOF'
version: '3.8'
services:
  tika:
    image: apache/tika:latest
    container_name: arcadia-tika-dev
    ports:
      - "9998:9998"
    environment:
      - TIKA_CONFIG_FILE=/opt/tika-config.xml
    volumes:
      - ./tika-config.xml:/opt/tika-config.xml:ro
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9998/version"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
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

# 5. Verify installation
curl http://localhost:9998/version
```

### Development Configuration

```go
// development/config.go
package development

import (
    "time"
    "arcadia/modules/documents/processors/tika"
)

func NewDevelopmentTikaConfig() *tika.TikaConfig {
    return &tika.TikaConfig{
        ServerURL:       "http://localhost:9998",
        Timeout:         30 * time.Second,
        MaxRetries:      2,
        MaxFileSize:     50 * 1024 * 1024, // 50MB
        MaxConnections:  5,
        IdleConnTimeout: 60 * time.Second,

        // Development-friendly settings
        AcceptAllFormats:   true,  // Enable fallback for testing
        OfficeConfidence:   0.9,
        FallbackConfidence: 0.4,

        CircuitBreaker: tika.CircuitBreakerConfig{
            FailureThreshold:    3,
            ResetTimeout:        30 * time.Second,
            HalfOpenMaxRequests: 2,
        },
    }
}
```

## Docker Deployment

### Basic Docker Deployment

```yaml
# docker-compose.production.yml
version: '3.8'

services:
  tika-primary:
    image: apache/tika:2.9.1
    container_name: tika-primary
    ports:
      - "9998:9998"
    environment:
      - JAVA_OPTS=-Xmx2g -Xms1g
      - TIKA_CONFIG_FILE=/opt/tika-config.xml
    volumes:
      - ./configs/tika-config.xml:/opt/tika-config.xml:ro
      - ./logs:/opt/logs
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9998/version"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 60s
    restart: unless-stopped
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "3"

  tika-backup:
    image: apache/tika:2.9.1
    container_name: tika-backup
    ports:
      - "9999:9998"
    environment:
      - JAVA_OPTS=-Xmx2g -Xms1g
      - TIKA_CONFIG_FILE=/opt/tika-config.xml
    volumes:
      - ./configs/tika-config.xml:/opt/tika-config.xml:ro
      - ./logs:/opt/logs
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9998/version"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 60s
    restart: unless-stopped
    depends_on:
      - tika-primary

  # Optional: Nginx load balancer
  nginx:
    image: nginx:alpine
    container_name: tika-loadbalancer
    ports:
      - "8080:80"
    volumes:
      - ./configs/nginx.conf:/etc/nginx/nginx.conf:ro
    depends_on:
      - tika-primary
      - tika-backup
    restart: unless-stopped
```

### Production Tika Configuration

```xml
<!-- configs/tika-config.xml -->
<?xml version="1.0" encoding="UTF-8"?>
<properties>
  <parsers>
    <parser class="org.apache.tika.parser.DefaultParser">
      <!-- Exclude potentially dangerous parsers -->
      <parser-exclude class="org.apache.tika.parser.executable.ExecutableParser"/>
      <parser-exclude class="org.apache.tika.parser.pkg.PackageParser"/>
    </parser>
  </parsers>

  <server>
    <params>
      <!-- Connection limits -->
      <param name="maxConnections" type="int">200</param>
      <param name="maxFileSize" type="long">209715200</param> <!-- 200MB -->

      <!-- Timeouts -->
      <param name="serverReadTimeoutMillis" type="long">300000</param> <!-- 5 minutes -->
      <param name="serverParseTimeoutMillis" type="long">300000</param>

      <!-- Security -->
      <param name="enableUnsecureFeatures" type="boolean">false</param>
      <param name="maxEmbeddedResources" type="int">100</param>
    </params>
  </server>

  <!-- Detector configuration -->
  <detectors>
    <detector class="org.apache.tika.detect.DefaultDetector"/>
  </detectors>

  <!-- Service loader for additional parsers -->
  <service-loader initializableProblemHandler="ignore"/>
</properties>
```

### Nginx Load Balancer Configuration

```nginx
# configs/nginx.conf
events {
    worker_connections 1024;
}

http {
    upstream tika_backend {
        server tika-primary:9998 weight=3 max_fails=3 fail_timeout=30s;
        server tika-backup:9998 weight=1 max_fails=3 fail_timeout=30s backup;
    }

    server {
        listen 80;
        server_name tika.internal;

        location / {
            proxy_pass http://tika_backend;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

            # Timeouts
            proxy_connect_timeout 5s;
            proxy_send_timeout 300s;
            proxy_read_timeout 300s;

            # Buffer settings
            proxy_buffering on;
            proxy_buffer_size 4k;
            proxy_buffers 8 4k;

            # Health check
            proxy_next_upstream error timeout invalid_header http_500 http_502 http_503;
        }

        location /health {
            access_log off;
            proxy_pass http://tika_backend/version;
            proxy_connect_timeout 2s;
            proxy_send_timeout 2s;
            proxy_read_timeout 2s;
        }
    }
}
```

### Deployment Commands

```bash
# 1. Create production directory structure
mkdir -p arcadia-tika-prod/{configs,logs,data}
cd arcadia-tika-prod

# 2. Copy configuration files
cp docker-compose.production.yml docker-compose.yml
cp configs/tika-config.xml configs/
cp configs/nginx.conf configs/

# 3. Set proper permissions
chmod 644 configs/*
chmod 755 logs data

# 4. Deploy services
docker-compose up -d

# 5. Verify deployment
docker-compose ps
curl http://localhost:8080/version  # Through load balancer
curl http://localhost:9998/version  # Direct to primary
curl http://localhost:9999/version  # Direct to backup

# 6. Check logs
docker-compose logs -f tika-primary
```

## Production Deployment

### High-Availability Setup

```yaml
# production-ha.yml
version: '3.8'

services:
  tika-1:
    image: apache/tika:2.9.1
    hostname: tika-1
    environment:
      - JAVA_OPTS=-Xmx4g -Xms2g -XX:+UseG1GC
      - TIKA_CONFIG_FILE=/opt/tika-config.xml
    volumes:
      - ./configs/tika-config-prod.xml:/opt/tika-config.xml:ro
      - /var/log/tika:/opt/logs
    networks:
      - tika-network
    deploy:
      replicas: 2
      resources:
        limits:
          memory: 5G
          cpus: '2.0'
        reservations:
          memory: 2G
          cpus: '1.0'
      restart_policy:
        condition: on-failure
        delay: 10s
        max_attempts: 3
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9998/version"]
      interval: 30s
      timeout: 10s
      retries: 3

  tika-2:
    image: apache/tika:2.9.1
    hostname: tika-2
    environment:
      - JAVA_OPTS=-Xmx4g -Xms2g -XX:+UseG1GC
      - TIKA_CONFIG_FILE=/opt/tika-config.xml
    volumes:
      - ./configs/tika-config-prod.xml:/opt/tika-config.xml:ro
      - /var/log/tika:/opt/logs
    networks:
      - tika-network
    deploy:
      replicas: 2
      resources:
        limits:
          memory: 5G
          cpus: '2.0'
        reservations:
          memory: 2G
          cpus: '1.0'

  load-balancer:
    image: haproxy:alpine
    ports:
      - "9998:9998"
      - "8404:8404"  # HAProxy stats
    volumes:
      - ./configs/haproxy.cfg:/usr/local/etc/haproxy/haproxy.cfg:ro
    networks:
      - tika-network
    depends_on:
      - tika-1
      - tika-2

networks:
  tika-network:
    driver: overlay
```

### HAProxy Configuration

```
# configs/haproxy.cfg
global
    daemon
    log stdout local0

defaults
    mode http
    timeout connect 5000ms
    timeout client 300000ms
    timeout server 300000ms
    option httplog
    log global

frontend tika_frontend
    bind *:9998
    default_backend tika_backend

backend tika_backend
    balance roundrobin
    option httpchk GET /version

    server tika-1-1 tika-1:9998 check inter 30s
    server tika-1-2 tika-1:9998 check inter 30s
    server tika-2-1 tika-2:9998 check inter 30s
    server tika-2-2 tika-2:9998 check inter 30s

frontend stats
    bind *:8404
    stats enable
    stats uri /stats
    stats refresh 10s
```

### Production Arcadia Configuration

```go
// production/config.go
package production

import (
    "time"
    "arcadia/modules/documents/processors/tika"
)

func NewProductionTikaConfig() *tika.TikaConfig {
    return &tika.TikaConfig{
        ServerURL:       "http://tika-lb.internal:9998",
        Timeout:         60 * time.Second,
        MaxRetries:      5,
        MaxFileSize:     200 * 1024 * 1024, // 200MB
        MaxConnections:  25,
        IdleConnTimeout: 120 * time.Second,

        // Conservative production settings
        AcceptAllFormats:   false, // Office-only mode
        OfficeConfidence:   0.95,
        FallbackConfidence: 0.3,

        CircuitBreaker: tika.CircuitBreakerConfig{
            FailureThreshold:    8,
            ResetTimeout:        120 * time.Second,
            HalfOpenMaxRequests: 5,
        },
    }
}
```

## Kubernetes Deployment

### Tika Deployment Manifests

```yaml
# k8s/tika-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tika-server
  namespace: arcadia
  labels:
    app: tika-server
    version: v2.9.1
spec:
  replicas: 3
  selector:
    matchLabels:
      app: tika-server
  template:
    metadata:
      labels:
        app: tika-server
        version: v2.9.1
    spec:
      containers:
      - name: tika
        image: apache/tika:2.9.1
        ports:
        - containerPort: 9998
          name: http
        env:
        - name: JAVA_OPTS
          value: "-Xmx3g -Xms1g -XX:+UseG1GC"
        - name: TIKA_CONFIG_FILE
          value: "/opt/tika-config.xml"
        volumeMounts:
        - name: tika-config
          mountPath: /opt/tika-config.xml
          subPath: tika-config.xml
          readOnly: true
        - name: logs
          mountPath: /opt/logs
        resources:
          requests:
            memory: "1Gi"
            cpu: "500m"
          limits:
            memory: "4Gi"
            cpu: "2"
        livenessProbe:
          httpGet:
            path: /version
            port: 9998
          initialDelaySeconds: 60
          periodSeconds: 30
          timeoutSeconds: 10
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /version
            port: 9998
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
      volumes:
      - name: tika-config
        configMap:
          name: tika-config
      - name: logs
        emptyDir: {}
      restartPolicy: Always
---
apiVersion: v1
kind: Service
metadata:
  name: tika-service
  namespace: arcadia
  labels:
    app: tika-server
spec:
  selector:
    app: tika-server
  ports:
  - port: 9998
    targetPort: 9998
    protocol: TCP
    name: http
  type: ClusterIP
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: tika-config
  namespace: arcadia
data:
  tika-config.xml: |
    <?xml version="1.0" encoding="UTF-8"?>
    <properties>
      <parsers>
        <parser class="org.apache.tika.parser.DefaultParser">
          <parser-exclude class="org.apache.tika.parser.executable.ExecutableParser"/>
        </parser>
      </parsers>
      <server>
        <params>
          <param name="maxConnections" type="int">300</param>
          <param name="maxFileSize" type="long">209715200</param>
          <param name="serverReadTimeoutMillis" type="long">300000</param>
          <param name="enableUnsecureFeatures" type="boolean">false</param>
        </params>
      </server>
    </properties>
```

### HorizontalPodAutoscaler

```yaml
# k8s/tika-hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: tika-server-hpa
  namespace: arcadia
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: tika-server
  minReplicas: 2
  maxReplicas: 10
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
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 10
        periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
      - type: Percent
        value: 50
        periodSeconds: 60
```

### Deployment Commands

```bash
# 1. Create namespace
kubectl create namespace arcadia

# 2. Apply configurations
kubectl apply -f k8s/tika-deployment.yaml
kubectl apply -f k8s/tika-hpa.yaml

# 3. Verify deployment
kubectl get pods -n arcadia -l app=tika-server
kubectl get svc -n arcadia
kubectl get hpa -n arcadia

# 4. Test connectivity
kubectl port-forward -n arcadia svc/tika-service 9998:9998
curl http://localhost:9998/version

# 5. Check logs
kubectl logs -n arcadia -l app=tika-server -f
```

## Environment-Specific Configurations

### Development Environment

```yaml
# config/development.yaml
tika:
  server_url: "http://localhost:9998"
  timeout: "30s"
  max_retries: 2
  max_file_size: 52428800  # 50MB
  max_connections: 5
  idle_conn_timeout: "60s"
  accept_all_formats: true
  office_confidence: 0.9
  fallback_confidence: 0.4
  circuit_breaker:
    failure_threshold: 3
    reset_timeout: "30s"
    half_open_max_requests: 2
```

### Staging Environment

```yaml
# config/staging.yaml
tika:
  server_url: "http://tika-staging.internal:9998"
  timeout: "45s"
  max_retries: 3
  max_file_size: 104857600  # 100MB
  max_connections: 15
  idle_conn_timeout: "90s"
  accept_all_formats: false
  office_confidence: 0.9
  fallback_confidence: 0.3
  circuit_breaker:
    failure_threshold: 5
    reset_timeout: "60s"
    half_open_max_requests: 3
```

### Production Environment

```yaml
# config/production.yaml
tika:
  server_url: "http://tika-production.internal:9998"
  timeout: "60s"
  max_retries: 5
  max_file_size: 209715200  # 200MB
  max_connections: 25
  idle_conn_timeout: "120s"
  accept_all_formats: false
  office_confidence: 0.95
  fallback_confidence: 0.3
  circuit_breaker:
    failure_threshold: 8
    reset_timeout: "120s"
    half_open_max_requests: 5
```

## Monitoring and Health Checks

### Prometheus Monitoring

```yaml
# monitoring/prometheus-config.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'tika-server'
    static_configs:
      - targets: ['tika-1:9998', 'tika-2:9998']
    metrics_path: /metrics
    scrape_interval: 30s
    scrape_timeout: 10s

  - job_name: 'arcadia-tika-processor'
    static_configs:
      - targets: ['arcadia-1:8080', 'arcadia-2:8080']
    metrics_path: /metrics
    scrape_interval: 15s

rule_files:
  - "tika-alerts.yml"

alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - alertmanager:9093
```

### Grafana Dashboard

```json
{
  "dashboard": {
    "title": "Apache Tika Monitoring",
    "panels": [
      {
        "title": "Tika Server Health",
        "type": "stat",
        "targets": [
          {
            "expr": "up{job=\"tika-server\"}",
            "legendFormat": "{{ instance }}"
          }
        ]
      },
      {
        "title": "Document Processing Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(tika_documents_processed_total[5m])",
            "legendFormat": "Documents/sec"
          }
        ]
      },
      {
        "title": "Processing Latency",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(tika_processing_duration_seconds_bucket[5m]))",
            "legendFormat": "95th percentile"
          }
        ]
      },
      {
        "title": "Circuit Breaker Status",
        "type": "stat",
        "targets": [
          {
            "expr": "tika_circuit_breaker_state",
            "legendFormat": "{{ instance }}"
          }
        ]
      }
    ]
  }
}
```

### Health Check Script

```bash
#!/bin/bash
# scripts/health-check.sh

TIKA_URL="${TIKA_URL:-http://localhost:9998}"
TIMEOUT="${TIMEOUT:-10}"
RETRIES="${RETRIES:-3}"

check_tika_health() {
    local url="$1"
    local attempt=1

    while [ $attempt -le $RETRIES ]; do
        echo "Health check attempt $attempt/$RETRIES for $url"

        if curl -f -s --max-time $TIMEOUT "$url/version" > /dev/null; then
            echo "✓ Tika server at $url is healthy"
            return 0
        fi

        echo "✗ Health check failed for $url"
        attempt=$((attempt + 1))
        sleep 2
    done

    return 1
}

# Check primary server
if ! check_tika_health "$TIKA_URL"; then
    echo "ERROR: Tika server health check failed"
    exit 1
fi

echo "All health checks passed"
exit 0
```

## Backup and Disaster Recovery

### Configuration Backup

```bash
#!/bin/bash
# scripts/backup-config.sh

BACKUP_DIR="/backup/tika-config/$(date +%Y%m%d-%H%M%S)"
CONFIG_DIR="/opt/tika-config"

mkdir -p "$BACKUP_DIR"

# Backup configuration files
cp -r "$CONFIG_DIR"/* "$BACKUP_DIR/"

# Backup Docker configurations
cp docker-compose.yml "$BACKUP_DIR/"
cp -r configs/ "$BACKUP_DIR/"

# Create backup manifest
cat > "$BACKUP_DIR/manifest.txt" << EOF
Backup created: $(date)
Tika version: $(curl -s http://localhost:9998/version)
Configuration files:
$(ls -la "$BACKUP_DIR/")
EOF

echo "Backup created at: $BACKUP_DIR"
```

### Disaster Recovery Procedure

```bash
#!/bin/bash
# scripts/disaster-recovery.sh

echo "Starting Tika disaster recovery..."

# 1. Stop current services
docker-compose down

# 2. Restore from backup
BACKUP_DIR="$1"
if [ -z "$BACKUP_DIR" ]; then
    echo "Usage: $0 <backup_directory>"
    exit 1
fi

# 3. Restore configuration
cp -r "$BACKUP_DIR"/configs/* ./configs/
cp "$BACKUP_DIR"/docker-compose.yml ./

# 4. Start services
docker-compose up -d

# 5. Verify recovery
sleep 30
if curl -f http://localhost:9998/version; then
    echo "✓ Disaster recovery successful"
else
    echo "✗ Disaster recovery failed"
    exit 1
fi
```

## Scaling Considerations

### Horizontal Scaling Strategy

1. **Load Balancing**: Use HAProxy or Nginx for distributing requests
2. **Auto Scaling**: Implement based on CPU/memory usage and queue depth
3. **Resource Allocation**: Size instances based on document processing load

### Scaling Guidelines

| Load Level | Instances | CPU/Instance | Memory/Instance |
|------------|-----------|--------------|------------------|
| Low | 1-2 | 1 core | 2GB |
| Medium | 3-5 | 2 cores | 4GB |
| High | 6-10 | 4 cores | 8GB |
| Very High | 10+ | 8 cores | 16GB |

### Auto-Scaling Configuration

```yaml
# autoscaling/docker-swarm.yml
version: '3.8'
services:
  tika:
    image: apache/tika:2.9.1
    deploy:
      replicas: 3
      update_config:
        parallelism: 1
        order: start-first
      restart_policy:
        condition: on-failure
      resources:
        limits:
          memory: 4G
          cpus: '2.0'
        reservations:
          memory: 2G
          cpus: '1.0'
```

## Security Best Practices

### Network Security

```yaml
# security/network-policy.yaml (Kubernetes)
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: tika-network-policy
  namespace: arcadia
spec:
  podSelector:
    matchLabels:
      app: tika-server
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: arcadia-app
    ports:
    - protocol: TCP
      port: 9998
  egress:
  - {}  # Allow all outbound traffic for Tika dependencies
```

### File Security

```xml
<!-- Security-hardened Tika configuration -->
<properties>
  <parsers>
    <parser class="org.apache.tika.parser.DefaultParser">
      <!-- Exclude dangerous parsers -->
      <parser-exclude class="org.apache.tika.parser.executable.ExecutableParser"/>
      <parser-exclude class="org.apache.tika.parser.pkg.PackageParser"/>
      <parser-exclude class="org.apache.tika.parser.crypto.Pkcs7Parser"/>
    </parser>
  </parsers>

  <server>
    <params>
      <!-- Security limits -->
      <param name="maxFileSize" type="long">104857600</param>
      <param name="maxEmbeddedResources" type="int">50</param>
      <param name="enableUnsecureFeatures" type="boolean">false</param>
      <param name="maxDocumentLength" type="int">100000000</param>
    </params>
  </server>
</properties>
```

### Container Security

```dockerfile
# Dockerfile.secure
FROM apache/tika:2.9.1

# Run as non-root user
USER 1001:1001

# Remove unnecessary packages
RUN apt-get remove -y wget curl && \
    apt-get autoremove -y && \
    apt-get clean

# Set secure file permissions
COPY --chown=1001:1001 tika-config.xml /opt/tika-config.xml
RUN chmod 644 /opt/tika-config.xml

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD java -cp /opt/tika-server.jar org.apache.tika.server.core.TikaServerCli --version || exit 1
```

## Maintenance and Updates

### Update Procedure

```bash
#!/bin/bash
# scripts/update-tika.sh

NEW_VERSION="$1"
if [ -z "$NEW_VERSION" ]; then
    echo "Usage: $0 <new_version>"
    exit 1
fi

echo "Updating Tika to version $NEW_VERSION"

# 1. Backup current configuration
./scripts/backup-config.sh

# 2. Update docker-compose.yml
sed -i "s/apache\/tika:.*/apache\/tika:$NEW_VERSION/" docker-compose.yml

# 3. Rolling update
docker-compose pull
docker-compose up -d --no-deps --scale tika=2 tika
sleep 60

# 4. Health check
if curl -f http://localhost:9998/version | grep -q "$NEW_VERSION"; then
    echo "✓ Update successful"
    docker-compose up -d --scale tika=1
else
    echo "✗ Update failed, rolling back"
    git checkout docker-compose.yml
    docker-compose up -d
    exit 1
fi
```

### Maintenance Tasks

```bash
#!/bin/bash
# scripts/maintenance.sh

echo "Starting Tika maintenance tasks..."

# 1. Log rotation
docker-compose exec tika logrotate /etc/logrotate.d/tika

# 2. Memory cleanup
docker-compose restart tika

# 3. Health verification
sleep 30
curl -f http://localhost:9998/version

# 4. Performance metrics collection
curl -s http://localhost:9998/metrics > /var/log/tika/metrics-$(date +%Y%m%d).json

echo "Maintenance completed"
```

## Troubleshooting

### Common Issues and Solutions

#### 1. Tika Server Won't Start

```bash
# Check logs
docker-compose logs tika

# Common fixes:
# - Increase memory allocation
# - Check port conflicts
# - Verify configuration syntax
# - Check file permissions
```

#### 2. High Memory Usage

```bash
# Monitor memory usage
docker stats tika

# Solutions:
# - Reduce max file size
# - Limit concurrent connections
# - Restart services periodically
# - Increase memory allocation
```

#### 3. Connection Timeouts

```bash
# Check network connectivity
telnet tika-server 9998

# Solutions:
# - Increase timeout values
# - Check network latency
# - Verify firewall rules
# - Scale Tika instances
```

#### 4. Processing Failures

```bash
# Enable debug logging
export TIKA_LOG_LEVEL=DEBUG
docker-compose restart tika

# Check processor logs
docker-compose exec tika cat /opt/logs/tika.log

# Common fixes:
# - Update Tika version
# - Adjust file size limits
# - Check file corruption
# - Review security settings
```

### Diagnostic Commands

```bash
# Health check
curl -f http://localhost:9998/version

# Server status
curl -s http://localhost:9998/status

# Memory usage
curl -s http://localhost:9998/metrics | grep memory

# Active connections
netstat -an | grep :9998

# Container resource usage
docker stats --no-stream

# Log analysis
docker-compose logs --tail=100 tika | grep ERROR
```

This comprehensive deployment guide covers all aspects of deploying Apache Tika with the Arcadia Documents Module across different environments and scales.