#!/bin/zsh

go build
docker build -t siging-worker:latest-demo .
