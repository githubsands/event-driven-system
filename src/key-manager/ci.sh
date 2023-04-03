#!/bin/zsh

go build
docker build -t key-manager:latest .
