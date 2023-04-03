package main

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/clientv3/concurrency"
)

func main() {
	// Create an etcd client
	client, err := clientv3.New(clientv3.Config{
		Endpoints: []string{"localhost:2379"},
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	defer client.Close()

	// Set the key to retrieve
	key := "mykey"

	// Get the value for the key
	_, err = client.Get(context.Background(), key)
	if err != nil {
		fmt.Println(err)
		return
	}
	push_private_keys(client)
	grabLastUsedKey(client, "3")
	grabLastUsedKeyTransaction(client, "4")
}

func sort_test(client *clientv3.Client) {
	// Retrieve all keys with a prefix
	resp, err := client.Get(context.Background(), "private_key_stack/", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend))
	if err != nil {
		fmt.Println(err)
		return
	}

	if len(resp.Kvs) == 0 {
		fmt.Println("No keys found with prefix")
		return
	}

	// Print all keys and values
	for _, kv := range resp.Kvs {
		fmt.Printf("Key: %s, Value: %s\n", kv.Key, kv.Value)
	}

	// Find the smallest key
	fmt.Printf("Smallest key for prefix: %s\n", resp.Kvs[0].Key)
}

func randString() string {
	const letters = "0123456789akjasdkf934kjasdj93"
	b := make([]byte, 10)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func push_private_keys(client *clientv3.Client) {
	privateKeys := make([]string, 10)
	for i := range privateKeys {
		privateKeys[i] = randString()
	}
	for _, pk := range privateKeys {
		timestamp := time.Now().Unix()
		timestampStr := strconv.FormatInt(timestamp, 10)
		fmt.Println(pk)
		_, err := client.KV.Put(context.Background(), "/private_key_stack/"+timestampStr, pk)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("push key success")
	}
}

func grabLastUsedKey(client *clientv3.Client, id string) {
	resp, err := client.Get(context.Background(), "/private_key_stack/", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend))
	if err != nil {
		fmt.Println(err)
		return
	}
	if len(resp.Kvs) == 0 {
		fmt.Println("No keys found with prefix")
		return
	}
	for _, kv := range resp.Kvs {
		fmt.Printf("Key: %s, Value: %s\n", kv.Key, kv.Value)
	}
	oldestUsedKey := resp.Kvs[0].Value
	fmt.Printf("last used key: %s\n", oldestUsedKey)
	timestamp := time.Now().Unix()
	timestampStr := strconv.FormatInt(timestamp, 10)
	lastUsedKey := oldestUsedKey
	client.KV.Put(context.Background(), "/private_key_stack/"+timestampStr, string(lastUsedKey))
	client.KV.Delete(context.Background(), "/private_key_stack/"+string(resp.Kvs[0].Key))
	client.KV.Put(context.Background(), "/in_use/"+id, string(lastUsedKey))
}

func grabLastUsedKeyTransaction(client *clientv3.Client, id string) {
	resp, err := client.Get(context.Background(), "/private_key_stack/", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend))
	if err != nil {
		fmt.Println(err)
		return
	}
	if len(resp.Kvs) == 0 {
		fmt.Println("No keys found with prefix")
		return
	}
	for _, kv := range resp.Kvs {
		fmt.Printf("Key: %s, Value: %s\n", kv.Key, kv.Value)
	}
	oldestUsedKey := resp.Kvs[0].Value
	oldestUsedKeyStr := string(oldestUsedKey)
	oldestUsedKeyTime := string(resp.Kvs[0].Key)
	txnResp2, err := client.Txn(context.Background()).
		Then(
			clientv3.OpPut("/in_use/"+id, oldestUsedKeyStr),
			clientv3.OpPut("/private_key_stack/"+strconv.FormatInt(time.Now().Unix(), 10), ""),
			clientv3.OpDelete(oldestUsedKeyTime),
		).
		Commit()
	if err != nil {
		fmt.Println(err)
		return
	}
	if !txnResp2.Succeeded {
		fmt.Println("Error: Transaction failed.")
		return
	}
	fmt.Printf("last used key: %s\n", oldestUsedKey)
}

func grabLastUsedKeyTransactionLock(client *clientv3.Client, id string) {
	session, err := concurrency.NewSession(client)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer session.Close()
	mutex := concurrency.NewMutex(session, "/lock/")
	if err := mutex.Lock(context.Background()); err != nil {
		fmt.Println(err)
		return
	}
	defer func() {
		if err := mutex.Unlock(context.Background()); err != nil {
			fmt.Println(err)
		}
	}()
	resp, err := client.Get(context.Background(), "/private_key_stack/", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortAscend))
	if err != nil {
		fmt.Println(err)
		return
	}
	if len(resp.Kvs) == 0 {
		fmt.Println("No keys found with prefix")
		return
	}
	for _, kv := range resp.Kvs {
		fmt.Printf("Key: %s, Value: %s\n", kv.Key, kv.Value)
	}
	oldestUsedKey := resp.Kvs[0].Value
	oldestUsedKeyStr := string(oldestUsedKey)
	oldestUsedKeyTime := string(resp.Kvs[0].Key)
	txnResp2, err := client.Txn(context.Background()).
		Then(
			clientv3.OpPut("/in_use/"+id, oldestUsedKeyStr),
			clientv3.OpPut("/private_key_stack/"+strconv.FormatInt(time.Now().Unix(), 10), ""),
			clientv3.OpDelete(oldestUsedKeyTime),
		).
		Commit()
	if err != nil {
		fmt.Println(err)
		return
	}
	if !txnResp2.Succeeded {
		fmt.Println("Error: Transaction failed.")
		return
	}
}
