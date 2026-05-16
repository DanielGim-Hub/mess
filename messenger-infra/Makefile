.PHONY: help build-infra build-services up down deploy-k8s test clean

help:
	@echo "Messenger Infrastructure Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  build-infra     Build Docker images for infrastructure (Kafka, Valkey, etc.)"
	@echo "  build-services  Build Docker images for microservices"
	@echo "  up              Start local environment via Docker Compose"
	@echo "  down            Stop Docker Compose environment"
	@echo "  deploy-k8s      Deploy all components to Kubernetes"
	@echo "  test            Run Locust load tests"
	@echo "  clean           Remove all Docker images and volumes"

build-infra:
	@echo "Infrastructure images are pulled from registries."

build-services:
	docker build -t localhost:5000/chat-service:latest -f docker/chat-service/Dockerfile ../chatService-main
	docker build -t localhost:5000/message-service:latest -f docker/message-service/Dockerfile ../message_service-main
	docker build -t localhost:5000/realtime-gateway:latest -f docker/realtime-gateway/Dockerfile ../realtime_service_deprecated-main

up:
	docker compose up -d

down:
	docker compose down -v

deploy-k8s:
	./deploy.sh all

test:
	./deploy.sh test

clean:
	docker compose down -v --rmi all
	docker system prune -f
