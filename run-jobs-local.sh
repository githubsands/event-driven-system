export ETCD_USER="root"
export ETCD_DB="database"
export ETCD_PASSWORD="1G9jdQuVeZ"
export ETCD_LOCAL_URI="http://127.0.0.1:2379"
export ETCD_CLUSTER_URI="etcds.demo.svc.cluster.local:2379"

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

# RUN $HOME/rustsrc/axelar-core/cluster-command.sh --connect_all before proceeding

$HOME/rust/src/axelar-core/src/jobs/jobs $@
