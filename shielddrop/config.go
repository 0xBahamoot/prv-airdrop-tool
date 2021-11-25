package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

type Config struct {
	Coinservice    string
	Airdropservice string
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
}
