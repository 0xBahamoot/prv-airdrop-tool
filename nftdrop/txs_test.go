package main

import (
	"fmt"
	"github.com/incognitochain/go-incognito-sdk-v2/common"
	"github.com/incognitochain/go-incognito-sdk-v2/incclient"
	"github.com/incognitochain/go-incognito-sdk-v2/wallet"
	"log"
	"strings"
	"sync"
	"testing"
	"time"
)

func init() {
	logger.Println("This runs before tests!!")
	var err error
	incClient, err = incclient.NewTestNetClientWithCache()
	if err != nil {
		log.Fatal(err)
	}
	minPRVRequired = incClient.GetMinPRVRequiredToMintNFT(0)

	privateKeys := []string {
		"111111168BGnyRzuMt6B9JpkhUpydQx8wDGhjm6GFjmHd7haKCp4eRiiwdSix3EfW5uobZUchW5FBQTBwVuSiBx2KZPVkBs1k6FRJm4qMga",
		"11111114LRaA4LN3QfEGLGL8CQD2WNCvTLmqjsovsoQkEQ6i2Dyi4CXrpcKQWU4hkaDFXUgNKqMyWEgYo8wXVWUKMYSxLpD7zTdW462AYf7",
		"1111111GWrmYNckA3JbW7WEh6yo3nq1SShoTSi2MDiV3Qs43A8pC5PURxkiwDbJwikY9CxEYnM7X9U7KSb2VxcVaF21mxFY2F7Zhm9FBAke",
		"11111116HF5hmznYqmdHSxpSC1MJBbMsTRy688mscLyPmxFLUFa7P6ASvnqF1iPBBJfx63cF24i8KvUECn8i8amKskB5pUTjmiSriRLrdFi",
		"11111119ufCKq6CqCqVXVHxC7dqVQqUKWF3G42hRfTFxse9h4w6QZ2YaqosWHpQ3vXyGSbdRCyqSzTswgETpvVwcjfTq9ZkaY3Kk4NaWsks",
		"1111111GLL522QcsAPYAw1ovLZBgnNqzjWnJQgXHktfmXvr3fFkyUSWeaNiZJGjtmWsmA5wUXCavwry2de3T9wrH4EJiEKKwJFr2AVLEji4",
		"11111115gwXxKgGRgXxsrTH5o7r5ATmqbRQXe5jBVHsaesFhQMZmjaCjNpDaGr32V83N8wfbU2V4tb4qDRGygWevMfp1Y2gnPh3ua4Eq1cH",
		"1111111DKq3k7CzF37E99hLmvakaSvbaZYH1XnbzsbfbM1megdvDKeNcyGTuiMHGFaoGYnZhno3c8EhwgJsbiUo33XKKUC3RUB1Qgg5H1Ss",
		"1111111F75R4jxgtUuqX9HumJaJaSrj1WmTHq5iYYec52HB1qP8HzQioBQEQJbUquAQEtpD1TNkqnf73pCXaUC1iPW8ipayYqYEPuqSFCaF",
		"11111119cMyaFU4hMycMvbX6h6JkXGezCEVZhsXpXo4ywDmYJ5EmXA9tXWMYGgqHMaorYjLdVDMKo51fHZvMWhDwENRQwgSeuuoFUXuU2yK",
		"1111111CvKBKh8pVFoZzcyFRF9nUQFv8zsjUYcC9ohRKpt9igjNvqbfdWQPfe4RmyrewzgpWp66Wn7Fx1iShLwfe2jagBW49V7cyEz2VmVW",
		"1111111AnmqpbpbR8oayrFCS2jyw8yP3Hy55MbaGXLHqKoChz5x3vq6oPPeBdaih2we6nyKnM4VK9Qg3NG49o1T9WME5zNAEAxvskBoCZpW",
		"11111113oqVMgLjCB2HsNbUo4MF6AphuzkZsRTDtMZA5ssfSDTyrJK2otFFts59Bw2oNWnM1vRDQqrTH2eFB9He3vndP9Z6ghnoWg3QuFVc",
		"11111117gjRrqFcfYckKWLHgQCfV1sZpKrrHPsgJpB2DtmFKcpVJqD69w6R2erzujtM4GviBkd6kJZrXVGxwAQ7Swz4q6LTtgsQ7h8CeFxV",
		"11111116Yq5wGy2d4H4ApNW68yfkx6rkfyED1apU7nu9ubWL1YztfiRPudffFzJGrF2HrGbdX45fxMNpL76GXqok1aQwSKJzGHMKSjSLpyH",
		"11111113kSwHCw5jGDsLTXXuELEeHYMVrzWnxE7F9siXipRwULA6us4Bg7zZQqprhNe2L42M1uC5wF5RQYDryjom9SrenXGtPTxd5dehkPz",
		"1111111AFLy2AdjkCUYLeJccBe5a2ZVt6yT5P8TcEvEH4EzSwT2zMBL9H2XH1Hku2FfHx6Vwzfc2EVL8yHTxfnh14z4dwuqs1iRTTJ1xJtY",
		"111111191ks9RRFCamFsob9KufmjhJhTPB2f6CVaarjSAoEsBjwvbL55JmDeAMyv6gAJkyjjRciEC9ZEjiiaGxUeeqKDFenp6g14cRyzkM6",
		"1111111Cnknjo1VTqdrZR7iEjPU6mFfM2LXx3dpjXV6dLofZMuNpYhHmkDzeZJofSLGZrFCWNHXaKjE4EJQAnFZX5jxNXcqsiaPV37NucxY",
		"11111116dxhHynySEVags1WNbNMvGr4qUH8jvdWwocur476YWsUnZg6hbU7cBKMtgLgbEN6E9Js4Hj8RhpUMX9J3F5CNjdK5P1b41mGprdN",
		"11111113xSLe31FVGLBP4pgeLihGc8j9zZgR2CSE2kBqDtRkvajTc5CyBNfpugHeVW9hLwTUVqit5Vop9aW8TBUwMLgnRsydBnSncLhcgVe",
		"1111111FZvZb8EC6UARYdzkC9i8veeNvP6T25owuZujikvc9hYTh5KohwY8H9KjrKnXzehj5TBw8RwQN3nmqXo1hhHZ1mDv8sfCVLRfdRN4",
		"1111111utxhEzcpTVXgZUWmo6Xp3DqaBC6KvpJw9rvEievDnfu6T1wxoHGJrr6AQEZF7zL5jRv6vpbQ5uMVo3ZFzuQRYP3EikyUscZAoKC",
		"11111115JfJU4fQPio37na6B1HNeqno1VoiWdgXW2P8Z55YX6dm7brzj5Z9u6StRPKerYWSfJwgHctgtG2c5SWumE9CkR6mTMvTFzzMsWDT",
	}

	config.Coinservice = "http://api-coinservice-staging2.incognito.org"

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
	logger.Println("Readyyyy, goooooooooooooo!!!")

	go adc.AirdropAccounts.manageNFTs()
	go adc.AirdropAccounts.managePRVUTXOs()
	logger.Println("Loaded config successfully!!")
}

func TestAirDrop(t *testing.T) {
	time.Sleep(10000 * time.Second)
	select {

	}
}

func TestTransferFunds(t *testing.T) {
	masterKey := "112t8rneWAhErTC8YUFTnfcKHvB1x6uAVdehy1S8GP2psgqDxK3RHouUcd69fz88oAL9XuMyQ8mBY5FmmGJdcyrpwXjWBXRpoWwgJXjsxi4j"
	addrList := make([]string, 0)
	amountList := make([]uint64, 0)
	for shard := 0; shard < common.MaxShardNumber; shard++ {
		for i := 0; i < 3; i++ {
			w, err := wallet.GenRandomWalletForShardID(byte(shard))
			if err != nil {
				panic(err)
			}
			logger.Printf("shard %v, %v\n", shard, w.Base58CheckSerialize(wallet.PrivateKeyType))
			addrList = append(addrList, w.Base58CheckSerialize(wallet.PaymentAddressType))
			amountList = append(amountList, uint64(10000000000))
		}
	}

	txHash, err := incClient.CreateAndSendRawTransaction(masterKey, addrList, amountList, 2, nil)
	if err != nil {
		panic(err)
	}
	logger.Println(txHash)
}

func TestTransferNFT(t *testing.T) {
	defaultReceiver := "1111111U1tofCB5sj3oKYgHbr6PXGtub7WTdKN2KcUdACTBN9GH5RYoAAYmeTgF6F6cfZ6HvYjSMiWWhfkLeGXD4Kw5auCFUqnaGrso7Eg"
	defaultReceiverAddr := incclient.PrivateKeyToPaymentAddress(defaultReceiver, -1)

	numTransferred := 100
	//myNFTs, err := incClient.GetMyNFTs(defaultReceiver)
	//if err != nil {
	//	logger.Println(err)
	//}
	//logger.Printf("old numNFTs: %v\n", len(myNFTs))

	doneCount := 0
	mtx := new(sync.Mutex)
	start := time.Now()
	for i := 0; i < numTransferred; i++ {
		go func(i int) {
			receiver := defaultReceiverAddr
			shardID := byte(common.RandInt() % common.MaxShardNumber)
			for {
				if shardID == 2 {
					shardID = byte(common.RandInt() % common.MaxShardNumber)
					continue
				}
				break
			}

			w, _ := wallet.GenRandomWalletForShardID(shardID)
			if w != nil {
				receiver = w.Base58CheckSerialize(wallet.PaymentAddressType)
			}
			var txHash, nft string
			attempt := 0
			for attempt < maxAttempts {
				acc, err := adc.AirdropAccounts.GetRandomAirdropAccount(shardID)
				if err != nil {
					logger.Printf("%v: attempt: %v, GetRandomAirdropAccount error: %v\n", i, attempt, err)
					time.Sleep(10 * time.Second)
					attempt++
					continue
				}
				txHash, nft, err = transferNFT(acc, receiver)
				if err != nil {
					if !strings.Contains(err.Error(), "reject") {
						logger.Printf("i: %v, attempt: %v, transferNFT %v error: %v\n", i, attempt, acc.toString(), err)
					}

					time.Sleep(10 * time.Second)
					attempt++
					continue
				}
				mtx.Lock()
				doneCount++
				mtx.Unlock()
				logger.Printf("Done i (%v, %v, %v), doneCount %v, acc %v, TxHash %v, nftID %v\n", i, attempt, shardID, doneCount, acc.toString(), txHash, nft)
				break
			}
			if attempt >=maxAttempts {
				panic(fmt.Sprintf("%v FAILED!!!", i))
			}
		}(i)
		sleep := 1 + common.RandInt() % 2000
		time.Sleep(time.Duration(sleep) * time.Millisecond)
	}

	//time.Sleep(100 * time.Second)
	//myNFTs, err = incClient.GetMyNFTs(defaultReceiver)
	//if err != nil {
	//	logger.Println(err)
	//}
	//logger.Printf("new numNFTs: %v\n", len(myNFTs))
	//if len(myNFTs) < numTransferred {
	//	panic(fmt.Sprintf("expected at least %v NFTs, got %v", numTransferred, len(myNFTs)))
	//}
	logger.Printf("timeElapsed: %v\n", time.Since(start).Seconds())
	select {}
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
	logger.Printf("old numNFTs: %v\n", len(myNFTs))

	mintNFTMany(acc, numRequired - len(myNFTs))
	time.Sleep(100 * time.Second)

	myNFTs, err = acc.GetMyNFTs()
	if err != nil {
		panic(err)
	}
	logger.Printf("new numNFTs: %v\n", len(myNFTs))
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
	logger.Printf("old numUTXOs: %v\n", len(utxoList))

	err = splitPRV(acc, 100, numRequired - len(utxoList))
	if err != nil {
		panic(err)
	}

	utxoList, err = acc.GetUTXOsByAmount(common.PRVIDStr, 100)
	if err != nil {
		panic(err)
	}
	logger.Printf("new numUTXOs: %v\n", len(utxoList))
	if len(utxoList) < numRequired {
		panic(fmt.Sprintf("expected at least %v UTXOs, got %v", numRequired, len(utxoList)))
	}
}
