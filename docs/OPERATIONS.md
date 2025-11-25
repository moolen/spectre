# Operations Guide: Kubernetes Event Monitor

**Purpose**: Reference guide for running and maintaining the Kubernetes Event Monitoring System in production

---

## Table of Contents

1. [Deployment](#deployment)
2. [Monitoring](#monitoring)
3. [Troubleshooting](#troubleshooting)
4. [Storage Management](#storage-management)
5. [Performance Tuning](#performance-tuning)
6. [Backup & Recovery](#backup--recovery)

---

## Deployment

### Local Development

```bash
# Build and run locally
make build
make run

# Application starts on http://localhost:8080
# Data stored in ./data directory
```

### Docker Container

```bash
# Build image
make docker-build

# Run container
docker run -p 8080:8080 -v $(pwd)/data:/data k8s-event-monitor:latest

# With environment variables
docker run \
  -p 8080:8080 \
  -v $(pwd)/data:/data \
  -e LOG_LEVEL=debug \
  k8s-event-monitor:latest
```

### Kubernetes with Helm

```bash
# Install with defaults
helm install k8s-event-monitor ./chart \
  --namespace monitoring \
  --create-namespace

# Install with custom values
helm install k8s-event-monitor ./chart \
  --namespace monitoring \
  -f chart/examples/prod-values.yaml

# Verify deployment
kubectl get pods -n monitoring
kubectl get svc -n monitoring
kubectl get pvc -n monitoring
```

### Helm Upgrade

```bash
# Update with new values
helm upgrade k8s-event-monitor ./chart \
  --namespace monitoring \
  --values new-values.yaml

# Verify upgrade
kubectl rollout status deployment/k8s-event-monitor -n monitoring
```

### Helm Uninstall

```bash
# Remove deployment
helm uninstall k8s-event-monitor --namespace monitoring

# Optionally delete namespace
kubectl delete namespace monitoring
```

---

## Monitoring

### Pod Status

```bash
# Check if pod is running
kubectl get pods -n monitoring

# Expected output:
# NAME                                    READY   STATUS    RESTARTS
# k8s-event-monitor-5d4c6f7g8h-9i0j1k    1/1     Running   0

# Get detailed status
kubectl describe pod -n monitoring -l app.kubernetes.io/name=k8s-event-monitor
```

### Logs

```bash
# View recent logs
kubectl logs -n monitoring deployment/k8s-event-monitor

# Stream logs in real-time
kubectl logs -n monitoring deployment/k8s-event-monitor -f

# View specific number of lines
kubectl logs -n monitoring deployment/k8s-event-monitor --tail=100

# Logs from previous instance (if crashed)
kubectl logs -n monitoring deployment/k8s-event-monitor --previous
```

### Health Checks

```bash
# Liveness probe (is pod alive?)
kubectl get pod -n monitoring -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")]}'

# Readiness probe (is pod ready for traffic?)
kubectl get pod -n monitoring -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")]}'

# Manual health check
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- \
  curl localhost:8080/v1/search?start=1\&end=2
```

### Storage Usage

```bash
# Check PVC status
kubectl get pvc -n monitoring

# Check disk usage in pod
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- du -sh /data

# Check individual files
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- \
  du -sh /data/* | sort -h

# Check available space
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- df -h /data
```

### Resource Usage

```bash
# CPU and memory usage
kubectl top pod -n monitoring

# View requested vs actual
kubectl get pod -n monitoring -o jsonpath='{.items[0].spec.containers[0].resources}'
```

### API Health

```bash
# Port-forward to local machine
kubectl port-forward -n monitoring svc/k8s-event-monitor 8080:8080 &

# Test API
curl http://localhost:8080/v1/search?start=1\&end=2

# Check response time
time curl http://localhost:8080/v1/search?start=1\&end=2

# Check execution metrics
curl -s http://localhost:8080/v1/search?start=1\&end=2 | jq '{executionTimeMs, segmentsScanned, segmentsSkipped}'
```

---

## Troubleshooting

### Pod Won't Start

```bash
# Check pod status
kubectl describe pod -n monitoring -l app.kubernetes.io/name=k8s-event-monitor

# View logs
kubectl logs -n monitoring deployment/k8s-event-monitor

# Common issues:
# 1. ImagePullBackOff - Image not found
#    Solution: Build and push image, update values.yaml
#
# 2. CrashLoopBackOff - Application crashes
#    Solution: Check logs for error messages
#
# 3. Pending - Resource constraints
#    Solution: Check node resources, adjust pod requests
```

### RBAC Permission Errors

```bash
# Check if service account has permissions
kubectl auth can-i watch pods \
  --as=system:serviceaccount:monitoring:k8s-event-monitor

# Expected output: yes

# If "no", check ClusterRole
kubectl describe clusterrole k8s-event-monitor

# Check ClusterRoleBinding
kubectl describe clusterrolebinding k8s-event-monitor

# Common fix: Ensure namespace matches
kubectl describe clusterrolebinding k8s-event-monitor | grep -i "namespace"
```

### No Events Being Captured

```bash
# Check logs for watcher initialization
kubectl logs -n monitoring deployment/k8s-event-monitor | grep -i watcher

# Verify RBAC permissions
kubectl auth can-i watch pods --as=system:serviceaccount:monitoring:k8s-event-monitor

# Create a test resource
kubectl run test-pod --image=nginx

# Query for the test event
curl "http://localhost:8080/v1/search?start=$(date -d '5 minutes ago' +%s)&end=$(date +%s)&kind=Pod"

# If no events, check:
# 1. Application has been running (needs to be initialized when Pod was created)
# 2. RBAC permissions are correct
# 3. Data directory is writable
```

### Query Returns Empty Results

```bash
# Verify events exist at all
curl "http://localhost:8080/v1/search?start=0&end=9999999999"

# Check time range (common mistake)
NOW=$(date +%s)
YESTERDAY=$((NOW - 86400))
curl "http://localhost:8080/v1/search?start=$YESTERDAY&end=$NOW"

# Verify filter values are case-sensitive
# ❌ Wrong: kind=pod
# ✅ Correct: kind=Pod

# Check available storage files
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- ls -la /data/

# If no files, events haven't been captured yet
```

### High Memory Usage

```bash
# Check current usage
kubectl top pod -n monitoring

# Reduce EventBuffer size (in env vars)
# Or reduce max decompressed block size

# Check what's consuming memory
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- pmap -x <PID>

# If query is slow, could be large result set
# Try narrowing time window or adding filters
```

### Slow Query Performance

```bash
# Check execution time
curl -s "http://localhost:8080/v1/search?start=1700000000&end=1700086400" | jq .executionTimeMs

# Check segment skipping efficiency
curl -s "http://localhost:8080/v1/search?start=1700000000&end=1700086400" | \
  jq '{scanned: .segmentsScanned, skipped: .segmentsSkipped, ratio: (.segmentsSkipped / .segmentsScanned)}'

# If ratio < 0.5 (50%), add more filters
curl -s "http://localhost:8080/v1/search?start=1700000000&end=1700086400&kind=Pod&namespace=default" | \
  jq '.executionTimeMs'

# Check storage I/O
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- iostat -x 1 5
```

### Disk Full

```bash
# Check available space
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- df -h /data

# Check what's consuming space
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- du -sh /data/* | sort -h

# Identify oldest files
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- ls -lt /data/ | tail -5

# Temporary fix: Delete old files
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- rm /data/old-file.bin

# Permanent fix:
# 1. Increase PVC size
#    kubectl patch pvc k8s-event-monitor -n monitoring -p '{"spec":{"resources":{"requests":{"storage":"20Gi"}}}}'
# 2. Implement TTL-based cleanup
# 3. Archive data to external storage
```

---

## Storage Management

### File Organization

```bash
# List all event files
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- ls -la /data/

# Example:
# -rw-r--r--  1 1000 1000  1048576 Nov 25 00:00 2025-11-25T00.bin
# -rw-r--r--  1 1000 1000  1245632 Nov 25 01:01 2025-11-25T01.bin
# -rw-r--r--  1 1000 1000   923456 Nov 25 02:02 2025-11-25T02.bin

# Files are immutable after hour completion
```

### Disk Space Analysis

```bash
# Total storage used
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- \
  du -sh /data

# Storage per hour
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- \
  du -sh /data/* | sort -h

# Calculate growth rate
# Example: 1GB per 24 hours → 30GB per month

# Calculate cost
# Size = events_per_day * 30 days * avg_event_size * compression_ratio
# Example: 100K events/day * 30 days * 5KB * 0.08 = ~120MB
```

### Archive Old Files

```bash
# Compress old files
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- \
  gzip /data/2025-11-20*.bin

# Copy to external storage
kubectl cp monitoring/k8s-event-monitor:/data/2025-11-20T00.bin.gz \
  ./backups/2025-11-20T00.bin.gz

# Verify then delete
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- \
  rm /data/2025-11-20*.bin.gz
```

### Cleanup Policy

Implement one of these strategies:

**1. Manual Cleanup**
```bash
# Delete files older than N days
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- \
  find /data -name "*.bin" -mtime +30 -delete  # Keep 30 days
```

**2. File Rotation (future feature)**
```
- Automatically rotate files older than N days
- Archive to S3/GCS
- Maintain local cache of recent N days
```

**3. TTL with External Storage**
```
- Keep recent files locally (e.g., 7 days)
- Archive older files to cloud storage
- Query can transparently access archived data
```

---

## Performance Tuning

### Configure Block Size

Block size affects compression ratio vs. memory usage:

```yaml
# In values.yaml
config:
  blockSize: 262144  # 256KB (default)
  # Larger blocks: better compression, more memory
  # Smaller blocks: less memory, faster decompression
```

**Recommended**:
- Development: 32KB (low memory)
- Production: 256KB (optimal balance)
- High-volume: 512KB-1MB (better compression)

### Configure Event Buffer

Event buffer size affects throughput and memory:

```yaml
# In values.yaml
resources:
  requests:
    memory: "256Mi"
  limits:
    memory: "1Gi"
```

**Tuning**:
- Buffer size ≈ 10-20% of memory limit
- Larger buffer = better compression, more memory
- Smaller buffer = lower memory, faster flushing

### Configure Concurrency

```bash
# Number of parallel file readers (in code)
# Default: number of CPU cores

# Increase for I/O bound workloads
# Decrease for CPU bound workloads
```

### Monitor Query Performance

```bash
# Track metrics over time
while true; do
  curl -s "http://localhost:8080/v1/search?start=$(date -d '1 hour ago' +%s)&end=$(date +%s)" | \
    jq '{time: .executionTimeMs, scanned: .segmentsScanned, skipped: .segmentsSkipped}'
  sleep 60
done
```

---

## Backup & Recovery

### Regular Backups

```bash
# Backup storage to local disk
kubectl cp monitoring/k8s-event-monitor:/data ./k8s-event-monitor-backup

# Compress backup
tar -czf k8s-event-monitor-backup-$(date +%Y%m%d).tar.gz k8s-event-monitor-backup

# Upload to cloud storage
gsutil -m cp k8s-event-monitor-backup-*.tar.gz gs://my-backups/

# Or AWS S3
aws s3 sync k8s-event-monitor-backup s3://my-backups/
```

### Restore from Backup

```bash
# Download backup from cloud
gsutil cp gs://my-backups/k8s-event-monitor-backup-*.tar.gz .

# Extract backup
tar -xzf k8s-event-monitor-backup-*.tar.gz

# Copy to pod
kubectl cp k8s-event-monitor-backup monitoring/k8s-event-monitor:/data-restore

# Verify integrity
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- \
  find /data-restore -name "*.bin" -exec md5sum {} \; | head -5

# Swap directories
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- \
  mv /data /data-old && mv /data-restore /data

# Restart pod
kubectl rollout restart deployment/k8s-event-monitor -n monitoring
```

### Disaster Recovery Plan

1. **Regular Backups**: Every 24 hours
2. **Test Restores**: Monthly
3. **Off-site Storage**: Cloud provider
4. **Retention Policy**: Keep 90 days of backups
5. **RTO Target**: <1 hour
6. **RPO Target**: <24 hours

---

## Common Maintenance Tasks

### Update Container Image

```bash
# Build new image
make docker-build

# Update Helm values
helm upgrade k8s-event-monitor ./chart \
  --namespace monitoring \
  --set image.tag=<new-tag>

# Verify update
kubectl rollout status deployment/k8s-event-monitor -n monitoring
```

### Increase Storage Size

```bash
# For PVC (if PVC supports resize)
kubectl patch pvc k8s-event-monitor -n monitoring \
  -p '{"spec":{"resources":{"requests":{"storage":"50Gi"}}}}'

# Verify
kubectl get pvc -n monitoring

# If PVC doesn't support resize:
# 1. Create new larger PVC
# 2. Copy data to new PVC
# 3. Update deployment to use new PVC
```

### Change Log Level

```bash
# Update environment variable
kubectl set env deployment/k8s-event-monitor \
  -n monitoring \
  LOG_LEVEL=debug

# Verify
kubectl get deployment -n monitoring -o jsonpath='{.items[0].spec.template.spec.containers[0].env}'
```

### Scale Replicas (read-only)

```bash
# Note: Only query replicas can be scaled (read-only)
# Writing replicas must be single instance

# Scale query replicas
kubectl scale deployment/k8s-event-monitor-query \
  --replicas=3 \
  -n monitoring
```

---

## Support & Debugging

### Collect Debug Information

```bash
# Pod info
kubectl describe pod -n monitoring <pod-name>

# Recent events
kubectl get events -n monitoring --sort-by='.lastTimestamp'

# Full logs
kubectl logs -n monitoring deployment/k8s-event-monitor > debug.log

# Pod manifest
kubectl get pod -n monitoring <pod-name> -o yaml > pod-config.yaml

# PVC status
kubectl get pvc -n monitoring -o yaml > pvc-status.yaml

# Create debug bundle
kubectl debug -n monitoring <pod-name> --image=busybox
```

### Performance Profiling

```bash
# Check Go runtime stats
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- \
  curl localhost:8080/debug/pprof/

# CPU profile
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- \
  curl localhost:8080/debug/pprof/profile > cpu.prof

# Memory profile
kubectl exec -n monitoring -it deployment/k8s-event-monitor -- \
  curl localhost:8080/debug/pprof/heap > mem.prof
```

---

## References

- [Quickstart Guide](../specs/001-k8s-event-monitor/quickstart.md)
- [API Documentation](./API.md)
- [Architecture Overview](./ARCHITECTURE.md)
- [Helm Chart README](../chart/README.md)
