package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	_ "github.com/lib/pq"
	"go.etcd.io/etcd/clientv3"
)

var POSTGRES_DB = os.Getenv("PGSQL_DB")
var POSTGRES_PASSWORD = os.Getenv("PGSQL_PASSWORD")
var POSTGRES_USER = os.Getenv("PGSQL_USER")
var POSTGRES_LOCAL_URI = os.Getenv("ETCD_LOCAL_URI")
var POSTGRES_STRING = os.Getenv("POSTGRES_STRING")

var ETCD_USER = os.Getenv("ETCD_USER")
var ETCD_DB = os.Getenv("ETCD_DB")
var ETCD_PASSWORD = os.Getenv("ETCD_PASSWORD")
var ETCD_LOCAL_URI = os.Getenv("ETCD_LOCAL_URI")

func main() {
	seedTablesFlag := flag.Bool("seed_tables", false, "Seed tables to postgres")
	uploadTransactionsFlag := flag.Bool("upload_transactions", false, "Upload transactions to postgres")
	uploadPrivateKeyFlag := flag.Bool("upload_privatekeys", false, "Upload private keys to postgres and etcd")
	igniteFlag := flag.Bool("ignite", false, "Starts workers")
	checkETCDFlag := flag.Bool("check_etcd", false, "Checks etcd connection")
	deleteTablesFlag := flag.Bool("delete_tables", false, "Deletes private keys and transaction tables")

	flag.Parse()
	if *uploadTransactionsFlag {
		err := uploadTransactions()
		if err != nil {
			panic(err)
		}
	}
	if *uploadPrivateKeyFlag {
		keyPairs := createKeyPairs(50)
		err := uploadKeyPairs(keyPairs)
		if err != nil {
			panic(err)
		}
		err = seedETCD(keyPairs)
		if err != nil {
			panic(err)
		}
	}
	if *seedTablesFlag {
		err := seedTables()
		if err != nil {
			panic(err)
		}
	}
	if *igniteFlag {
		err := ignite()
		if err != nil {
			panic(err)
		}
	}
	if *checkETCDFlag {
		checkETCD()
	}

	if *deleteTablesFlag {
		deleteTables()
	}
}

func uploadTransactions() error {
	db, err := sql.Open("postgres", os.Getenv("POSTGRES_STRING_LOCAL"))
	if err != nil {
		panic(err)
	}
	toAddress := common.HexToAddress("0x8d8fbb38541e21a5e5e5c8a5d5a5c35f5c2ebf8b")
	value := big.NewInt(1000000000000000000) // 1 ETH
	gasLimit := uint64(21000)
	gasPrice := big.NewInt(112)
	// Stock github with uniswap limit orders
	tx := types.NewTransaction(0, toAddress, value, gasLimit, gasPrice, []byte("0x7b2264656164696e67223a22303130222c22616d6f756e74223a223030222c2274797065223a302c2273656c6c496e74223a7b22617373657473223a5b7b2274797065223a226c696d6974222c22616d6f756e74223a226269745472616465222c2276616c7565223a22353130227d2c7b2274797065223a2273697a65222c22616d6f756e74223a226269745472616465222c2276616c7565223a223634222c2261646472657373223a22306x30783330303030222c22657870697279223a223130303030303030303030303030227d5d2c22746f223a22576952656d6f74655f4275666665725f565831222c2276616c7565223a22313030303030303030303030303030303030227d"))
	txBytes := tx.Data()
	fmt.Println(txBytes)
	for i := 0; i < 101000; i++ {
		fmt.Println(i, "Uploading transactions:", txBytes)
		_, err = db.Exec(`INSERT INTO transactions (tx_bytes) VALUES ($1)`, txBytes)
		if err != nil {
			return err
		}
	}
	defer db.Close()
	return nil
}

func checkETCD() {
	cfg := clientv3.Config{
		Endpoints:   []string{os.Getenv("ETCD_LOCAL_URI")},
		DialTimeout: 5 * time.Second,
		Username:    os.Getenv("ETCD_USER"),
		Password:    os.Getenv("ETCD_PASSWORD"),
	}
	client, err := clientv3.New(cfg)
	if err != nil {
		panic(err)
	}
	conn, err := client.Dial(os.Getenv("ETCD_LOCAL_URI"))
	if err != nil {
		panic(err)
	}
	fmt.Println("etcd connection success")
	fmt.Println(spew.Sdump(conn))
}

func keyPair() KeyPair {
	privKey, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	if err != nil {
		panic(err)
	}
	pubKey := privKey.PublicKey
	privKeyBytes := privKey.D.Bytes()
	pubKeyBytes := elliptic.Marshal(elliptic.P224(), pubKey.X, pubKey.Y)
	privKeyHex := hex.EncodeToString(privKeyBytes)
	pubKeyHex := hex.EncodeToString(pubKeyBytes)
	return KeyPair{privKeyHex, pubKeyHex}
}

func createKeyPairs(keys int) []KeyPair {
	keyPairs := make([]KeyPair, 0)
	for i := 0; i < keys+1; i++ {
		fmt.Println("Created key pair")
		keyPairs = append(keyPairs, keyPair())
	}
	return keyPairs
}

func uploadKeyPairs(keyPairs []KeyPair) error {
	db, err := sql.Open("postgres", os.Getenv("POSTGRES_STRING_LOCAL"))
	if err != nil {
		panic(err)
	}
	defer db.Close()
	for _, keyPair := range keyPairs {
		fmt.Println("Uploading key pair", keyPair)
		_, err = db.Exec(`
        INSERT INTO key_pair (private_key, public_key)
        VALUES ($1, $2)`, []byte(keyPair.PrivateKey), []byte(keyPair.PublicKey))
		if err != nil {
			return err
		}
		fmt.Println("Uploaded key pair", keyPair.PrivateKey, keyPair.PublicKey)
	}
	return nil
}

func seedETCD(keyPairs []KeyPair) error {
	return pushPrivateKeys(keyPairs)
}

func pushPrivateKeys(keyPairs []KeyPair) error {
	cfg := clientv3.Config{
		Endpoints:   []string{os.Getenv("ETCD_LOCAL_URI")},
		DialTimeout: 5 * time.Second,
		Username:    os.Getenv("ETCD_USER"),
		Password:    os.Getenv("ETCD_PASSWORD"),
	}
	client, err := clientv3.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to push keys")
	}
	for i, pk := range keyPairs {
		timestamp := time.Now().Unix()
		timestampStr := strconv.FormatInt(timestamp, 10)
		fmt.Println("Pushing private key to etcd", pk)
		_, err := client.KV.Put(context.Background(), "/private_key_stack/"+timestampStr, keyPairs[i].PrivateKey)
		if err != nil {
			return err
		}
		fmt.Println("push key success")
	}
	return nil
}

func deleteTables() {
	db, err := sql.Open("postgres", os.Getenv("POSTGRES_STRING_LOCAL"))
	if err != nil {
		panic(err)
	}
	_, err = db.Exec("DROP TABLE IF EXISTS key_pair")
	if err != nil {
		panic(err)
	}
	_, err = db.Exec("DROP TABLE IF EXISTS transactions")
	if err != nil {
		panic(err)
	}
	return
}

func seedTables() error {
	db, err := sql.Open("postgres", os.Getenv("POSTGRES_STRING_LOCAL"))
	if err != nil {
		return err
	}
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS key_pair (
            id SERIAL PRIMARY KEY,
            private_key BYTEA NOT NULL,
            public_key BYTEA NOT NULL,
			signature BYTEA
        )
    `)
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS transactions (
			id SERIAL PRIMARY KEY,
			tx_bytes BYTEA NOT NULL
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
	var exists bool
	err = db.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)", "key_pair").Scan(&exists)
	if err != nil {
		log.Fatal(err)
	}
	err = db.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)", "transactions").Scan(&exists)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("KP table and transactions table created with no errors")
	return nil
}

func ignite() error {
	worker := []string{"signing-worker-1.demo.svc.cluster.local", "signing-worker-2.demo.svc.cluster.local", "signing-worker-3.demo.svc.cluster.local"}
	for _, uri := range worker {
		resp, err := http.Post(uri, "application/json", nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		fmt.Printf("Response from %s: %s\n", uri, resp.Status)
	}
	return nil
}

type KeyPair struct {
	PrivateKey string
	PublicKey  string
}
