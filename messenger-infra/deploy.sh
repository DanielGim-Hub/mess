#!/bin/bash
set -euo pipefail

# ═══════════════════════════════════════════════════════════════════════════════
# Messenger Infrastructure Deployment Script
# ═══════════════════════════════════════════════════════════════════════════════

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
K8S_DIR="$SCRIPT_DIR/k8s"
OBS_DIR="$SCRIPT_DIR/observability"

usage() {
    echo "Usage: $0 <command>"
    echo ""
    echo "Commands:"
    echo "  infra        Deploy base infrastructure (namespaces, SA, secrets)"
    echo "  terraform    Apply Terraform configuration"
    echo "  argocd       Deploy ArgoCD and App of Apps"
    echo "  kafka        Deploy Kafka via Helm (Bitnami)"
    echo "  redis        Deploy Valkey via Helm"
    echo "  mesh         Deploy Istio manifests (mTLS, CB, retries)"
    echo "  rate-limit   Deploy Rate Limiting service + Valkey"
    echo "  ingress      Deploy HAProxy + Keepalived"
    echo "  services     Deploy chat, message, realtime-gateway via Helm"
    echo "  observability Deploy VictoriaMetrics, VictoriaLogs, Jaeger, Grafana"
    echo "  all          Deploy everything in order"
    echo "  test         Run Locust load tests"
    echo ""
}

deploy_infra() {
    echo "[INFRA] Deploying namespaces and RBAC..."
    kubectl apply -f "$K8S_DIR/namespaces/"
}

deploy_terraform() {
    echo "[TERRAFORM] Applying Terraform..."
    cd "$SCRIPT_DIR/terraform"
    terraform init
    terraform apply -auto-approve
}

deploy_argocd() {
    echo "[ARGOCD] Installing ArgoCD..."
    kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f -
    kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
    echo "[ARGOCD] Waiting for ArgoCD server..."
    kubectl wait --for=condition=available deployment/argocd-server -n argocd --timeout=300s
    echo "[ARGOCD] Deploying App of Apps..."
    kubectl apply -f "$K8S_DIR/argo-apps/root-application.yaml"
}

deploy_kafka() {
    echo "[KAFKA] Deploying Kafka via Helm..."
    helm repo add bitnami https://charts.bitnami.com/bitnami
    helm repo update
    helm upgrade --install kafka "$K8S_DIR/helm-charts/kafka" \
        --namespace kafka --create-namespace
}

deploy_redis() {
    echo "[REDIS] Deploying Valkey via Helm..."
    helm repo add bitnami https://charts.bitnami.com/bitnami
    helm repo update
    helm upgrade --install redis "$K8S_DIR/helm-charts/redis" \
        --namespace redis --create-namespace
}

deploy_mesh() {
    echo "[ISTIO] Deploying Istio manifests..."
    kubectl apply -f "$K8S_DIR/istio/"
}

deploy_rate_limit() {
    echo "[RATE-LIMIT] Deploying Envoy Rate Limit + Valkey..."
    kubectl apply -f "$K8S_DIR/valkey/"
    kubectl apply -f "$K8S_DIR/istio/rate-limit-service.yaml"
    kubectl apply -f "$K8S_DIR/istio/envoyfilter-ratelimit.yaml"
}

deploy_ingress() {
    echo "[INGRESS] Deploying HAProxy + Keepalived..."
    kubectl apply -f "$K8S_DIR/ingress/"
}

deploy_services() {
    echo "[SERVICES] Deploying microservices via Helm..."
    helm upgrade --install chat-service "$K8S_DIR/helm-charts/chat-service" \
        --namespace chat-service --create-namespace
    helm upgrade --install message-service "$K8S_DIR/helm-charts/message-service" \
        --namespace message-service --create-namespace
    helm upgrade --install realtime-gateway "$K8S_DIR/helm-charts/realtime-gateway" \
        --namespace realtime-gateway --create-namespace
}

deploy_observability() {
    echo "[OBSERVABILITY] Deploying observability stack..."
    kubectl apply -f "$OBS_DIR/victoria-metrics/"
    kubectl apply -f "$OBS_DIR/victoria-logs/"
    kubectl apply -f "$OBS_DIR/jaeger/"
    kubectl apply -f "$OBS_DIR/otel-collector/"
    kubectl apply -f "$OBS_DIR/grafana/"
}

run_tests() {
    echo "[TEST] Running Locust load tests..."
    cd "$SCRIPT_DIR/tests/locust"
    pip install -r requirements.txt
    locust -f locustfile_rest.py --host http://api.messenger.local -u 500 -r 50 -t 5m --headless
}

# ─── Main ────────────────────────────────────────────────────────────────────
CMD="${1:-help}"

case "$CMD" in
    infra)
        deploy_infra
        ;;
    terraform)
        deploy_terraform
        ;;
    argocd)
        deploy_argocd
        ;;
    kafka)
        deploy_kafka
        ;;
    redis)
        deploy_redis
        ;;
    mesh)
        deploy_mesh
        ;;
    rate-limit)
        deploy_rate_limit
        ;;
    ingress)
        deploy_ingress
        ;;
    services)
        deploy_services
        ;;
    observability)
        deploy_observability
        ;;
    all)
        deploy_infra
        deploy_terraform
        deploy_kafka
        deploy_redis
        deploy_mesh
        deploy_rate_limit
        deploy_ingress
        deploy_services
        deploy_observability
        deploy_argocd
        echo "[DONE] Full deployment completed."
        ;;
    test)
        run_tests
        ;;
    *)
        usage
        exit 1
        ;;
esac
