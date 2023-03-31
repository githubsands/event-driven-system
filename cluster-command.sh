#!/bin/bash

# Initialize variables
UPLOAD_DOCKER_IMAGES=false
PROVISION=false
START=false
DEPLOY_APPS=false
DEPLOY_INFRA=false
DEPLOY_CRONJOBS=false
JOB_PGSQL_FILL=false
JOB_ETCD_FILL=false
JOB_CLUSTER_TESTS=false
JOB_WATCH=false
RESET=false
LOGS=false

# Parse command-line arguments
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --upload_docker_images) UPLOAD_DOCKER_IMAGES=true;;
        --provision) PROVISION=true;;
		--start) START=true;;
        --deploy_infra) DEPLOY_INFRA=true;;
		--deploy_apps) DEPLOY_APPS=true;;
		--job_pgsqfill) JOB_PGSQL_FILL=true;;
		--job_etcdfill) JOB_ETCD_FILL=true;;
		--job_cluster_tests) JOB_CLUSTER_TESTS=true;;
        --job_watch) JOB_WATCH=true;;
        --reset) RESET=true;;
        --logs) LOGS=true;;
        *) echo "Unknown parameter passed: $1"; exit 1;;
    esac
    shift
done

if [ "$PROVISION" = provision ]; then
    helm repo add twuni https://helm.twun.io
	helm repo add bitnami https://charts.bitnami.com/bitnami
    exit 0
fi

# Reset demo namespace
if [ "$START" = start ]; then
    exit 0
fi

# Reset demo namespace
if [ "$RESET" = true ]; then
    kubectl delete namespace demo
    exit 0
fi

# Install PostgreSQL chart
if [ "$DEPLOY_INFRA" = true ]; then
    helm install -f devops/bitnami-values/values-pgsql.yaml demo-pgsql bitnami/postgresql --create-namespace --namespace=demo --atomic
    helm install twuni/docker-registry name=docker-registry
fi

if ["$UPLOAD_DOCKER_IMAGES" == "upload_docker_images"]; then
    export DOCKER_REGISTRY_POD_NAME=$(kubectl get pods --namespace default -l "app=docker-registry" -o jsonpath="{.items[0].metadata.name}")
    kubectl -n default port-forward $DOCKER_REGISTRY_POD_NAME 8080:5000 &
    exit 0
fi

# Install etcd chart
if [ "$DEPLOY_INFRA" = true ]; then
    helm install -f devops/bitnami-values/values-etcd.yaml demo-etcd bitnami/etcd --create-namespace --namespace=demo --atomic
fi

# Install Kafka chart
if [ "$DEPLOY_INFRA" = true ]; then
    helm install -f devops/bitnam-values/values-kafka.yaml demo-kafka bitnami/kafka --create-namespace --namespace=demo --atomic
fi

# Install Kafka chart
if [ "$DEPLOY_INFRA" = true ]; then
    helm install -f devops/bitnam-values/values-kafka.yaml demo-kafka bitnami/kafka --create-namespace --namespace=demo --atomic
fi

if [ "$DEPLOY_APPS" = true ]; then
	helm install -f signing-service
	helm install -f signers-lock
	helm install -f io-reader
	helm install -f io-writer
fi

if [ "$DEPLOY_CRONJOBS" = true ]; then
	helm install -f cron-jobs
fi

if [ "$JOB_PGSQL_FILL" = true ]; then
	kubectl apply -f devops/cron-jobs/templates/seed-db-cronjob.yaml
fi

if [ "$JOB_ETCD_FILL" = true ]; then
	kubectl apply -f devops/cron-jobs/templates/seed-etcd-cronjob.yaml
fi

if [ "$JOB_CLUSTER_TESTS" = true ]; then
	kubectl apply -f devops/cron-jobs/templates/job-cluster-tests.yaml
fi

if [ "$JOB_WATCH" = true ]; then
    kubectl get jobs --watch
fi

# Display logs for all containers in demo namespace
if [ "$LOGS" = true ]; then
	kubectl logs --max-log-requests=100 -l app=demo --all-containers -f --namespace demo
    exit 0
fi

# Display logs for all containers in demo namespace
if [ "$LOGS_KAFKA" = true ]; then
	kubectl logs --max-log-requests=100 -l app=demo --all-containers -f --namespace demo
    exit 0
fi

# Display logs for all containers in demo namespace
if [ "$LOGS_TEST" = true ]; then
	kubectl logs --max-log-requests=100 -l app=demo --all-containers -f --namespace demo
    exit 0
fi
