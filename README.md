# Pod Recycle Controller

A minimal Kubernetes controller that watches for pods in CrashLoopBackOff state and force deletes them.

## Build

```bash
# Download dependencies
go mod download

# Build locally
go build -o controller main.go

# Build Docker image
docker build -t pod-recycle-controller:latest .
```

## Deploy to EKS

```bash
# Apply the deployment
kubectl apply -f deployment.yaml

# Check controller logs
kubectl logs -n kube-system -l app=pod-recycle-controller -f
```

## How it works

1. Watches all pods across all namespaces
2. Detects pods with CrashLoopBackOff status
3. Force deletes the pod (grace period = 0)
4. Automatically reconnects if watch connection drops

## RBAC Permissions

The controller requires:
- `get`, `list`, `watch` on pods (to monitor status)
- `delete` on pods (to force delete)
