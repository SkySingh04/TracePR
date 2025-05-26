# TracePR Deployment Guide

This guide explains how to deploy the TracePR application using Docker, Kubernetes, and Jenkins CI/CD pipelines.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Environment Configuration](#environment-configuration)
- [Docker Deployment](#docker-deployment)
- [Kubernetes Deployment](#kubernetes-deployment)
- [Jenkins CI/CD Setup](#jenkins-cicd-setup)
- [Monitoring and Observability](#monitoring-and-observability)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### Required Tools

- **Docker** (v20.10+)
- **Kubernetes** (v1.24+)
- **kubectl** (compatible with your cluster version)
- **Kustomize** (v4.0+)
- **Jenkins** (v2.400+)
- **Go** (v1.24+)
- **Python** (v3.11+)

### Required Accounts/Services

- GitHub account with repository access
- Claude AI API key
- Docker registry access
- Kubernetes cluster access
- Grafana instance (optional)
- Amplitude account (optional)
- Datadog account (optional)

## Environment Configuration

### 1. Copy Environment Template

```bash
cp env.template .env
```

### 2. Configure Environment Variables

Edit the `.env` file with your actual values:

```bash
# GitHub Configuration
GITHUB_TOKEN=ghp_your_actual_github_token
REPO_OWNER=your_github_username
REPO_NAME=your_repository_name

# Claude AI Configuration
CLAUDE_API_KEY=sk-ant-your_actual_claude_api_key

# Grafana Configuration (if using)
GRAFANA_SERVICE_ACCOUNT_TOKEN=your_grafana_token
GRAFANA_URL=https://your-grafana-instance.com

# Other services...
```

### 3. Encode Secrets for Kubernetes

For Kubernetes deployment, encode your secrets:

```bash
echo -n "your_actual_secret" | base64
```

Update the `k8s/secret.yaml` file with the base64-encoded values.

## Docker Deployment

### 1. Build the Application

```bash
# Build the Go application
go build -o tracepr .

# Build Docker image
docker build -t tracepr:latest .
```

### 2. Run with Docker Compose (for testing)

```bash
# Start test environment
docker-compose -f docker-compose.test.yml up -d

# Check logs
docker-compose -f docker-compose.test.yml logs -f tracepr

# Stop environment
docker-compose -f docker-compose.test.yml down
```

### 3. Run Single Container

```bash
# Run the application
docker run -d \
  --name tracepr \
  --env-file .env \
  -p 8080:8080 \
  tracepr:latest

# Check logs
docker logs -f tracepr

# Stop container
docker stop tracepr && docker rm tracepr
```

## Kubernetes Deployment

### 1. Prepare Kubernetes Cluster

Ensure you have a running Kubernetes cluster and `kubectl` is configured:

```bash
kubectl cluster-info
kubectl get nodes
```

### 2. Update Configuration

Edit the Kubernetes manifests as needed:

- Update `k8s/secret.yaml` with your base64-encoded secrets
- Update `k8s/ingress.yaml` with your domain name
- Update `k8s/configmap.yaml` with your configuration

### 3. Deploy Using Kustomize

```bash
# Deploy to Kubernetes
kubectl apply -k k8s/

# Check deployment status
kubectl get pods -n tracepr
kubectl get services -n tracepr
kubectl get ingress -n tracepr

# Check logs
kubectl logs -f deployment/tracepr -n tracepr
```

### 4. Alternative: Deploy Individual Manifests

```bash
# Create namespace
kubectl apply -f k8s/namespace.yaml

# Create service account and RBAC
kubectl apply -f k8s/serviceaccount.yaml

# Create configuration
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secret.yaml

# Deploy application
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml

# Optional: Create ingress, HPA, and network policy
kubectl apply -f k8s/ingress.yaml
kubectl apply -f k8s/hpa.yaml
kubectl apply -f k8s/networkpolicy.yaml
```

### 5. Verify Deployment

```bash
# Check all resources
kubectl get all -n tracepr

# Check pod status
kubectl describe pod -l app=tracepr -n tracepr

# Test service
kubectl port-forward service/tracepr-service 8080:80 -n tracepr
curl http://localhost:8080/health
```

## Jenkins CI/CD Setup

### 1. Jenkins Prerequisites

Install required Jenkins plugins:

- Pipeline
- Docker Pipeline
- Kubernetes
- GitHub Integration
- Slack Notification (optional)

### 2. Configure Jenkins Credentials

Add the following credentials in Jenkins:

- `docker-registry-creds`: Docker registry username/password
- `kubeconfig`: Kubernetes configuration file
- `github-token`: GitHub personal access token
- `teams-webhook`: Microsoft Teams webhook URL (optional)

### 3. Create Jenkins Jobs

#### Main Pipeline Job

1. Create a new Pipeline job
2. Configure GitHub webhook trigger
3. Use the `Jenkinsfile` for the pipeline script
4. Configure branch sources (main, develop)

#### Pull Request Pipeline Job

1. Create a new Multibranch Pipeline job
2. Configure GitHub source with PR discovery
3. Use the `jenkins/Jenkinsfile.pr` for PR builds

### 4. Configure Webhooks

In your GitHub repository settings:

1. Go to Settings → Webhooks
2. Add webhook URL: `https://your-jenkins-url/github-webhook/`
3. Select events: Push, Pull requests
4. Ensure webhook is active

### 5. Using Shared Library (Optional)

If using the Jenkins shared library:

1. Configure the shared library in Jenkins Global Configuration
2. Use in your Jenkinsfile:

```groovy
@Library('your-shared-library') _

buildTracePR([
    dockerRegistry: 'your-registry.com',
    kubeNamespace: 'tracepr',
    notifications: [
        slack: [
            channel: '#deployments'
        ]
    ]
])
```

## Monitoring and Observability

### 1. Application Metrics

The application exposes metrics at `/metrics` endpoint for Prometheus scraping.

### 2. Health Checks

- Liveness probe: `/health`
- Readiness probe: `/ready`

### 3. Logging

Logs are structured JSON format and can be collected by:

- Kubernetes: `kubectl logs`
- Fluentd/Fluent Bit
- ELK Stack
- Grafana Loki

### 4. Grafana Dashboards

If using Grafana integration, the application can create dashboards automatically based on PR analysis.

## Troubleshooting

### Common Issues

#### 1. Pod CrashLoopBackOff

```bash
# Check pod logs
kubectl logs -f deployment/tracepr -n tracepr

# Check pod events
kubectl describe pod -l app=tracepr -n tracepr

# Common causes:
# - Missing environment variables
# - Invalid API keys
# - Resource limits too low
```

#### 2. ImagePullBackOff

```bash
# Check image name and tag
kubectl describe pod -l app=tracepr -n tracepr

# Verify image exists in registry
docker pull your-registry.com/tracepr:latest

# Check image pull secrets
kubectl get secrets -n tracepr
```

#### 3. Service Not Accessible

```bash
# Check service endpoints
kubectl get endpoints -n tracepr

# Check service configuration
kubectl describe service tracepr-service -n tracepr

# Test internal connectivity
kubectl run test-pod --image=busybox --rm -it -- wget -qO- http://tracepr-service.tracepr.svc.cluster.local
```

#### 4. Jenkins Build Failures

```bash
# Check Jenkins logs
# Common issues:
# - Missing credentials
# - Docker daemon not accessible
# - Kubernetes cluster not reachable
# - Go/Python dependencies not available
```

### Debug Commands

```bash
# Get all resources in namespace
kubectl get all -n tracepr

# Check resource usage
kubectl top pods -n tracepr

# Check events
kubectl get events -n tracepr --sort-by='.lastTimestamp'

# Execute into pod
kubectl exec -it deployment/tracepr -n tracepr -- /bin/sh

# Check configuration
kubectl get configmap tracepr-config -n tracepr -o yaml
kubectl get secret tracepr-secrets -n tracepr -o yaml
```

### Performance Tuning

#### 1. Resource Limits

Adjust in `k8s/deployment.yaml`:

```yaml
resources:
  requests:
    memory: "512Mi"  # Increase if needed
    cpu: "200m"      # Increase if needed
  limits:
    memory: "1Gi"    # Increase if needed
    cpu: "1000m"     # Increase if needed
```

#### 2. Horizontal Pod Autoscaler

Adjust in `k8s/hpa.yaml`:

```yaml
spec:
  minReplicas: 3     # Increase for higher availability
  maxReplicas: 20    # Increase for higher scale
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 50  # Lower for more aggressive scaling
```

#### 3. Database Connections

If using database, tune connection pool settings in environment variables.

## Security Considerations

### 1. Secrets Management

- Never commit secrets to version control
- Use Kubernetes secrets or external secret management
- Rotate secrets regularly
- Use least privilege access

### 2. Network Security

- Enable network policies
- Use TLS for all external communication
- Restrict ingress to necessary ports only

### 3. Container Security

- Run as non-root user (already configured)
- Use minimal base images
- Scan images for vulnerabilities
- Keep dependencies updated

### 4. RBAC

- Use minimal required permissions
- Regular audit of service account permissions
- Separate service accounts for different environments

## Backup and Recovery

### 1. Configuration Backup

```bash
# Backup Kubernetes manifests
kubectl get all -n tracepr -o yaml > tracepr-backup.yaml

# Backup secrets (be careful with this)
kubectl get secrets -n tracepr -o yaml > tracepr-secrets-backup.yaml
```

### 2. Application Data

If the application stores data:

- Regular database backups
- Persistent volume snapshots
- Configuration file backups

### 3. Disaster Recovery

- Document recovery procedures
- Test recovery processes regularly
- Maintain infrastructure as code
- Use multiple availability zones

## Support

For issues and questions:

1. Check the application logs
2. Review this deployment guide
3. Check the main README.md
4. Create an issue in the GitHub repository

## Contributing

To contribute to the deployment configuration:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

---

**Note**: This deployment guide assumes a basic understanding of Docker, Kubernetes, and Jenkins. For production deployments, additional considerations for security, monitoring, and high availability should be implemented. 