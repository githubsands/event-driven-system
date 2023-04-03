FROM golang:1.17-alpine

ENV ETCD_USER="user"
ENV ETCD_DB="database"
ENV ETCD_PASSWORD="1G9jdQuVeZ"
ENV ETCD_CLUSTER_URI="etcds.demo.svc.cluster.local:5432"

ENV POSTGRES_USER="user"
ENV POSTGRES_DB="database"
ENV POSTGRES_PASSWORD="password"
ENV POSTGRES_STRING_CLUSTER="postgres://user:password@pgsql-postgresql.demo.svc.cluster.local:5432/database?sslmode=disable"

ENV KAFKA_USER="user"
ENV KAFKA_PASSWORD="password"
ENV KAFKA_CLUSTER_URI="kafka.demo.svc.cluster.local:31092"

RUN apk add --no-cache bash postgresql-client curl git && \
    apk add --no-cache ca-certificates && \
    update-ca-certificates && \
    go get -u github.com/Shopify/sarama && \
    apk del git && \
    echo "#!/bin/bash\nsleep 157680000" > /usr/local/bin/sleep.sh && \
    chmod +x /usr/local/bin/sleep.sh

CMD ["/bin/sh", "-c", "sleep 157680000"]
