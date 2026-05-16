terraform {
  required_version = ">= 1.5.0"
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "~> 2.35"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.17"
    }
  }
}

provider "kubernetes" {
  config_path = var.kubeconfig_path
}

provider "helm" {
  kubernetes {
    config_path = var.kubeconfig_path
  }
}

# ─── Namespaces ─────────────────────────────────────────────────────────────
locals {
  namespaces = {
    "chat-service"    = { istio_injection = "enabled" }
    "message-service" = { istio_injection = "enabled" }
    "realtime-gateway"= { istio_injection = "enabled" }
    "kafka"           = { istio_injection = "disabled" }
    "redis"           = { istio_injection = "disabled" }
    "observability"   = { istio_injection = "disabled" }
    "argocd"          = { istio_injection = "disabled" }
    "istio-system"    = { istio_injection = "disabled" }
    "gitlab-runners"  = { istio_injection = "disabled" }
  }
}

resource "kubernetes_namespace" "messenger" {
  for_each = local.namespaces
  metadata {
    name = each.key
    labels = {
      "kubernetes.io/metadata.name" = each.key
      "istio-injection"             = each.value.istio_injection
    }
  }
}

# ─── Service Accounts ───────────────────────────────────────────────────────
locals {
  service_accounts = {
    "chat-service"     = { namespace = "chat-service" }
    "message-service"  = { namespace = "message-service" }
    "realtime-gateway" = { namespace = "realtime-gateway" }
  }
}

resource "kubernetes_service_account" "messenger" {
  for_each = local.service_accounts
  metadata {
    name      = each.key
    namespace = each.value.namespace
  }
  depends_on = [kubernetes_namespace.messenger]
}

# ─── RBAC (least privilege) ─────────────────────────────────────────────────
resource "kubernetes_role" "chat_service" {
  metadata {
    name      = "chat-service-role"
    namespace = "chat-service"
  }
  rule {
    api_groups = [""]
    resources  = ["pods", "services", "configmaps", "secrets"]
    verbs      = ["get", "list", "watch"]
  }
  depends_on = [kubernetes_namespace.messenger]
}

resource "kubernetes_role_binding" "chat_service" {
  metadata {
    name      = "chat-service-binding"
    namespace = "chat-service"
  }
  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "Role"
    name      = kubernetes_role.chat_service.metadata[0].name
  }
  subject {
    kind      = "ServiceAccount"
    name      = "chat-service"
    namespace = "chat-service"
  }
}

resource "kubernetes_role" "message_service" {
  metadata {
    name      = "message-service-role"
    namespace = "message-service"
  }
  rule {
    api_groups = [""]
    resources  = ["pods", "services", "configmaps", "secrets"]
    verbs      = ["get", "list", "watch"]
  }
  depends_on = [kubernetes_namespace.messenger]
}

resource "kubernetes_role_binding" "message_service" {
  metadata {
    name      = "message-service-binding"
    namespace = "message-service"
  }
  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "Role"
    name      = kubernetes_role.message_service.metadata[0].name
  }
  subject {
    kind      = "ServiceAccount"
    name      = "message-service"
    namespace = "message-service"
  }
}

resource "kubernetes_role" "realtime_gateway" {
  metadata {
    name      = "realtime-gateway-role"
    namespace = "realtime-gateway"
  }
  rule {
    api_groups = [""]
    resources  = ["pods", "services", "configmaps", "secrets"]
    verbs      = ["get", "list", "watch"]
  }
  depends_on = [kubernetes_namespace.messenger]
}

resource "kubernetes_role_binding" "realtime_gateway" {
  metadata {
    name      = "realtime-gateway-binding"
    namespace = "realtime-gateway"
  }
  role_ref {
    api_group = "rbac.authorization.k8s.io"
    kind      = "Role"
    name      = kubernetes_role.realtime_gateway.metadata[0].name
  }
  subject {
    kind      = "ServiceAccount"
    name      = "realtime-gateway"
    namespace = "realtime-gateway"
  }
}

# ─── External Secrets Operator (упрощённо) ──────────────────────────────────
# В production интеграция с HashiCorp Vault. Для локальной среды — dummy.
resource "helm_release" "external_secrets" {
  name       = "external-secrets"
  repository = "https://charts.external-secrets.io"
  chart      = "external-secrets"
  version    = "0.14.0"
  namespace  = "observability"
  create_namespace = false
  depends_on = [kubernetes_namespace.messenger]
}
