package main

import (
	"context"
	"fmt"
	"github.com/incognitochain/go-incognito-sdk-v2/incclient"
	"log"
	"sync"
	"time"
)

var incClient *incclient.IncClient

type UserAccount struct {
	PaymentAddress     string
	Pubkey             string
	ShardID            int
	TotalTokens        map[string]uint64
	OngoingTxs         []string
	Txs                map[string]*AirdropTxDetail
	LastAirdropRequest time.Time
	AirdropSuccess     bool
}

type AirdropTxDetail struct {
	TxHash  string
	NFTused string
	Status  int
}

type AirdropController struct {
	userlock        sync.RWMutex
	airlock         sync.RWMutex
	UserAccounts    map[string]*UserAccount
	AirdropAccounts *AccountManager
	lastUsedADA     int
}

var adc AirdropController

var (
	defaultSleepTime = 120 // seconds
	maxAttempts      = 100
	numMintBatchNFTs = 200
	numSplitPRVs     = 200
	minPRVRequired   = uint64(0)
)

func waitingCheckTxInBlock(acc *AccountInfo, txHash, tokenIDStr string, utxoList []Coin) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	success := false
	for {
		select {
		case <-ctx.Done():
			// We assume timed-out = failed
			logger.Printf("Checking status of tx %v timed-out\n", txHash)
			acc.ClearTempUsed(tokenIDStr, utxoList)
			return
		default:
			isInBlock, err := incClient.CheckTxInBlock(txHash)
			if err != nil || !isInBlock {
				time.Sleep(10 * time.Second)
			} else {
				//logger.Printf("Tx %v is in block\n", txHash)
				success = true
				acc.MarkUsed(tokenIDStr, utxoList)
				time.Sleep(10 * time.Second)
			}
		}
		if success {
			break
		}
	}
}

func watchUserAirdropStatus(user *UserAccount, ctx context.Context) {
	defer func() {
		err := UpdateUserAirdropInfo(user)
		if err != nil {
			logger.Println(err)
		}
		if !user.AirdropSuccess {
			AirdropNFT(user)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			user.OngoingTxs = []string{}
			user.AirdropSuccess = false
			return
		default:
			txToWatchLeft := []string{}
			for _, txhash := range user.OngoingTxs {
				isInBlock, err := incClient.CheckTxInBlock(txhash)
				if err != nil {
					fmt.Println(err)
					continue
				}
				if !isInBlock {
					txToWatchLeft = append(txToWatchLeft, txhash)
				} else {
					user.Txs[txhash].Status = 2
				}
				fmt.Println("txDetail.IsInBlock", isInBlock)
			}
			user.OngoingTxs = txToWatchLeft
			err := UpdateUserAirdropInfo(user)
			if err != nil {
				logger.Println(err)
			}
			if len(user.OngoingTxs) == 0 {
				user.AirdropSuccess = true
				logger.Println("Done airdrop for user", user.PaymentAddress)
				return
			}
			time.Sleep(15 * time.Second)
		}
	}
}
