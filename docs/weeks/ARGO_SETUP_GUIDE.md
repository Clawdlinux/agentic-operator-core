# Argo Workflows Setup & Testing Guide

## Overview

This guide walks through setting up Argo Workflows with the agentic-k8s-operator for local development and testing.

**Time estimate:** 15-20 minutes (depending on network speed and PVC provisioning)

---

## Prerequisites

### Required Tools

```bash
# Check installed versions
kind version        # v0.21.0+
kubectl version --client
docker --version    # Docker 20.10+
```

**Installation links:**
- [kind](https://kind.sigs.k8s.io/docs/user/quick-start/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [docker](https://docs.docker.com/install/)
- [helm](https://helm.sh/docs/intro/install/) (optional, for Argo)

### System Requirements

- **Disk space:** ~30 GB (for PVCs: PostgreSQL 10GB, MinIO 50GB, etc.)
- **Memory:** ~8 GB (kind cluster: 4-6 GB, services: 2-3 GB)
- **CPU:** 4+ cores recommended

---

## Quick Start

### Step 1: Create kind Cluster with Argo

```bash
# Make script executable
chmod +x scripts/setup-kind.sh

# Run setup
./scripts/setup-kind.sh agentic-k8s-dev
```

This creates:
- 2-node kind cluster (control plane + worker)
- Argo Workflows v4.0.1
- Shared services (PostgreSQL, MinIO, Browserless, LiteLLM)
- WorkflowTemplate

**Output:**
```
[1/5] Creating kind cluster...
✓ kind cluster created: agentic-k8s-dev
[2/5] Installing Argo Workflows...
✓ Argo Workflows installed
[3/5] Deploying shared services...
✓ Shared services deployed
[4/5] Deploying WorkflowTemplate...
✓ WorkflowTemplate deployed
[5/5] Setup complete!
```

### Step 2: Verify Installation

```bash
# Check cluster nodes
kubectl get nodes
# Expected: 2 nodes (control plane, worker)

# Check Argo controller
kubectl -n argo-workflows get deployment
# Expected: argo-workflows-workflow-controller READY

# Check shared services
kubectl -n shared-services get deployment
# Expected: postgres, minio, browserless, litellm

# Check WorkflowTemplate
kubectl -n argo-workflows get workflowtemplate
# Expected: visual-analysis-template
```

### Step 3: Access Argo UI (Optional)

```bash
# Port-forward to Argo Server
kubectl port-forward -n argo-workflows svc/argo-workflows-server 2746:2746

# Open browser
open http://localhost:2746
```

---

## Manual Installation (Alternative)

If you prefer step-by-step installation:

### Install kind Cluster

```bash
kind create cluster --name agentic-k8s-dev --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: worker
EOF
```

### Install Argo Workflows (via Helm)

```bash
# Add Helm repository
helm repo add argo https://argoproj.github.io/argo-helm
helm repo update

# Create namespace
kubectl create namespace argo-workflows

# Install Argo
helm install argo-workflows \
  argo/argo-workflows \
  --namespace argo-workflows \
  --version 0.41.0 \
  --set serviceAccount.create=true \
  --set server.authMode=server
```

### Deploy Shared Services

```bash
# Create namespace
kubectl create namespace shared-services

# Apply manifests
kubectl apply -f config/shared-services/
```

### Deploy WorkflowTemplate

```bash
# Create namespace
kubectl create namespace argo-workflows

# Apply template
kubectl apply -f config/argo/workflowtemplate.yaml
```

---

## Testing Workflow

### Test 1: Verify WorkflowTemplate Syntax

```bash
# Check YAML syntax (kubectl dry-run)
kubectl apply -f config/argo/workflowtemplate.yaml --dry-run=client

# Expected: No errors
```

### Test 2: Manual Workflow Submission

```bash
# Submit a test workflow (without agent image)
# This verifies Argo can execute the template

argo submit \
  -n argo-workflows \
  --from workflowtemplate/visual-analysis-template \
  --parameter job_id=test-001 \
  --parameter target_urls='["https://example.com"]' \
  --parameter minio_bucket=artifacts
```

**Note:** Workflow will fail at Step 1 (scraper) because agent image doesn't exist.  
This is expected and OK for basic Argo testing.

### Test 3: Check MinIO Connectivity

```bash
# Port-forward MinIO
kubectl port-forward -n shared-services svc/minio 9000:9000

# In another terminal, test S3 connectivity
# Install mc (MinIO client)
curl -O https://dl.min.io/client/mc/release/linux-amd64/mc
chmod +x mc

# Configure MinIO
./mc alias set minio http://localhost:9000 minioadmin minioadmin

# List buckets
./mc ls minio
# Expected: artifacts bucket
```

### Test 4: Check PostgreSQL Connectivity

```bash
# Port-forward PostgreSQL
kubectl port-forward -n shared-services svc/postgres 5432:5432

# In another terminal, connect
psql postgresql://langgraph:langgraph@localhost:5432/langgraph

# List checkpoint table
langgraph=# \dt langgraph.*;
# Expected: checkpoint table schema

langgraph=# \q
```

### Test 5: Check Browserless Connectivity

```bash
# Port-forward Browserless
kubectl port-forward -n shared-services svc/browserless 3000:3000

# In another terminal, test WebSocket
curl http://localhost:3000/health
# Expected: {"success": true}
```

### Test 6: Check LiteLLM Connectivity

```bash
# Port-forward LiteLLM
kubectl port-forward -n shared-services svc/litellm 8000:8000

# In another terminal, test health
curl http://localhost:8000/health
# Expected: {"status": "ok"}
```

---

## Troubleshooting

### Issue: Pods stuck in `Pending`

**Cause:** PVC not bound (no StorageClass or not enough disk space)

**Solution:**
```bash
# Check PVC status
kubectl get pvc -n shared-services

# Check events
kubectl describe pvc postgres-data -n shared-services

# Create a default StorageClass (for kind)
kubectl apply -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: standard
provisioner: rancher.io/local-path
volumeBindingMode: WaitForFirstConsumer
EOF
```

### Issue: Argo controller not starting

**Cause:** Insufficient CPU or memory for controller pod

**Solution:**
```bash
# Check controller logs
kubectl logs -n argo-workflows deployment/argo-workflows-workflow-controller

# Check node resources
kubectl describe nodes

# Increase kind memory allocation:
# Edit your Docker Desktop/Colima resources allocation
```

### Issue: `No route to host` when accessing services

**Cause:** Services not deployed or pods not running

**Solution:**
```bash
# Check service status
kubectl get svc -n shared-services

# Check pod status
kubectl get pods -n shared-services

# Check pod logs
kubectl logs -n shared-services deployment/minio
```

---

## Cleanup

### Delete entire cluster

```bash
kind delete cluster --name agentic-k8s-dev
```

### Delete specific namespace

```bash
# Delete shared services (keeps cluster running)
kubectl delete namespace shared-services

# Delete Argo workflows (keeps cluster running)
kubectl delete namespace argo-workflows
```

---

## Next Steps

Once setup is complete:

1. **Deploy the operator:** See `docs/OPERATOR_DEPLOYMENT.md`
2. **Create a test AgentWorkload CR:** See `docs/TESTING_AGENTWORKLOAD.md`
3. **Monitor workflow execution:** Use Argo UI or `argo get <workflow-name>`
4. **Inspect artifacts:** Use MinIO console or `mc` client

---

## Environment Variables

If you need to set environment variables for manual testing:

```bash
# Argo server (skip auth for testing)
export ARGO_SERVER=localhost:2746
export ARGO_NAMESPACE=argo-workflows

# MinIO credentials
export MINIO_ACCESS_KEY=minioadmin
export MINIO_SECRET_KEY=minioadmin
export MINIO_URL=http://minio.shared-services:9000

# PostgreSQL connection
export POSTGRES_DSN="postgresql://langgraph:langgraph@postgres.shared-services:5432/langgraph"

# Browserless WebSocket
export BROWSERLESS_URL="ws://browserless.shared-services:3000"

# LiteLLM HTTP API
export LITELLM_URL="http://litellm.shared-services:8000"
```

---

## Advanced: Custom Configuration

### Modify resource limits

Edit `config/shared-services/*.yaml` before applying:

```yaml
resources:
  requests:
    cpu: "250m"
    memory: "512Mi"
  limits:
    cpu: "1000m"
    memory: "1Gi"
```

### Modify Argo retention policy

Edit `config/argo/workflowtemplate.yaml`:

```yaml
spec:
  ttlSecondsAfterFinished: 900  # Delete after 15 minutes
  activeDeadlineSeconds: 1800   # Fail after 30 minutes
```

---

## References

- [Argo Workflows Documentation](https://argoproj.github.io/argo-workflows/)
- [kind Documentation](https://kind.sigs.k8s.io/)
- [MinIO Documentation](https://docs.min.io/)
- [Browserless Documentation](https://www.browserless.io/)
- [LiteLLM Documentation](https://docs.litellm.ai/)

---

**Last updated:** 2026-02-24  
**Maintainer:** agentic-k8s-operator team
