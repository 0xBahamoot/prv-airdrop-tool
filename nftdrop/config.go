package main

import (
	"encoding/json"
	"fmt"
	"github.com/incognitochain/go-incognito-sdk-v2/common"
	"github.com/incognitochain/go-incognito-sdk-v2/incclient"
	"io/ioutil"
	"log"
	"os"
	"time"
)

type Config struct {
	NumMintBatchNFTs      int
	NumSplitPRVs          int
	ThresholdTriggerMint  int
	ThresholdTriggerSplit int
	MaxGetCoinThreads     int
	EnableSDKLog          bool
	SDKLog                string
	Port                  int
	Coinservice           string
	Fullnode              string
	AirdropKeys           []AirdropKey
}
type AirdropKey struct {
	PrivateKey string
}

var config Config

func readConfig() {
	logger.Printf("Loading config...\n")
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
	if config.NumMintBatchNFTs != 0 {
		numMintBatchNFTs = config.NumMintBatchNFTs
	}
	if config.NumSplitPRVs != 0 {
		numSplitPRVs = config.NumSplitPRVs
	}
	if config.ThresholdTriggerMint != 0 {
		thresholdTriggerMint = config.ThresholdTriggerMint
	}
	if config.ThresholdTriggerSplit != 0 {
		thresholdTriggerSplit = config.ThresholdTriggerSplit
	}
	if config.MaxGetCoinThreads != 0 {
		incclient.MaxGetCoinThreads = config.MaxGetCoinThreads
	} else {
		incclient.MaxGetCoinThreads = 2
	}
	if config.EnableSDKLog {
		incclient.Logger.IsEnable = config.EnableSDKLog
	}
	if config.SDKLog != "" {
		writer, err := os.OpenFile(config.SDKLog, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			fmt.Println("Error opening file:", err)
			os.Exit(1)
		}
		incclient.Logger.Log = log.New(writer, "", log.Ldate|log.Ltime)
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
	logger.Println("Loading accounts...")
	adc.AirdropAccounts, err = NewAccountManager(privateKeys)
	if err != nil {
		panic(err)
	}
	logger.Printf("Loaded accounts: %v\n", len(adc.AirdropAccounts.Accounts))

	go adc.AirdropAccounts.Sync()
	shardStatus := make(map[byte]bool)
	for {
		ready := true
		for _, acc := range adc.AirdropAccounts.Accounts {
			if acc.isAvailable() {
				shardStatus[acc.ShardID] = true
			}
		}
		for shard := 1; shard < common.MaxShardNumber; shard++ {
			if !shardStatus[byte(shard)] {
				ready = false
				logger.Printf("Shard %v not ready!!\n", shard)
			}
		}
		if !ready {
			time.Sleep(10 * time.Second)
		} else {
			break
		}
	}
	go adc.AirdropAccounts.manageNFTs()
	go adc.AirdropAccounts.managePRVUTXOs()
	logger.Println("Loaded config successfully!!")
}
