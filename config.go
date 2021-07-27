package main

import (
	"encoding/json"
	"io/ioutil"
	"log"

	"github.com/incognitochain/go-incognito-sdk-v2/common"
	"github.com/incognitochain/go-incognito-sdk-v2/wallet"
)

type Config struct {
	Port        int
	Coinservice string
	Fullnode    string
	AirdropKeys []AirdropKey
}
type AirdropKey struct {
	PrivateKey string
}

var config Config

func readConfig() {
	data, err := ioutil.ReadFile("./cfg.json")
	if err != nil {
		log.Fatalln(err)
	}
	if data != nil {
		err = json.Unmarshal(data, &config)
		if err != nil {
			panic(err)
		}
	}

	adc.airlock.Lock()
	for _, key := range config.AirdropKeys {
		wl, err := wallet.Base58CheckDeserialize(key.PrivateKey)
		if err != nil {
			panic(err)
		}

		acc := &AirdropAccount{
			Privatekey: key.PrivateKey,
			ShardID:    int(common.GetShardIDFromLastByte(wl.KeySet.PaymentAddress.Pk[31])),
			UTXOInUse:  make(map[string]struct{}),
		}
		adc.AirdropAccounts = append(adc.AirdropAccounts, acc)
	}
	adc.airlock.Unlock()
}
