#!/bin/bash

WATCH_KEYMANAGER=false
WATCH_WORKERS=false
START=false
DEPLOY_APPS=false
DEPLOY_INFRA=false
UPLOAD_DOCKER_IMAGES=false
KILL_DOCKER=false
DEPLOY_CRONJOBS=false
JOB_PGSQL_FILL=false
JOB_ETCD_FILL=false
JOB_CLUSTER_TESTS=false
JOB_WATCH=false
RESET=false
LOGS=false
SSH_COMMANDER=false
PGSQL_CONNECT=false
ETDC_CONNECT=false
KAFKA_CONNECT=false
CONNECT_ALL=false

export ETCD_USER="user"
export ETCD_DB="database"
export ETCD_PASSWORD=1G9jdQuVeZ
export ETCD_LOCAL_URI="http://127.0.0.1:2379"
export ETCD_CLUSTER_URI="etcds.demo.svc.cluster.local:5432"

export POSTGRES_USER="user"
export POSTGRES_DB="database"
export POSTGRES_PASSWORD="password"
export POSTGRES_LOCAL_URI="http://127.0.0.1:5432"
export POSTGRES_STRING_LOCAL="postgres://user:password@localhost:5432/database?sslmode=disable"
export POSTGRES_STRING_CLUSTER="postgres://user:password@pgsql-postgresql.demo.svc.cluster.local:5432/database?sslmode=disable"

export KAFKA_USER="user"
export KAFKA_PASSWORD="password"
export KAFKA_LOCAL_URI="http://127.0.0.1:31092"
export KAFKA_CLUSTER_URI="kafka.demo.svc.cluster.local:31092"


# Parse command-line arguments
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --connect_all) CONNECT_ALL=true;;
        --ssh_commander) SSH_COMMANDER=true;;
        --watch_key_manager) WATCH_KEYMANAGER=true;;
        --pgsql_connect) PGSQL_CONNECT=true;;
        --etcd_connect) ETCD_CONNECT=true;;
        --kafka_connect) KAFKA_CONNECT=true;;
        --watch_workers) WATCH_WORKERS=true;;
		--start) START=true;;
        --deploy_infra) DEPLOY_INFRA=true;;
        --upload_docker_images) UPLOAD_DOCKER_IMAGES=true;;
		--deploy_apps) DEPLOY_APPS=true;;
		--job_pgsqfill) JOB_PGSQL_FILL=true;;
		--job_etcdfill) JOB_ETCD_FILL=true;;
		--job_cluster_tests) JOB_CLUSTER_TESTS=true;;
        --job_watch) JOB_WATCH=true;;
        --reset) RESET=true;;
        *) echo "Unknown parameter passed: $1"; exit 1;;
    esac
    shift
done

if [ "$SSH_COMMANDER" = true ]; then
    COMMAND_POD=$(kubectl get pods -l app=command -n=demo | ack -o 'command-hub-[a-z0-9]+-[a-z0-9]+')
    kubectl exec -it ${COMMAND_POD} -n=demo -- /bin/sh
    exit 0
fi

# Reset demo namespace
if [ "$WATCH_WORKERS" = true ]; then
    kubectl logs --follow -l key-manager=true -n=demo --max-log-requests 5
    exit 0
fi

# Reset demo namespace
if [ "$RESET" = true ]; then
    kubectl delete namespace demo
    exit 0
fi

if [ "$DEPLOY_INFRA" = true ]; then
    helm install -f devops/bitnami-values/values-pgsql.yaml pgsql bitnami/postgresql --create-namespace --namespace=demo --atomic
fi

# Install etcd chart
if [ "$DEPLOY_INFRA" = true ]; then
    helm install -f devops/bitnami-values/values-etcd.yaml etcd bitnami/etcd --create-namespace --namespace=demo --atomic
fi

# Install Kafka chart
if [ "$DEPLOY_INFRA" = true ]; then
    helm install kafka wikimedia/kafka-dev --version 0.1.0 --namespace=demo --atomic
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

if [ "$PGSQL_CONNECT" = true ]; then
    kubectl port-forward --namespace demo svc/pgsql-postgresql 5432:5432 & PGPASSWORD="$PGSQL_PW" psql --host 127.0.0.1 -U user -d database -p 5432
    exit 0
fi

if [ "$ETCD_CONNECT" = true ]; then
    kubectl port-forward --namespace demo svc/etcds 2379:2379 &
    exit 0
fi

if [ "$CONNECT_ALL" = true ]; then
    kubectl port-forward --namespace demo svc/kafka 30092:30092 &
    kubectl port-forward --namespace demo svc/etcds 2379:2379 &
    kubectl port-forward --namespace demo svc/pgsql-postgresql 5432:5432 &
    exit 0
fi

if [ "$KAFKA_CONNECT" = true ]; then
    kubectl port-forward --namespace demo svc/kafka 30092:30092 &
    exit 0
fi
