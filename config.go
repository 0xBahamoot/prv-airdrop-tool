package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/incognitochain/go-incognito-sdk-v2/common"
	"github.com/incognitochain/go-incognito-sdk-v2/wallet"
)

type Config struct {
	Port          int
	Coinservice   string
	Fullnode      string
	AirdropKeys   []AirdropKey
	CaptchaSecret string
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
	if config.CaptchaSecret == "" {
		capSecret := os.Getenv("CAPTCHA_SECRET")
		config.CaptchaSecret = capSecret
	}

	adc.airlock.Lock()
	for idx, key := range config.AirdropKeys {
		wl, err := wallet.Base58CheckDeserialize(key.PrivateKey)
		if err != nil {
			panic(err)
		}

		acc := &AirdropAccount{
			PaymentAddress: wl.Base58CheckSerialize(wallet.PaymentAddressType),
			Privatekey:     key.PrivateKey,
			ShardID:        int(common.GetShardIDFromLastByte(wl.KeySet.PaymentAddress.Pk[31])),
			UTXOInUse:      make(map[string]struct{}),
		}
		fmt.Printf("idx %v -> shardid %v\n", idx, acc.ShardID)
		adc.AirdropAccounts = append(adc.AirdropAccounts, acc)
	}
	adc.airlock.Unlock()
}
