package main

import (
	"fmt"
	"github.com/incognitochain/go-incognito-sdk-v2/common"
	"github.com/incognitochain/go-incognito-sdk-v2/incclient"
	"log"
	"strings"
	"sync"
	"testing"
	"time"
)

func init() {
	log.Println("This runs before tests!!")
	var err error
	incClient, err = incclient.NewTestNet1ClientWithCache()
	if err != nil {
		log.Fatal(err)
	}
	minPRVRequired = incClient.GetMinPRVRequiredToMintNFT(0)

	privateKeys := []string {
		//"112t8rneWAhErTC8YUFTnfcKHvB1x6uAVdehy1S8GP2psgqDxK3RHouUcd69fz88oAL9XuMyQ8mBY5FmmGJdcyrpwXjWBXRpoWwgJXjsxi4j",
		//"11111117yu4WAe9fiqmRR4GTxocW6VUKD4dB58wHFjbcQXeDSWQMNyND6Ms3x136EfGcfL7rk3L83BZBzUJLSczmmNi1ngra1hUSSAPf5Jo",
		//"11111117XpDPfSFgYnmCh5REx3jXSUXTHjyn8ijeTYLY7ZmP44sFvmM8vAJwmp8CMuW8X77i3eotRHch3RYS4GzUn7cZYa21zCx5dWbG1d6",
		//"1111111EvAeDS6QDJpCFJW36rADQAV6RbsrPpwFxXSmXgzk6sjb7u6JWC7FRv7NvT4EMu7oLCqox2vPDrjqL4YBS4CW6fH8mZ7oEM8VWK1Z",
		//"111111137xURMTSVjxKRbVrBCVc1sUF1MQq6wXjw2V96A6pqMKvg2hy9EdrL3PcPmVosPRnPmNqJnKNRnJWVVZpx9KvTc1shgKoyMkQz1KA",
		//"1111111HFTFd36Bxok5TjBNoyyvFnT1PQAzVa6wUawvr3g1cseQtSesFKrd7BKM5udiqyjKMfU4GQKNF1e2nq7j8yguwcUGAVrKHBSBDK6k",
		//"1111111GPbi5M6isyuBQdHHr2dZzsjbxpv1SyTNnvjAWBSMdB1DbtjiBtG9uaNT6mCJVqYQdexnZwu4txz6TSCNwUs6YyboWfyt5vaPkCsi",
		"1111111AW19LkUVTDuYQXwrMhfJKAM1vHKAiNVyFLdsGvZZyzC7wbgvbHBnLZ9WwaeYhFu63NukbjYwifiZS5n9nSauvxzXiMBtn6HwdTAz",
		//"11111118kg49FkMUVxKssLMEUYREpTqWNct2zdHQmiNCsQ5BCuLABC6y3YD73fxNpTF1HGrFtuocKRVBbqttsjAVgfNVWNiL9nGUrTezuzL",
		//"1111111FXwFLFNbRxUnBgxpy9mgnGPK7W9rPuHp6S93JGUS1E5W3zRcPTP959X9zDVtriaDYPNbCp9VYP7zQwjfiuYVACZBg8UnWLfLFoZK",
		//"1111111GykScyt1hzDTrbi4iWWkZP5gcFvpsh37f16oKhGfqiub5Jr2269vWjeZhAtBXKehdTTrgR4DMw35sGs48DEKJiksznBPBFM6ZGHF",
		//"11111114jRSTkBjyJ6jBLTcL6sAZQ5hymQcpv4ojKuyzpsPMWAQqGAjN4oNqPnun9HGQ2xhHufCXjnhNEY19rnBHAZnFyBcbgdVeydLD16S",
		//"1111111ActgJou25GHjmruaz474A2x6fd3eZSPK29QiGXzc3c579RZPvrchE2Yt6G9ApK8NYGKPXK5qV4M1R2L9xEaw9TBnjJXgp1DoHriP",
	}

	log.Println("Loading accounts...")
	adc.AirdropAccounts, err = NewAccountManager(privateKeys)
	if err != nil {
		panic(err)
	}
	log.Printf("Loaded accounts: %v\n", len(adc.AirdropAccounts.Accounts))

	//for {
	//	ready := true
	//	for _, acc := range adc.AirdropAccounts.Accounts {
	//		if !acc.available {
	//			log.Printf("Account %v not ready!!\n", acc.PublicKey)
	//			ready = false
	//			break
	//		}
	//	}
	//	if !ready {
	//		time.Sleep(20 * time.Second)
	//	} else {
	//		log.Printf("All accounts are ready!!!\n")
	//		break
	//	}
	//}
	go adc.AirdropAccounts.Sync()
	time.Sleep(10 * time.Second)
	go adc.AirdropAccounts.manageNFTs()
	go adc.AirdropAccounts.managePRVUTXOs()
	log.Println("Loaded config successfully!!")
}

func TestAirDrop(t *testing.T) {
	select {

	}
}

func TestTransferNFT(t *testing.T) {
	receiver := "1111111U1tofCB5sj3oKYgHbr6PXGtub7WTdKN2KcUdACTBN9GH5RYoAAYmeTgF6F6cfZ6HvYjSMiWWhfkLeGXD4Kw5auCFUqnaGrso7Eg"
	receiverAddr := incclient.PrivateKeyToPaymentAddress(receiver, -1)

	numTransferred := 100
	myNFTs, err := incClient.GetMyNFTs(receiver)
	if err != nil {
		log.Println(err)
	}
	log.Printf("old numNFTs: %v\n", len(myNFTs))

	//time.Sleep(2 * time.Minute)
	doneCount := 0
	mtx := new(sync.Mutex)
	for i := 0; i < numTransferred; i++ {
		go func(i int) {
			var txHash, nft string
			attempt := 0
			for attempt < maxAttempts {
				acc, err := adc.AirdropAccounts.GetRandomAccount(255)
				if err != nil {
					log.Printf("%v: attempt: %v, GetRandomAccount error: %v\n", i, attempt, err)
					time.Sleep(10 * time.Second)
					attempt++
					continue
				}
				txHash, nft, err = transferNFT(acc, receiverAddr)
				if err != nil {
					if !strings.Contains(err.Error(), "reject") {
						log.Printf("i: %v, attempt: %v, transferNFT %v error: %v\n", i, attempt, acc.toString(), err)
					}

					time.Sleep(10 * time.Second)
					attempt++
					continue
				}
				mtx.Lock()
				doneCount++
				mtx.Unlock()
				log.Printf("Done i %v, doneCount %v, acc %v, TxHash %v, nftID %v\n", i, doneCount, acc.toString(), txHash, nft)
				break
			}
		}(i)
		sleep := 1 + common.RandInt() % 100
		time.Sleep(time.Duration(sleep) * time.Millisecond)
	}

	time.Sleep(100 * time.Second)
	myNFTs, err = incClient.GetMyNFTs(receiver)
	if err != nil {
		log.Println(err)
	}
	log.Printf("new numNFTs: %v\n", len(myNFTs))
	if len(myNFTs) < numTransferred {
		panic(fmt.Sprintf("expected at least %v NFTs, got %v", numTransferred, len(myNFTs)))
	}
}

func TestMintNFTMany(t *testing.T) {
	var err error

	privateKey := "112t8rneWAhErTC8YUFTnfcKHvB1x6uAVdehy1S8GP2psgqDxK3RHouUcd69fz88oAL9XuMyQ8mBY5FmmGJdcyrpwXjWBXRpoWwgJXjsxi4j"
	acc, err := NewAccountFromPrivateKey(privateKey)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			time.Sleep(20 * time.Second)
			acc.Update()
		}
	}()

	numRequired := 800
	myNFTs, err := acc.GetMyNFTs()
	if err != nil {
		panic(err)
	}
	log.Printf("old numNFTs: %v\n", len(myNFTs))

	mintNFTMany(acc, numRequired - len(myNFTs))
	time.Sleep(100 * time.Second)

	myNFTs, err = acc.GetMyNFTs()
	if err != nil {
		panic(err)
	}
	log.Printf("new numNFTs: %v\n", len(myNFTs))
	if len(myNFTs) < numRequired {
		panic(fmt.Sprintf("expected at least %v NFTs, got %v", numRequired, len(myNFTs)))
	}
}

func TestSplitPRV(t *testing.T) {
	var err error
	incClient, err = incclient.NewTestNetClientWithCache()
	if err != nil {
		panic(err)
	}

	privateKey := "112t8rneWAhErTC8YUFTnfcKHvB1x6uAVdehy1S8GP2psgqDxK3RHouUcd69fz88oAL9XuMyQ8mBY5FmmGJdcyrpwXjWBXRpoWwgJXjsxi4j"
	acc, err := NewAccountFromPrivateKey(privateKey)
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			acc.Update()
			time.Sleep(20 * time.Second)
		}
	}()


	numRequired := 1000
	utxoList, err := acc.GetUTXOsByAmount(common.PRVIDStr, 100)
	if err != nil {
		panic(err)
	}
	log.Printf("old numUTXOs: %v\n", len(utxoList))

	err = splitPRV(acc, 100, numRequired - len(utxoList))
	if err != nil {
		panic(err)
	}

	utxoList, err = acc.GetUTXOsByAmount(common.PRVIDStr, 100)
	if err != nil {
		panic(err)
	}
	log.Printf("new numUTXOs: %v\n", len(utxoList))
	if len(utxoList) < numRequired {
		panic(fmt.Sprintf("expected at least %v UTXOs, got %v", numRequired, len(utxoList)))
	}
}
