package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/incognitochain/coin-service/shared"
	"github.com/incognitochain/go-incognito-sdk-v2/coin"
	"github.com/incognitochain/go-incognito-sdk-v2/common"
	"github.com/incognitochain/go-incognito-sdk-v2/common/base58"
	"github.com/incognitochain/go-incognito-sdk-v2/incclient"
	"github.com/incognitochain/go-incognito-sdk-v2/wallet"
	"github.com/pkg/errors"
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

type AirdropAccount struct {
	lock           sync.Mutex
	Privatekey     string
	PaymentAddress string
	TotalUTXO      int
	ShardID        int
	// UTXOList   []Coin
	UTXOInUse   map[string]struct{}
	NFTs        map[string]Coin
	NFTInUse    map[string]struct{}
	LastNFTMint time.Time
}

type Coin struct {
	Coin  coin.PlainCoin
	Index uint64
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
	AirdropAccounts []*AirdropAccount
	lastUsedADA     int
}

var adc AirdropController

func main() {
	adc.UserAccounts = make(map[string]*UserAccount)
	readConfig()
	if err := initDB(); err != nil {
		panic(err)
	}
	var err error
	incClient, err = incclient.NewIncClient(config.Fullnode, "", 2)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("initiating airdrop-tool")
	otaKeyList := []string{}
	for _, acc := range adc.AirdropAccounts {
		wl, err := wallet.Base58CheckDeserialize(acc.Privatekey)
		if err != nil {
			panic(err)
		}
		otaKeyList = append(otaKeyList, wl.Base58CheckSerialize(wallet.OTAKeyType))
	}
	for _, key := range otaKeyList {
		err := incClient.SubmitKey(key)
		if err != nil {
			panic(err)
		}
	}
	for _, acc := range adc.AirdropAccounts {
		go func(a *AirdropAccount) {
			getAirdropAccountUTXOs(a)
			fmt.Println(a.Privatekey, a.TotalUTXO)
			if a.TotalUTXO < 20 {
				for i := 0; i < 40; i++ {
					mintNFT(a)
					time.Sleep(20 * time.Second)
				}

			}
		}(acc)
	}
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
				AirdropNFT(v)
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
		log.Println(err)
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
	txsToWatch := []string{}
retry:
	user.LastAirdropRequest = time.Now()
	airdropAccount := chooseAirdropAccount(user.ShardID)
	airdropAccount.lock.Lock()

	txDetail, txBytes, txHash, err := CreateAirDropTx(airdropAccount, user.PaymentAddress)
	if err != nil {
		airdropAccount.lock.Unlock()
		log.Println(err)
		time.Sleep(10 * time.Second)
		goto retry
	}

	airdropAccount.lock.Unlock()
	txsToWatch = append(txsToWatch, txHash)
	user.Txs[txHash] = txDetail
	err = incClient.SendRawTokenTx(txBytes)
	if err != nil {
		strings.Contains(err.Error(), "Reject")
		log.Println("send tx error", err)
		user.Txs[txHash].Status = 3
		time.Sleep(20 * time.Second)
		goto retry
	} else {
		user.Txs[txHash].Status = 1
	}
	user.OngoingTxs = txsToWatch

	ctx, _ := context.WithTimeout(context.Background(), 5*time.Minute)
	watchUserAirdropStatus(user, ctx)
}

func watchUserAirdropStatus(user *UserAccount, ctx context.Context) {
	defer func() {
		err := UpdateUserAirdropInfo(user)
		if err != nil {
			log.Println(err)
		}
		if !user.AirdropSuccess {
			AirdropNFT(user)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			// for _, txHash := range user.OngoingTxs {
			// 	user.Txs[txHash].Status = 3
			// }
			user.OngoingTxs = []string{}
			user.AirdropSuccess = false
			return
		default:
			txToWatchLeft := []string{}
			for _, txhash := range user.OngoingTxs {
				txDetail, err := incClient.GetTxDetail(txhash)
				if err != nil {
					fmt.Println(err)
					continue
				}
				if !txDetail.IsInBlock {
					txToWatchLeft = append(txToWatchLeft, txhash)
				} else {
					user.Txs[txhash].Status = 2
					// user.CompletedTxs = append(user.CompletedTxs, txhash)
				}
				fmt.Println("txDetail.IsInBlock", txDetail.IsInBlock)
			}
			user.OngoingTxs = txToWatchLeft
			err := UpdateUserAirdropInfo(user)
			if err != nil {
				log.Println(err)
			}
			if len(user.OngoingTxs) == 0 {
				user.AirdropSuccess = true
				log.Println("Done airdrop for user", user.PaymentAddress)
				return
			}
			time.Sleep(15 * time.Second)
		}
	}
}

func CreateAirDropTx(ada *AirdropAccount, paymentAddress string) (*AirdropTxDetail, []byte, string, error) {
	log.Println("Creating tx with param", ada.Privatekey, paymentAddress)
	valueList := []uint64{}
	paymentList := []string{}
	nftToUse := ""
	// var nftCoin *Coin
	valueList = append(valueList, 1)
	paymentList = append(paymentList, paymentAddress)

	// coinsDataToUse := []coin.PlainCoin{}
	// coinsToUseIdx := []uint64{}

	for nftID, _ := range ada.NFTs {
		nftToUse = nftID
		// nftCoin = &coin
		break
	}
	// fmt.Println("CreateAirDropTx", nftToUse, nftCoin.Coin)

	ada.NFTInUse[nftToUse] = struct{}{}

	tokenParam := incclient.NewTxTokenParam(nftToUse, 1, paymentList, valueList, false, 0, nil)
	txParam := incclient.NewTxParam(ada.Privatekey, nil, nil, 0, tokenParam, nil, nil)
	encodedTx, txHash, err := incClient.CreateRawTokenTransactionVer2(txParam)
	if err != nil {
		return nil, nil, "", err
	}

	txDetail := AirdropTxDetail{
		TxHash:  txHash,
		NFTused: nftToUse,
	}
	return &txDetail, encodedTx, txHash, nil
}

func chooseAirdropAccount(shardID int) *AirdropAccount {
	var result *AirdropAccount
	var i int
	adc.airlock.Lock()
retry:
	if i+1 == len(adc.AirdropAccounts) {
		time.Sleep(20 * time.Second)
		log.Println("can't choose airdrop account")
		i = 0
	}
	i++
	adc.lastUsedADA = (adc.lastUsedADA + 1) % len(adc.AirdropAccounts)
	result = adc.AirdropAccounts[adc.lastUsedADA]
	if result.ShardID != shardID {
		fmt.Println("chooseAirdropAccount", result.ShardID, shardID)
		goto retry
	}
	if result.TotalUTXO == 0 {
		getAirdropAccountUTXOs(result)
	}
	if len(result.UTXOInUse) == result.TotalUTXO {
		getAirdropAccountUTXOs(result)
	}
	if result.TotalUTXO == 0 {
		if time.Since(result.LastNFTMint) >= 5*time.Minute {
			go mintNFTMany(result, 40)
		}
		goto retry
	}
	if result.TotalUTXO <= 20 {
		go mintNFTMany(result, 40)
	}
	adc.airlock.Unlock()
	return result
}

func mintNFTMany(ada *AirdropAccount, numb int) {
	for i := 0; i < numb; i++ {
		mintNFT(ada)
		time.Sleep(40 * time.Second)
	}
}

func getAirdropAccountUTXOs(adc *AirdropAccount) {
	UTXOInUsePendingList := make(map[string]struct{})
	var utxos []Coin
	NFTlist, err := getNFTList(adc.PaymentAddress)
	if err != nil {
		log.Println("getAirdropAccountUTXOs", err)
		return
	}
	adc.lock.Lock()
	for _, nftID := range NFTlist {
		uxto, indices, err := incClient.GetUnspentOutputCoins(adc.Privatekey, nftID, 0)
		if err != nil {
			panic(err)
		}
		if len(uxto) > 1 {
			panic("wow")
		}
		for idx, v := range uxto {
			if v.GetVersion() == 2 {
				c := Coin{
					Coin:  v,
					Index: indices[idx].Uint64(),
				}
				utxos = append(utxos, c)
				adc.NFTs[nftID] = c
			}
		}
	}
	for _, v := range utxos {
		if _, ok := adc.UTXOInUse[v.Coin.GetPublicKey().String()]; ok {
			UTXOInUsePendingList[v.Coin.GetPublicKey().String()] = struct{}{}
		}
	}
	adc.TotalUTXO = len(utxos)
	adc.UTXOInUse = UTXOInUsePendingList
	adc.lock.Unlock()
}

func getNFTList(paymentAddress string) ([]string, error) {
	resp, err := http.Get(config.Coinservice + "/getkeyinfo?key=" + paymentAddress + "&version=2")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := readRespBody(resp)
	if err != nil {
		return nil, err
	}
	var apiResp struct {
		Result shared.KeyInfoData
		Error  string
	}

	err = json.Unmarshal(body, &apiResp)
	if err != nil {
		return nil, err
	}
	if apiResp.Error != "" {
		return nil, errors.New(apiResp.Error)
	}
	keyinfo := apiResp.Result
	result := []string{}
	for token, _ := range keyinfo.NFTIndex {
		result = append(result, token)
	}

	return result, nil
}

func mintNFT(ada *AirdropAccount) {
	txBytes, txhash, err := incClient.CreatePdexv3MintNFT(ada.Privatekey)
	if err != nil {
		log.Println("mintNFT", err)
		return
	}
	err = incClient.SendRawTx(txBytes)
	if err != nil {
		log.Println("mintNFT", err)
		return
	}
	log.Println("mint nft for", ada.Privatekey, txhash)
	ada.LastNFTMint = time.Now()
}
