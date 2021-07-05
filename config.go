package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

type Config struct {
	Port        int
	Coinservice string
	Fullnode    string
	AirdropKeys []string
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
		acc := &AirdropAccount{
			Privatekey: key,
			UTXOInUse:  make(map[string]struct{}),
		}
		adc.AirdropAccounts = append(adc.AirdropAccounts, acc)
	}
	adc.airlock.Unlock()
}
