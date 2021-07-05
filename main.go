package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LampardNguyen234/go-incognito/coin"
	"github.com/LampardNguyen234/go-incognito/common"
	"github.com/LampardNguyen234/go-incognito/incclient"
	"github.com/LampardNguyen234/go-incognito/wallet"
	"github.com/gin-gonic/gin"
	"github.com/incognitochain/coin-service/shared"
	"github.com/pkg/errors"
)

var incClient *incclient.IncClient

type UserAccount struct {
	PaymentAddress     string
	TotalTokens        map[string]uint64
	OngoingTxs         []string
	Txs                map[string]*AirdropTxDetail
	LastAirdropRequest int64
	AirdropSuccess     bool
}

type AirdropAccount struct {
	lock       sync.Mutex
	Privatekey string
	TotalUTXO  int
	UTXOList   []Coin
	UTXOInUse  map[string]struct{}
}

type Coin struct {
	Coin  coin.PlainCoin
	Index uint64
}

type AirdropTxDetail struct {
	TxHash string
	Value  uint64
	Amount uint64
	Status int
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
	adc.lastUsedADA = 0
	airdroppedUser, err := LoadUserAirdropInfo()
	if err != nil {
		panic(err)
	}
	for _, v := range airdroppedUser {
		adc.UserAccounts[v.PaymentAddress] = v
		if len(v.OngoingTxs) != 0 {
			ctx, _ := context.WithTimeout(context.Background(), 35*time.Minute)
			go watchUserAirdropStatus(v, ctx)
		}
	}
	r := gin.Default()

	r.GET("/requestdrop", APIReqDrop)

	r.Run("0.0.0.0:" + strconv.Itoa(config.Port))
	select {}
}

func APIReqDrop(c *gin.Context) {
	paymentkey := c.Query("paymentkey")
	if paymentkey == "" {
		c.JSON(http.StatusOK, gin.H{
			"result": 0,
		})
		return
	}
	adc.userlock.RLock()
	if user, ok := adc.UserAccounts[paymentkey]; ok {
		adc.userlock.RUnlock()
		_ = user
		// t := time.Unix(user.LastAirdropRequest, 0)
		// if time.Since(t) < 30*time.Minute {
		// 	c.JSON(http.StatusOK, gin.H{
		// 		"result": 0,
		// 	})
		// 	return
		// }
		// go AirdropUser(user)
		c.JSON(http.StatusOK, gin.H{
			"result": 0,
		})
		return
	}
	adc.userlock.RUnlock()
	newUserAccount := new(UserAccount)
	newUserAccount.PaymentAddress = paymentkey
	newUserAccount.Txs = make(map[string]*AirdropTxDetail)
	adc.userlock.Lock()
	adc.UserAccounts[paymentkey] = newUserAccount
	adc.userlock.Unlock()
	go AirdropUser(newUserAccount)
	c.JSON(http.StatusOK, gin.H{
		"result": 1,
	})
}

func GetTokenAmounts(paymentAddress string) (map[string]uint64, error) {
	resp, err := http.Get(config.Coinservice + "/getkeyinfo?key=" + paymentAddress)
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
	result := make(map[string]uint64)
	for token, info := range keyinfo.CoinIndex {
		if token == common.PRVCoinID.String() {
			continue
		}
		result[token] = info.Total
	}
	return result, nil
}

func AirdropUser(user *UserAccount) {
	total, err := GetTokenAmounts(user.PaymentAddress)
	if err != nil {
		panic(err)
	}
	user.TotalTokens = total
	log.Println("total tokens", total)
	txsToWatch := []string{}
	totalPRVAmountNeeded := uint64(0)
	totalPRVCoinsNeeded := 0
	// for _, tokenAmount := range user.TotalTokens {
	// 	txNeedToSend := int(math.Ceil(float64(tokenAmount) / float64(PRVCoinPerTokenCoins)))
	// 	totalPRVAmountNeeded += (incclient.DefaultPRVFee + (AirdropCoinValue * PRVCoinPerTokenCoins)) * uint64(txNeedToSend)
	// 	totalPRVCoinsNeeded += txNeedToSend
	// }

	totalPRVAmountNeeded = uint64(len(user.TotalTokens)) * 2 * AirdropCoinValue
	totalPRVCoinsNeeded = len(user.TotalTokens) * 2

	airdropAccount := chooseAirdropAccount(totalPRVAmountNeeded)
	airdropAccount.lock.Lock()
	totalTxNeeded := int(math.Ceil(float64(totalPRVCoinsNeeded) / float64(MaxTxOutput)))

	log.Printf("sending txs for user %v: %v %v %v", user.PaymentAddress, totalPRVAmountNeeded, totalPRVCoinsNeeded, totalTxNeeded)
	txsToSend := [][]byte{}
	for i := 0; i < totalTxNeeded; i++ {
		if i+1 == totalTxNeeded {
			txDetail, txBytes, txHash, err := CreateAirDropTx(airdropAccount, user.PaymentAddress, uint64(totalPRVCoinsNeeded-(i*MaxTxOutput)))
			if err != nil {
				panic(err)
			}
			txsToWatch = append(txsToWatch, txHash)
			txsToSend = append(txsToSend, txBytes)
			user.Txs[txHash] = txDetail
		} else {
			txDetail, txBytes, txHash, err := CreateAirDropTx(airdropAccount, user.PaymentAddress, MaxTxOutput)
			if err != nil {
				panic(err)
			}
			txsToWatch = append(txsToWatch, txHash)
			txsToSend = append(txsToSend, txBytes)
			user.Txs[txHash] = txDetail
		}
	}
	airdropAccount.lock.Unlock()
	user.LastAirdropRequest = time.Now().Unix()
	sc := 0
	fl := 0
	for idx, txBytes := range txsToSend {
		err = incClient.SendRawTx(txBytes)
		if err != nil {
			strings.Contains(err.Error(), "Reject")
			log.Println("send tx error", err)
			user.Txs[txsToWatch[idx]].Status = 3
			fl++
		} else {
			user.Txs[txsToWatch[idx]].Status = 1
			sc++
		}
	}
	log.Printf("%v txs success, %v txs failed, wait for result...\n", sc, fl)
	user.OngoingTxs = txsToWatch

	ctx, _ := context.WithTimeout(context.Background(), 45*time.Minute)
	watchUserAirdropStatus(user, ctx)
}

func watchUserAirdropStatus(user *UserAccount, ctx context.Context) {
	defer func() {
		err := UpdateUserAirdropInfo(user)
		if err != nil {
			log.Println(err)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			for _, txHash := range user.OngoingTxs {
				user.Txs[txHash].Status = 3
			}
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

func CreateAirDropTx(ada *AirdropAccount, paymentAddress string, UTXOamount uint64) (*AirdropTxDetail, []byte, string, error) {
	log.Println("Creating tx with param", ada.Privatekey, paymentAddress, UTXOamount)
	totalPRVNeeded := (UTXOamount * AirdropCoinValue) + incclient.DefaultPRVFee
	valueList := []uint64{}
	paymentList := []string{}
	coinsToUse := []string{}

	for i := uint64(0); i < UTXOamount; i++ {
		valueList = append(valueList, AirdropCoinValue)
		paymentList = append(paymentList, paymentAddress)
	}
	chosenValue := uint64(0)

	coinsDataToUse := []coin.PlainCoin{}
	coinsToUseIdx := []uint64{}

	for _, v := range ada.UTXOList {
		cpubkey := v.Coin.GetPublicKey()
		if _, ok := ada.UTXOInUse[cpubkey.String()]; ok {
			continue
		}
		chosenValue += v.Coin.GetValue()
		coinsDataToUse = append(coinsDataToUse, v.Coin)
		coinsToUseIdx = append(coinsToUseIdx, v.Index)
		if chosenValue >= totalPRVNeeded {
			break
		}
	}
	for _, v := range coinsToUse {
		ada.UTXOInUse[v] = struct{}{}
	}

	txParam := incclient.NewTxParam(ada.Privatekey, paymentList, valueList, 0, nil, nil, nil)
	encodedTx, txHash, err := incClient.CreateRawTransactionWithInputCoins(txParam, coinsDataToUse, coinsToUseIdx)
	if err != nil {
		return nil, nil, "", err
	}

	txDetail := AirdropTxDetail{
		TxHash: txHash,
		Value:  totalPRVNeeded,
		Amount: UTXOamount,
	}
	return &txDetail, encodedTx, txHash, nil
}

func chooseAirdropAccount(totalValueNeeded uint64) *AirdropAccount {
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
	if result.TotalUTXO == 0 {
		getAirdropAccountUTXOs(result)
	}
	if len(result.UTXOInUse) == len(result.UTXOList) {
		getAirdropAccountUTXOs(result)
	}
	totalADAValue := uint64(0)
	for _, v := range result.UTXOList {
		totalADAValue += v.Coin.GetValue()
	}
	if totalValueNeeded > totalADAValue {
		goto retry
	}
	adc.airlock.Unlock()
	return result
}

func getAirdropAccountUTXOs(adc *AirdropAccount) {
	uxto, indices, err := incClient.GetUnspentOutputCoins(adc.Privatekey, common.PRVCoinID.String(), 0)
	if err != nil {
		panic(err)
	}
	var utxos []Coin
	for idx, v := range uxto {
		if v.GetVersion() == 2 {
			utxos = append(utxos, Coin{
				Coin:  v,
				Index: indices[idx].Uint64(),
			})
		}
	}
	adc.lock.Lock()
	adc.TotalUTXO = len(utxos)
	adc.UTXOList = utxos
	adc.UTXOInUse = make(map[string]struct{})
	adc.lock.Unlock()
}
