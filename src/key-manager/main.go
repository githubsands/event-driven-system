package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	clientv3 "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	"github.com/gorilla/mux"

	"github.com/sirupsen/logrus"
)

type KeyRotator struct {
	etcdClient *clientv3.Client
	logger     *logrus.Logger
}

type WorkerResponse struct {
	PrivateKey string `json:"privatekey"`
	Working    string `json:"working"`
}

// conditionHandler checks if the worker id has a id within the /in_use/ KV. Returns working true or false
// if so
func (kr *KeyRotator) conditionHandler(w http.ResponseWriter, r *http.Request) {
	kr.logger.Info("Receive com")
	id := r.Header.Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Missing 'id' header")
		return
	}
	kr.logger.Info("Receive condition check from worker: ", id)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	tx := "/in_use/" + id
	resp, err := kr.etcdClient.Get(ctx, tx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Failed to get key from etcd: %v", err)
		return
	}
	if len(resp.Kvs) == 0 {
		w.WriteHeader(http.StatusOK)
		response := WorkerResponse{
			Working: "true",
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}
}

// inUseKeysHandler checks if the worker was in use through HTTP. This handler is used
// when a signing-worker falls over to get its in progress key.
func (kr *KeyRotator) inUseKeysHandler(w http.ResponseWriter, r *http.Request) {
	id := r.Header.Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Missing 'id' header")
		return
	}
	kr.logger.Info("Receive in use check from worker: ", id)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	tx := "/in_use/" + id
	resp, err := kr.etcdClient.Get(ctx, tx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Failed to get key from etcd: %v", err)
		return
	}
	if len(resp.Kvs) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	// NOTE: Our api should be support kvs with more then one value here
	if len(resp.Kvs) > 1 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	inusePrivateKey := string(resp.Kvs[0].Value)
	workerResponse := WorkerResponse{
		PrivateKey: inusePrivateKey,
	}
	responseData, err := json.Marshal(workerResponse)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Failed to marshal response object: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseData)
}

// grabOldestKey grabs the oldest key used in the /private_key_stack and updates the
// in_use stack while doing so.
func (kr *KeyRotator) grabOldestKey(id string) (string, error) {
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

// privateKeyStackHandler is the HTTP handler that grabs the last used key and updates in_use
func (kr *KeyRotator) privateKeyStackHandler(w http.ResponseWriter, r *http.Request) {
	id := r.Header.Get("id")
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Missing 'id' header")
		return
	}
	kr.logger.Info("Accessing the key stack for worker: ", id)
	key, err := kr.grabOldestKey(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Failed to marshal response object: %v", err)
		return
	}
	workerResponse := WorkerResponse{
		PrivateKey: key,
	}
	responseData, err := json.Marshal(workerResponse)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Failed to marshal response object: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseData)
}

func main() {
	cfg := clientv3.Config{
		Endpoints:   []string{os.Getenv("ETCD_CLUSTER_URI")},
		DialTimeout: 5 * time.Second,
		Username:    os.Getenv("ETCD_USER"),
		Password:    os.Getenv("ETCD_PASSWORD"),
	}
	etcdClient, err := clientv3.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create etcd client: %v", err)
	}
	defer etcdClient.Close()
	logger := logrus.New()
	logger.Out = os.Stdout
	kr := &KeyRotator{
		etcdClient: etcdClient,
		logger:     logger,
	}

	r := mux.NewRouter()

	// There are three handlers to this server
	// 1. condition: checks the condition (or state) of a worker to see
	//    if it had an ongoing key during a signing-worker failure
	// 2. in_use_keys: grabs an in progress key for that worker using the workers ID
	// 3. private_key_stack: grabs the oldest key used while updating the in_use stack
	r.HandleFunc("/condition", kr.conditionHandler).Methods("GET")
	r.HandleFunc("/in_use_keys", kr.inUseKeysHandler).Methods("GET")
	////  r.HandleFunc("/private_key_stack", kr.privateKeyStackHandler).Methods("GET")
	srv := &http.Server{
		Handler:      r,
		Addr:         ":8080",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	log.Println("Server is running on port 8080")
	log.Fatal(srv.ListenAndServe())
}
