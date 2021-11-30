package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/go-incognito-sdk-v2/common"
	"github.com/incognitochain/go-incognito-sdk-v2/common/base58"
	"github.com/incognitochain/go-incognito-sdk-v2/wallet"
	"github.com/pkg/errors"
)

func main() {
	adc.UserAccounts = make(map[string]*UserAccount)
	readConfig()
	if err := initDB(); err != nil {
		panic(err)
	}
	var err error

	logger.Println("initiating airdrop-tool")
	adc.lastUsedADA = 0
	airdroppedUser, err := LoadUserAirdropInfo()
	if err != nil {
		panic(err)
	}
	for _, v := range airdroppedUser {
		adc.UserAccounts[v.Pubkey] = v
		if len(v.OngoingTxs) != 0 {
			ctx, _ := context.WithTimeout(context.Background(), 20*time.Minute)
			go watchUserAirdropStatus(v, ctx)
		} else {
			if !v.AirdropSuccess {
				go AirdropNFT(v)
			}
		}
	}
	r := gin.Default()

	r.GET("/requestdrop-nft", APIReqDrop)

	r.Run("0.0.0.0:" + strconv.Itoa(config.Port))
	select {}
}

func APIReqDrop(c *gin.Context) {
	paymentkey := c.Query("paymentkey")
	if paymentkey == "" {
		c.JSON(http.StatusOK, gin.H{
			"Result": -1,
		})
		return
	}
	shardID := 0
	wl, err := wallet.Base58CheckDeserialize(paymentkey)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"Result": 0,
			"Error":  err,
		})
	}
	shardID = int(common.GetShardIDFromLastByte(wl.KeySet.PaymentAddress.Pk[31]))
	pubkey := base58.Base58Check{}.Encode(wl.KeySet.PaymentAddress.Pk, 0)

	adc.userlock.Lock()
	if user, ok := adc.UserAccounts[pubkey]; ok {
		adc.userlock.Unlock()
		t := user.LastAirdropRequest
		r := 0
		if time.Since(t) <= 30*time.Minute {
			r = 1
		}
		if user.AirdropSuccess {
			r = 2
		}
		c.JSON(http.StatusOK, gin.H{
			"Result":    r,
			"AirdropTx": user.Txs,
		})
		return
	}
	existNFT, err := checkUserHaveNFT(paymentkey)
	if err != nil {
		panic(err)
	}
	if existNFT {
		adc.userlock.Unlock()
		c.JSON(http.StatusOK, gin.H{
			"Result": -1,
		})
		return
	}
	newUserAccount := new(UserAccount)
	newUserAccount.PaymentAddress = paymentkey
	newUserAccount.Pubkey = pubkey
	newUserAccount.ShardID = shardID
	newUserAccount.Txs = make(map[string]*AirdropTxDetail)
	adc.UserAccounts[pubkey] = newUserAccount
	adc.userlock.Unlock()
	err = UpdateUserAirdropInfo(newUserAccount)
	if err != nil {
		logger.Println(err)
	}
	go AirdropNFT(newUserAccount)
	c.JSON(http.StatusOK, gin.H{
		"Result": 1,
	})
}

func checkUserHaveNFT(paymentAddress string) (bool, error) {
	resp, err := http.Get(config.Coinservice + "/getkeyinfo?key=" + paymentAddress)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	body, err := readRespBody(resp)
	if err != nil {
		return false, err
	}
	var apiResp struct {
		Result shared.KeyInfoData
		Error  string
	}

	err = json.Unmarshal(body, &apiResp)
	if err != nil {
		return false, err
	}
	if apiResp.Error != "" {
		return false, errors.New(apiResp.Error)
	}
	keyinfo := apiResp.Result
	if len(keyinfo.NFTIndex) == 0 {
		return false, nil
	}
	return true, nil
}

func AirdropNFT(user *UserAccount) {
	logger.Printf("New Airdrop request from %v, shard %v\n", user.Pubkey, user.ShardID)
	txsToWatch := make([]string, 0)
	attempt := 0
	for attempt < maxAttempts {
		adc.userlock.Lock()
		user.LastAirdropRequest = time.Now()
		adc.userlock.Unlock()

		airdropAccount, err := adc.AirdropAccounts.GetRandomAirdropAccount(byte(user.ShardID))
		if err != nil {
			logger.Printf("Choose airdrop account at attempt %v error: %v\n", attempt, err)
			attempt++
			time.Sleep(10 * time.Second)
			continue
		}
		txHash, nftID, err := transferNFT(airdropAccount, user.PaymentAddress)
		if err != nil {
			if !strings.Contains(err.Error(), "reject") && !strings.Contains(err.Error(), "Reject") {
				logger.Printf("transferNFT from %v to %v(%v) at attempt %v error: %v\n", airdropAccount.toString(), user.Pubkey, user.ShardID, attempt, err)
			}
			attempt++
			time.Sleep(10 * time.Second)
			continue
		}

		txsToWatch = append(txsToWatch, txHash)

		adc.userlock.Lock()
		user.LastAirdropRequest = time.Now()
		user.Txs[txHash] = &AirdropTxDetail{
			TxHash:  txHash,
			NFTused: nftID,
			Status:  1,
		}
		user.OngoingTxs = txsToWatch
		adc.userlock.Unlock()

		ctx, _ := context.WithTimeout(context.Background(), 5*time.Minute)
		watchUserAirdropStatus(user, ctx)
		break
	}

	if attempt >= maxAttempts {
		logger.Printf("Cannot transferNFT to account %v: max attempt exceeded\n", user.PaymentAddress)
	} else {
		logger.Printf("transferNFT to %v FINISHED\n", user.PaymentAddress)
	}
}
