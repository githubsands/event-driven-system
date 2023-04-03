#!/bin/zsh

go build
docker build -t verify-tx-worker:latest .
minikube image load  verify-tx-worker:latest .
