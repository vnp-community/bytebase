# T-001-05: Kubernetes Deployment Spec

| Field | Value |
|---|---|
| **Task ID** | T-001-05 |
| **Solution** | SOL-AVAIL-001 |
| **Depends On** | T-001-04, T-003-02 |
| **Target File** | `deploy/k8s/deployment.yaml` (NEW) |

---

## Objective

Tạo K8s Deployment YAML: 3 replicas, zero-downtime rolling update, liveness/readiness probes.

## Implementation

Xem SOL-AVAIL-001 §2.5. Key specs:
- `replicas: 3`
- `strategy.rollingUpdate.maxUnavailable: 0`, `maxSurge: 1`
- `terminationGracePeriodSeconds: 60` (> 30s graceful shutdown)
- `livenessProbe: /healthz` (period 10s, failure 3)
- `readinessProbe: /readyz` (period 5s, failure 3)
- `preStop: sleep 5` (allow endpoint removal)
- `REPLICA_ID` from `metadata.name`
- `BB_HA: "true"`

## Acceptance Criteria

- [ ] Valid K8s Deployment YAML
- [ ] Zero-downtime rolling update (maxUnavailable=0)
- [ ] Probes configured correctly
- [ ] Resource requests/limits set
