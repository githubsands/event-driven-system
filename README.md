# Summary

A event driven system that leverages etcd, postgres, kafka, kubernetes, and various services.

etcd: etcd holds necessary information on state failures: what worker a key is using when failed
      as well as a list of all keys leveraged by the system. private keys key value store are locked while in use
      and are queried by the oldest unix time stamp

postgres: holds mocked ethereum transactions

kafka: serves as the event bus for the system

kubernetes: container scheduler or orchestration tool. helps handles fall overs of various go services and service discovery

services: see services segment of this document

# System design

Intended to be applied as a financial system from the start etcd was chosen over redis for distrubuted locks to guarantee no lost writes occur during a system partition.
Redis is AP (available & partitioned) system whereas etcd is a CP (consistent & partitioned) system. Although etcd may be less available it provides us with consistency
guarantees needed in financial systems.

# Project Layout

/src: holds golang src code for various services

/devops: holds helm charts for deploying services and cluster job task

# Services

(1) signing-worker: kafka producer and consumer, a pipe, that signs transactions.

(2) key-manager: service that signing-workers cordinate around. should maybe have a queue in future iterations.

(3) io-batch-reader: postgres to kafka consumer ethereum bridge

(4) tx-verify: verify that transactions were signed (unimplemented). tx-verify is IO heavy and should only be ran in staging. seems necessary to verify transaction signing rules (i.e only once) in capital intensive environments. maybe service should be ran in PROD also until system is more widely understood?

(5) key-manager-rs: a WIP rust implementation of key-manager

# Other

jobs: errands that have to be done around the clsuter such as stock it with transactions, and seed databases.

# Cluster

## Internal_IPS:

(1) etcd: etcds.demo.svc.cluster.local:2379

(2) postgres: pgsql-postgressql.demo.svc.cluster.local:5432

(3) kafka: kafka.demo.svc.cluster.local:31092

(4) key-manager: key-manager.demo.svc.cluster.local:8080
