#!/bin/zsh

go build
docker build -t batchreader:latest .
