package verifytxworker

import (
	"crypto/ecdsa"
	"database/sql"
	"encoding/hex"

	"github.com/ethereum/go-ethereum/crypto"
	_ "github.com/lib/pq"
)

type keyPair struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
}

func loadKeyPairsFromDB() ([]keyPair, error) {
	connStr := "user=your_user password=your_password dbname=your_db sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT private_key, public_key FROM keyPairs")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keyPairs := []keyPair{}

	for rows.Next() {
		var privateKeyHex, publicKeyHex string
		if err := rows.Scan(&privateKeyHex, &publicKeyHex); err != nil {
			return nil, err
		}

		privateKeyBytes, err := hex.DecodeString(privateKeyHex)
		if err != nil {
			return nil, err
		}
		publicKeyBytes, err := hex.DecodeString(publicKeyHex)
		if err != nil {
			return nil, err
		}

		privateKey, err := crypto.ToECDSA(privateKeyBytes)
		if err != nil {
			return nil, err
		}
		publicKey, err := crypto.UnmarshalPubkey(publicKeyBytes)
		if err != nil {
			return nil, err
		}

		keyPairs = append(keyPairs, keyPair{PrivateKey: privateKey, PublicKey: publicKey})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return keyPairs, nil
}
