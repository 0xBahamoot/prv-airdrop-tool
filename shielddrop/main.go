package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/incognitochain/coin-service/shared"
	"github.com/kamva/mgm/v3"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	readConfig()
	fromtime := time.Now().Unix()
	for {
		offset := int64(0)
	retry:
		list, err := getShieldWithRespond(uint64(fromtime), offset)
		if err != nil {
			panic(err)
		}
		for _, v := range list {
			if v.Locktime > fromtime && len(list) <= 1000 {
				fromtime = v.Locktime
			}
			for _, p := range v.PubKeyReceivers {
				err := requestAirdrop(p)
				if err != nil {
					log.Println(err)
				}
			}
		}
		if len(list) > 1000 {
			offset += int64(len(list))
			goto retry
		}
		time.Sleep(10 * time.Second)
	}
}

func ConnectDB(dbName string, mongoAddr string) error {
	err := mgm.SetDefaultConfig(nil, dbName, options.Client().ApplyURI(mongoAddr))
	if err != nil {
		return err
	}
	_, cd, _, _ := mgm.DefaultConfigs()
	err = cd.Ping(context.Background(), nil)
	if err != nil {
		return err
	}
	log.Println("Database Connected!")
	return nil
}

func getShieldWithRespond(fromtime uint64, offset int64) ([]shared.TxData, error) {
	resp, err := http.Get(config.Coinservice + "/shield/gettxshield?offset=" + fmt.Sprintf("%v", offset) + "&fromtime=" + fmt.Sprintf("%v", fromtime))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := readRespBody(resp)
	if err != nil {
		return nil, err
	}
	var apiResp struct {
		Result []shared.TxData
		Error  string
	}

	err = json.Unmarshal(body, &apiResp)
	if err != nil {
		return nil, err
	}
	if apiResp.Error != "" {
		return nil, errors.New(apiResp.Error)
	}
	return apiResp.Result, nil
}

func requestAirdrop(pubkey string) error {
	resp, err := http.Get(config.Airdropservice + "/requestdrop?pubkey=" + pubkey + "&shield=true")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := readRespBody(resp)
	if err != nil {
		return err
	}
	var apiResp struct {
		Result string
		Error  string
	}
	err = json.Unmarshal(body, &apiResp)
	if err != nil {
		return err
	}
	fmt.Println(apiResp)
	return nil
}
