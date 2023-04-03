#!/bin/zsh

go build
docker build -t jobs:latest .
minikube image load jobs:latest .
