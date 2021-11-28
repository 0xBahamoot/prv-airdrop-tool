package main

import (
	"encoding/json"
	"github.com/incognitochain/go-incognito-sdk-v2/incclient"
	"io/ioutil"
	"log"
	"time"
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
	log.Printf("Loading config...\n")
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

	incClient, err = incclient.NewIncClientWithCache(config.Fullnode, "", 2)
	if err != nil {
		log.Fatal(err)
	}

	privateKeys := make([]string, 0)
	for _, key := range config.AirdropKeys {
		privateKeys = append(privateKeys, key.PrivateKey)
	}
	adc.AirdropAccounts, err = NewAccountManager(privateKeys)
	if err != nil {
		panic(err)
	}
	log.Println("Loading accounts...")
	adc.AirdropAccounts, err = NewAccountManager(privateKeys)
	if err != nil {
		panic(err)
	}
	log.Printf("Loaded accounts: %v\n", len(adc.AirdropAccounts.Accounts))

	for {
		ready := true
		for _, acc := range adc.AirdropAccounts.Accounts {
			if !acc.available {
				log.Printf("Account %v not ready!!\n", acc.PublicKey)
				ready = false
				break
			}
		}
		if !ready {
			time.Sleep(20 * time.Second)
		} else {
			log.Printf("All accounts are ready!!!\n")
			break
		}
	}
	go adc.AirdropAccounts.Sync()
	go adc.AirdropAccounts.manageNFTs()
	go adc.AirdropAccounts.managePRVUTXOs()
	log.Println("Loaded config successfully!!")
}
