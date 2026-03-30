# Local vs. AKS Security Checklist

**Quick reference: What's safe locally vs. what breaks on AKS**

---

## ✅ Safe for Local Docker Compose Testing

- [x] **Hardcoded default credentials** (e.g., `minioadmin123`)
  - Why: Docker daemon is isolated to your machine
  - Git safety: `.env` is .gitignore'd, docker-compose.yml has no real credentials
  
- [x] **Ports exposed on localhost** (e.g., `:5432:5432`)
  - Why: Only accessible from your machine
  - Internet: Not exposed without explicit port forwarding
  
- [x] **No RBAC or NetworkPolicy**
  - Why: Docker Compose has its own networking model
  - Isolation: Services isolated within compose network
  
- [x] **Unvalidated input in orchestrator**
  - Why: Localhost only, can't reach external systems
  - Risk: Low since attacker would need local shell access anyway
  
- [x] **LoadBalancer service type in K8s YAML**
  - Why: Not being deployed to K8s yet
  - Safety: YAML is just reference documentation for now

---

## 🚨 BREAKS on AKS - Must Fix Before Deployment

### 1. Hardcoded Credentials in docker-compose.yml

```yaml
# ❌ BREAKS on AKS
environment:
  MINIO_ROOT_PASSWORD: minioadmin123

# ✅ FIXED for AKS
environment:
  MINIO_ROOT_PASSWORD: ${MINIO_ROOT_PASSWORD}  # From .env
```

**Action:** Update docker-compose.yml to use `${VAR}` substitution  
**Time:** 15 min  
**Impact:** Prevents credential theft via pod inspection

---

### 2. Virtual API Keys in K8s Manifests

```yaml
# ❌ BREAKS on AKS
apiVersion: v1
kind: Secret
stringData:
  key: sk-researcher-virtual  # Hardcoded!

# ✅ FIXED for AKS
# Use Azure Key Vault with workload identity
apiVersion: secrets-store.csi.x-k8s.io/v1
kind: SecretProviderClass
provider: azure
```

**Action:** Implement Azure Key Vault integration  
**Time:** 30 min (Phase 2)  
**Impact:** Prevents secret exposure in git, etcd, backups

---

### 3. LoadBalancer Service Exposes to Internet

```yaml
# ❌ BREAKS on AKS
apiVersion: v1
kind: Service
spec:
  type: LoadBalancer  # Gets public IP!

# ✅ FIXED for AKS
spec:
  type: ClusterIP  # Internal only
```

**Action:** Change to ClusterIP, add Ingress for controlled access  
**Time:** 5 min (Phase 1)  
**Impact:** Prevents DDoS/exploit bots from scanning service

---

### 4. No Input Validation in Orchestrator

```python
# ❌ BREAKS on AKS
class OrchestrationRequest(BaseModel):
    topic: str  # No validation!

# ✅ FIXED for AKS
class OrchestrationRequest(BaseModel):
    topic: str = Field(min_length=5, max_length=500)
    
    @field_validator('topic')
    def validate_injection(cls, v):
        if 'ignore' in v.lower():
            raise ValueError('Suspicious keywords detected')
```

**Action:** Add Pydantic field validators  
**Time:** 20 min (Phase 1)  
**Impact:** Prevents prompt injection, request smuggling, DoS

---

### 5. No RBAC Defined

```yaml
# ❌ BREAKS on AKS
spec:
  serviceAccountName: researcher  # Default permissions!

# ✅ FIXED for AKS
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: researcher-rolebinding
roleRef:
  kind: Role
  name: researcher-role
subjects:
- kind: ServiceAccount
  name: researcher
```

**Action:** Create ServiceAccounts, Roles, RoleBindings  
**Time:** 15 min (Phase 2)  
**Impact:** Prevents privilege escalation in cluster

---

### 6. No NetworkPolicy Defined

```yaml
# ❌ BREAKS on AKS
# All pods can reach all pods, metadata service, internet

# ✅ FIXED for AKS
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
spec:
  podSelector:
    matchLabels:
      agent: "true"
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: litellm-proxy  # ONLY allow LiteLLM
    ports:
    - protocol: TCP
      port: 8000
```

**Action:** Create NetworkPolicy to restrict pod egress  
**Time:** 20 min (Phase 2)  
**Impact:** Prevents SSRF attacks, metadata service exploitation, lateral movement

---

### 7. No Pod Security Standards

```yaml
# ❌ BREAKS on AKS
spec:
  containers:
  - name: app
    # Can run as root, privileged mode, etc.

# ✅ FIXED for AKS
spec:
  securityContext:
    runAsNonRoot: true
    fsReadOnlyRootFilesystem: true
  containers:
  - name: app
    securityContext:
      allowPrivilegeEscalation: false
      capabilities:
        drop:
        - ALL
```

**Action:** Add security context to deployments  
**Time:** 15 min (Phase 2)  
**Impact:** Prevents container escape, privilege escalation

---

## 📋 Deployment Workflow

### Stage 1: Local Development ✅
```bash
cd examples/research-swarm
echo "OPENAI_API_KEY=sk-proj-your-key" > .env.local
docker compose --env-file .env.local up -d
curl http://localhost:9000/orchestrate -d '{"topic": "AI"}'
```
**Current state:** Safe, uses hardcoded defaults (acceptable for localhost)

### Stage 2: Before Staging AKS ⚠️ 
```bash
# Must complete Phase 1 in REMEDIATION_GUIDE.md
# 1. Update docker-compose.yml to use ${VAR}
# 2. Add input validation to orchestrator.py
# 3. Change LoadBalancer to ClusterIP
# Test locally with strong passwords
MINIO_PASSWORD=$(openssl rand -base64 32) docker compose up -d
```

### Stage 3: Before Production AKS 🔴
```bash
# Must complete Phases 1-2 in REMEDIATION_GUIDE.md
# 1. Implement Azure Key Vault
# 2. Add RBAC, NetworkPolicy, PSP
# 3. Enable audit logging
# 4. Run security penetration test
az aks update --enable-managed-identity ...
kubectl apply -f k8s/*.yaml
```

---

## 🔍 Safety Verification Commands

### Check Local Safety
```bash
# Should show no real credentials
grep -r "sk-proj-\|minioadmin123\|spans_dev" examples/research-swarm --exclude-dir=.git

# Should show only .env in gitignore
git check-ignore -v examples/research-swarm/.env

# Verify credentials are environment-based
grep "environment:" docker-compose.yml | grep -v '\${' | head -5
```

### Check Kubernetes Safety (Before Deployment)
```bash
# Should show ONLY demo-only label and base64 (not plaintext)
kubectl get secret litellm-keys -o yaml 2>/dev/null | grep -A3 stringData

# Should show only ClusterIP, not LoadBalancer
kubectl get svc orchestrator -o wide 2>/dev/null | grep -v EXTERNAL

# Should show all 4 service accounts
kubectl get sa -n agentic-demo 2>/dev/null | wc -l

# Should show multiple RBAC bindings
kubectl get rolebindings -n agentic-demo 2>/dev/null | wc -l

# Should show network policies blocking egress
kubectl get networkpolicies -n agentic-demo 2>/dev/null | grep -i deny
```

---

## 🆘 Common Issues & Fixes

### "I can see passwords in `docker ps`"
**Cause:** Environment variables visible in container inspect  
**Status:** ✅ EXPECTED for local - OK because daemon is on your machine  
**For AKS:** Use mounted secrets via volume mounts, not env vars

### "K8s manifests have plaintext secrets"
**Cause:** Using helm/kubectl to deploy YAML with actual credentials  
**Status:** 🚨 DANGEROUS - do not deploy  
**Fix:** Use Azure Key Vault integration + External Secrets Operator

### "Pods can reach 169.254.169.254"
**Cause:** No NetworkPolicy blocking metadata service  
**Status:** ✅ LOCAL - OK, metadata service not reachable from docker  
**For AKS:** Add explicit egress NetworkPolicy denying metadata

### "I need to deploy this to AKS now"
**Action:** Read REMEDIATION_GUIDE.md and implement Phase 1-2  
**Time:** 3-5 hours  
**Risk if skipped:** Cluster compromise possible

---

## 📖 Reference Files

| File | Purpose | Audience |
|------|---------|----------|
| `SECURITY_AUDIT.md` | Detailed findings for each issue | Security auditors, architects |
| `REMEDIATION_GUIDE.md` | Step-by-step fixes with code | DevSecOps, backend engineers |
| `QUICKSTART.md` | Fast setup for local testing | Developers, product managers |
| `README.md` | Overview of demo pipeline | Everyone |
| `.env.example` | Environment variable template | Operators |

---

## 🎯 Next Steps

1. **For local testing:** You're good! Current setup is safe.
2. **For staging/production:** Read `REMEDIATION_GUIDE.md` Phases 1-2
3. **For security compliance:** Complete Phase 3 (audit logging, monitoring)
4. **Questions?** See `SECURITY_AUDIT.md` for detailed explanations

---

**Last Updated:** March 31, 2026  
**Status:** ✅ Local testing approved | 🚨 AKS deployment blocked until Phase 1-2 complete
