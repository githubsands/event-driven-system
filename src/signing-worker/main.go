package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gorilla/mux"
	kafka "github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/concurrency"
)

var UNSIGNED_RECORDS_TOPIC = "u-records'"
var SIGNED_RECORDS_TOPIC = "s-records'"

type keyManager struct {
	httpClient *http.Client
	privateKey *ecdsa.PrivateKey
	logger     *logrus.Logger
	uri        string
}

func getKeyManagerURI() string {
	value := os.Getenv("KEY_MANAGER_URI")
	if value == "" {
		log.Fatalf("user must set KEY_MANAGER_URI")
	}
	return value
}

func newKeyManager(logger *logrus.Logger) *keyManager {
	km := new(keyManager)
	km.httpClient = &http.Client{}
	km.logger = logger
	km.uri = getKeyManagerURI()
	return km
}

type keyRotatorServerResponse struct {
	Working    string `json:"working"`
	PrivateKey string `json:"privateKey"`
}

func main() {
	logger := logrus.New()
	logger.Out = os.Stdout
	workerID := new(int)
	*workerID = int(getWorkerIDEnv())
	ignite := getIgnition()
	batchSize := new(int)
	*batchSize = getBatchSize()
	log.Printf("unsignedRecordsTopic: %s", UNSIGNED_RECORDS_TOPIC)
	log.Printf("signedRecordsTopic: %s", SIGNED_RECORDS_TOPIC)
	log.Printf("batchSize: %s", batchSize)
	log.Printf("workerID: %s", workerID)
	log.Printf("ignite: %s", ignite)
	ctx := context.Background()
	wg := sync.WaitGroup{}
	wg.Add(1)
	signerService := NewSignerSvc(logger, batchSize, workerID, ignite)
	signerService.Start(ctx, ignite)
	wg.Wait()
}

func getWorkerIDEnv() int64 {
	value := os.Getenv("WORKER_ID")
	if value == "" {
		log.Fatalf("user must set WORKER_ID")
	}
	val, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("Failed to convert WORKER_ID to int")
	}
	return int64(val)
}

func getIgnition() bool {
	value := os.Getenv("IGNITE")
	if value == "" {
		log.Fatalf("user must set IGNITE")
	}
	val, err := strconv.ParseBool(value)
	if err != nil {
		log.Fatalf("Failed to convert IGNITE to int")
	}
	return val
}

func getBatchSize() int {
	value := os.Getenv("BATCH_SIZE")
	if value == "" {
		log.Fatalf("user must set BATCH_SIZE")
	}
	val, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("Failed to convert BATCH_SIZE to int")
	}
	return val
}

func getKafkaURI() string {
	value := os.Getenv("KAFKA_URI")
	if value == "" {
		log.Fatalf("user must set KAFKA_URI")
	}
	return value
}

type SignerSvc struct {
	logger                 *logrus.Logger
	inboundServer          *inboundServer
	kafkaBatchWorkerEngine *KafkaBatchWorkerEngine
}

func NewSignerSvc(logger *logrus.Logger, batchSize *int, workerID *int, ignition bool) *SignerSvc {
	starter := make(chan struct{}, 0)
	sw := new(SignerSvc)
	if ignition {
		ib := newInboundServer(logger, starter, int64(*workerID))
		sw.inboundServer = ib
	}
	ke, err := NewKafkaBatchWorkerEngine(logger, batchSize, workerID, starter, getKafkaURI())
	if err != nil {
		logger.Fatalf("failed to create KafkaBatchWorkerEngine")
	}
	time.Sleep(3 * time.Second)
	if !ignition {
		logger.Info("Starting kafka batch workers without ignition")
		go func() {
			err := ke.start()
			if err != nil {
				log.Fatalf("Failed to start KafkaBatchWorkerEngine", err)
			}
		}()
		time.Sleep(3 * time.Second)
		starter <- struct{}{}
		close(starter)
	}
	sw.logger = logger
	sw.kafkaBatchWorkerEngine = ke
	return sw
}

func (ss *SignerSvc) Start(ctx context.Context, ignition bool) {
	if ignition {
		go func() {
			ss.logger.Info("starting signer service")
			if err := ss.inboundServer.start(); err != nil {
				log.Fatalf("failed to start inboundServer")
			}
		}()
	}
	ss.logger.Info("Starting kafka batch worker engine")
	if err := ss.kafkaBatchWorkerEngine.start(); err != nil {
		log.Fatalf("failed to start batchWorkerEngine")
	}
	select {
	case <-ctx.Done():
		if err := ss.inboundServer.httpServer.Shutdown(ctx); err != nil {
			log.Fatalf("Failed to shutdown http server")
		}
		return
	}
}

type inboundServer struct {
	workerID   int64
	m          sync.Mutex
	keyNumber  int
	started    bool
	httpServer *http.Server
	logger     *logrus.Logger
	kmStarter  chan<- struct{}
}

func newInboundServer(logger *logrus.Logger, kafkaStarter chan<- struct{}, workerID int64) *inboundServer {
	r := mux.NewRouter()
	httpServer := &http.Server{Addr: ":8080", Handler: r}
	is := &inboundServer{httpServer: httpServer, kmStarter: kafkaStarter}
	r.HandleFunc("/ignite", is.IgniteHandler)
	is.workerID = workerID
	is.kmStarter = kafkaStarter
	is.logger = logger
	return is
}

func (s *inboundServer) IgniteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !s.started {
		s.m.Lock()
		defer close(s.kmStarter)
		w.WriteHeader(http.StatusAccepted)
		workerIDString := strconv.FormatInt(s.workerID, 1)
		w.Write([]byte("Started igniter service for worker: " + workerIDString))
		s.kmStarter <- struct{}{}
		s.started = true
		s.m.Unlock()
	}
	w.WriteHeader(http.StatusAlreadyReported)
	w.Write([]byte("This signing service has already been ignited"))
}

func (s *inboundServer) start() error {
	if err := s.httpServer.ListenAndServe(); err != nil {
		fmt.Printf("Error starting server: %v", err)
	}
	return nil
}

type batchWorker struct {
	logger      *logrus.Logger
	batchID     *int
	batchSize   *int
	keyManager  *keyManager
	kafkaWriter *kafka.Writer
	kafkaReader *kafka.Reader
}

type KafkaBatchWorkerEngine struct {
	workers []*batchWorker
	logger  *log.Logger
	starter <-chan struct{}
	broker  string
}

func NewKafkaBatchWorkerEngine(logger *log.Logger, batchSize *int, workerID *int, starter <-chan struct{}, broker string) (*KafkaBatchWorkerEngine, error) {
	kc := &KafkaBatchWorkerEngine{logger: logger, starter: starter, broker: broker, workers: make([]*batchWorker, 1)}
	kc.workers = append(kc.workers, newBatchWorker(logger, batchSize, workerID, broker))
	return kc, nil
}

func newBatchWorker(logger *log.Logger, batchSize *int, workerID *int, broker string) *batchWorker {
	// TODO: Set idempotence to true somewhere
	// RequiredAcks: Number of acknowledges from partition replicas required before receiving
	// a response to a produce request. The default is -1, which means to wait for
	// all replicas, and a value above 0 is required to indicate how many replicas
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:      []string{broker},
		Topic:        SIGNED_RECORDS_TOPIC,
		Balancer:     kafka.Murmur2Balancer{},
		BatchSize:    *batchSize,
		RequiredAcks: -1, // Wait for all replicas (worker or zookeeper replicas?) to acknowledge that they have received this commit before proceeding
	})
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{broker},
		Topic:          UNSIGNED_RECORDS_TOPIC,
		MinBytes:       *batchSize, // NOTE: Have a consistent batch size for reads
		MaxBytes:       *batchSize,
		GroupID:        fmt.Sprint(workerID), // Configure consumer to a unique group.instance id making the consumer a static member of the group. so when consumer falls over it rejoins the same partition
		CommitInterval: 0,                    // NOTE: Set to zero for sync committing. // NOTE: CommitInterval uses commitLoopInterval leverages useSyncCommits: https://github.com/segmentio/kafka-go/blob/main/reader.go#L280

	})
	bw := &batchWorker{}
	bw.logger = logger
	bw.keyManager = newKeyManager(logger)
	bw.batchID = workerID
	bw.batchSize = batchSize
	bw.kafkaWriter = writer
	bw.kafkaReader = reader
	return bw
}

func (kc *KafkaBatchWorkerEngine) start(ctx context.Conext) error {
	for {
		kc.logger.Info("Waiting for start signal")
		select {
		case <-kc.starter:
			kc.logger.Info("Received starting signal - starting up batch workers")
			for i := 0; i <= len(kc.workers)-1; i++ {
				// TODO: Each instance (or server process) only handles one "batch worker" for now.
				var workerNum int = i
				go func(int) {
					if kc.workers[workerNum] == nil {
						return
					}
					err := kc.workers[workerNum].run()
					if err != nil {
						fmt.Print("Received starting a batch worker", err)
						return
					}
					kc.workers[workerNum].runKafkaPipe()
				}(workerNum)
			}
			kc.logger.Info("Received signal to start closing this go routine")
			return nil
		default:
		}
	}
}

func (bw *batchWorker) run() error {
	bw.logger.Info("Running batch worker - checking for states:", bw.batchID)
	running, err := bw.keyManager.checkWorkerCondition(bw.batchID)
	switch running {
	case true:
		err = bw.keyManager.grabKeyWorkerFallOver(bw.batchID)
		if err != nil {
			return err
		}
	case false:
		err = bw.keyManager.grabKeyStart(bw.batchID)
		if err == nil {
			return err
		}
	default:
	}
	return nil
}

func (bw *batchWorker) runKafkaPipe() {
	bw.logger.Info("running kafka pipe for worker:", bw.batchID)
	/// var err error
	// TODO: Allocate message size here and reuse it for every batc
	// instead of appending a new batch array each time
	var kafkaMessages []kafka.Message
KAFKA_RETRY:
	for {
		// finish this batch and rotate the key
		// TODO: Implement batch reads of 500
		if len(kafkaMessages) == *bw.batchSize {
			// Batch is over. Write it to kafka and grab a new key synchronously - our new
			// batch is dependent off this key.
			bw.kafkaWriter.WriteMessages(context.Background(), kafkaMessages...)
			bw.keyManager.grabKeyRotate(*bw.batchID)
		}
		message, err := bw.kafkaReader.ReadMessage(context.Background())
		if err != nil {
			break KAFKA_RETRY
		}
		signedTransaction, err := bw.keyManager.signTransaction(message.Value)
		if err != nil {
			continue
		}
		msg := kafka.Message{Value: signedTransaction}
		kafkaMessages = append(kafkaMessages, msg)
		// Continue gathering kafka messages until our batch size is reached
	}
	if err := bw.kafkaReader.Close(); err != nil {
		log.Fatal("failed to close reader:", err)
	}
}

// grabOldestKey grabs the oldest key used in the /private_key_stack and updates the
// in_use stack while doing so.
func (kr *KeyManager) grabOldestKey(id string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// 1. Start a new session
	session, err := concurrency.NewSession(kr.etcdClient) // NOTE: ISSUE WITH IMPORT HERE
	if err != nil {
		return "", err
	}
	defer session.Close()
	// 2. Lock our session both /private_key_stack and /in_use_keys
	mutex := concurrency.NewMutex(session, "/")
	if err := mutex.Lock(ctx); err != nil {
		return "", err
	}
	mutex.Unlock(ctx)
	// 3. Grab our oldest key - we cant use it as can operation within a transaction due to needing it for the operation
	resp, err := kr.etcdClient.Get(context.Background(), "/private_key_stack/", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend))
	if err != nil {
		return "", err
	}
	if len(resp.Kvs) == 0 {
		return "", err
	}

	kv := resp.Kvs[0]
	oldestUsedKey := string(kv.Value)
	timeUsed := string(kv.Key)
	// 4. Use our oldest key grab previously to resort the stack and update workers inuse key
	_, err = kr.etcdClient.Txn(context.Background()).
		Then(
			//	clientv3.OpPut("/in_use/"+id, oldestUsedKey),                                                  // update this workers inuse key with the oldest key we found at the beginning of this function
			clientv3.OpDelete("/private_key_stack/"+timeUsed),                                             // delete the keys old time
			clientv3.OpPut("/private_key_stack/"+strconv.FormatInt(time.Now().Unix(), 10), oldestUsedKey), // update the keys new time
		).
		Commit()
	if err != nil {
		mutex.Unlock(ctx)
		return "", err
	}
	/*
		if txnResp.Succeded || err != nil {
			mutex.Unlock(ctx)
			return oldestUsedKey, nil
		}
	*/
	// 5. Unlock the stack
	mutex.Unlock(ctx)
	return "", err
}

// checkWorkerCondition checks the worker condition from the upstream key manager. did this worker fail previously or
// are we starting the system at an initial condition t=0.  Note that currently this system in its entirety does not
// currently have scaled workers (or instances) conditions when t != 0.  all known workers are known at system boot/deployment
// time.
func (km *keyManager) checkWorkerCondition(batchID *int) (bool, error) {
	km.logger.Info("KeyManager: checking worker condition from key manager server for worker: ", batchID)
	var err error
	var resp http.Response
	var response keyRotatorServerResponse
	// keep retrying until we get a response from the key-manager - the key manager could of fallen over lets wait for it to restart.
	// worker instances are isolated from key manager fall overs.  their only reaction to key manager fall overs is to retry infinitely
	// until a key manager instance is rebooted and is ready again to start serving keys.
HTTP_KEY_MANAGER_RETRY:
	for {
		url := km.uri + "/condition"
		req, err := http.NewRequest("GET", km.uri+"/condition", nil)
		req.Header.Add("id", strconv.FormatInt(int64(*batchID), 10))
		resp, err := km.httpClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			var response keyRotatorServerResponse
			err = json.NewDecoder(resp.Body).Decode(&response)
			km.logger.Info("KeyManager: Key manager feedback received")
			break HTTP_KEY_MANAGER_RETRY
		}
		km.logger.Info("keymanager: received status code:", resp.StatusCode)
		if err != nil {
			km.logger.Info("keymanager: received error code:", err.Error())
		}
		km.logger.Info("keymanager: retrying key manager server. state: worker_conditon at:", url)
	}
	defer resp.Body.Close()
	if err != nil {
		return false, err
	}
	if response.Working != "" {
		return strconv.ParseBool(response.Working)
	}
	return false, errors.New("Failed to get a working response from key manager")
}

func (km *keyManager) grabKeyStart(batchID *int) error {
	var err error
	var resp http.Response
	var response keyRotatorServerResponse
HTTP_KEY_MANAGER_RETRY:
	for {
		url := km.uri + "/private_key_stack"
		req, err := http.NewRequest("GET", url, nil)
		req.Header.Add("id", strconv.FormatInt(int64(*batchID), 1))
		resp, err := km.httpClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			var response keyRotatorServerResponse
			err = json.NewDecoder(resp.Body).Decode(&response)
			km.logger.Info("KeyManager: Key manager feedback received")
			break HTTP_KEY_MANAGER_RETRY
		}
		km.logger.Info("keymanager: received status code:", resp.StatusCode)
		if err != nil {
			km.logger.Info("keymanager: received error code:", err.Error())
		}
		km.logger.Info("keymanager: retrying key manager server. state: key_conditon at %v", url)
	}
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	privateKey, err := crypto.HexToECDSA(response.PrivateKey)
	if err != nil {
		log.Fatalf("failed to make private key from initital grabKey condition%v", err)
	}
	km.privateKey = privateKey
	km.logger.Info("State: worker grab key start. Grabbed key for worker state grab key start complete.", batchID)
	return nil
}

// grabKeyWorkerFallOver grabs a key in the case a worker has fallen over. this is a state when the system i
// not running smoothly
func (km *keyManager) grabKeyWorkerFallOver(batchWorkerID *int) error {
	var err error
	var resp http.Response
	var response keyRotatorServerResponse
	// keep retrying until we get a response from the key-manager - the key manager could of fallen over lets wait for it to restart
HTTP_KEY_MANAGER_RETRY:
	for {
		url := km.uri + "/in_use_keys"
		req, err := http.NewRequest("GET", url, nil)
		req.Header.Add("id", strconv.FormatInt(int64(*batchWorkerID), 1))
		resp, err := km.httpClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			var response keyRotatorServerResponse
			err = json.NewDecoder(resp.Body).Decode(&response)
			km.logger.Info("KeyManager: Key manager feedback received")
			break HTTP_KEY_MANAGER_RETRY
		}
		km.logger.Info("keymanager: received status code:", resp.StatusCode)
		if err != nil {
			km.logger.Info("keymanager: received error code:", err.Error())
		}
		km.logger.Info("keymanager: retrying key manager server. state: worker_fall_over at %v", url)
	}
	defer resp.Body.Close()
	if err != nil {
		return err
	}
	privateKey, err := crypto.ToECDSA([]byte(response.PrivateKey))
	if err != nil {
		return err
	}
	km.logger.Info("State: worker fall over. Grabbed key for worker state key rotate complete.", batchWorkerID)
	km.privateKey = privateKey
	return nil
}

// grabKeyRotate grabs a key in the case a worker needs a new key. this is steady state behavior
// when tn=tn-1+1
func (km *keyManager) grabKeyRotate(batchID int) error {
	km.logger.Info("State: worker key rotate. Grabbing key for worker key rotate.", batchID)
	var err error
	var resp http.Response
	var response keyRotatorServerResponse
	// keep retrying until we get a response from the key-manager - the key manager could of fallen over lets wait for it to restart
HTTP_KEY_MANAGER_RETRY:
	for {
		url := km.uri + "/private_key_stack"
		req, err := http.NewRequest("GET", url, nil)
		resp, err := km.httpClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			km.logger.Info("State: worker key rotate. Grabbed key for worker key rotate.", batchID)
			var response keyRotatorServerResponse
			err = json.NewDecoder(resp.Body).Decode(&response)
			break HTTP_KEY_MANAGER_RETRY
		}
		km.logger.Info("keymanager: received status code:", resp.StatusCode)
		if err != nil {
			km.logger.Info("keymanager: received error code:", err.Error())
		}
		km.logger.Info("keymanager: retrying key manager server. state: grab_key_rotate at %v", url)
	}
	defer resp.Body.Close()
	if err != nil {
		return nil
	}
	privateKey, err := crypto.ToECDSA([]byte(response.PrivateKey))
	if err != nil {
		return err
	}
	km.logger.Info("State: worker key rotate. Grabbed key for worker key rotate complete.", batchID)
	km.privateKey = privateKey
	return nil
}

func (km *keyManager) signTransaction(txBytes []byte) ([]byte, error) {
	// NOTE: For  sake of the demo random transactions are created here. In the future it would be better to
	// create a custom deserializer from a kafka message that has unique address, gas cost, etc.
	toAddress := common.HexToAddress("0x4592d8f8d7b001e72cb26a73e4fa1806a51ac79d")
	tx := types.NewTransaction(1, toAddress, nil, 1, nil, txBytes)
	chainID := big.NewInt(1000000000000000000)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), km.privateKey)
	if err != nil {
		log.Fatalf("failed to sign transaction: %v", err)
	}
	signedTxBytes, err := signedTx.MarshalBinary()
	if err != nil {
		log.Fatalf("failed to make marshal transaction: %v", err)
	}
	return signedTxBytes, nil
}
