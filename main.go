package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"main/slacknoti"
	"math"
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
	LastAirdropRequest int64
	AirdropSuccess     bool
}

type AirdropAccount struct {
	lock           sync.Mutex
	Privatekey     string
	PaymentAddress string
	TotalUTXO      int
	ShardID        int
	UTXOList       []Coin
	UTXOInUse      map[string]struct{}
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
	go slacknoti.StartSlackHook()
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
			log.Printf("SubmitKey %v encoutered an error: %v\n", key, err)
		}
	}
	adc.lastUsedADA = 0
	airdroppedUser, err := LoadUserAirdropInfo()
	if err != nil {
		panic(err)
	}
	for _, v := range airdroppedUser {
		adc.UserAccounts[v.Pubkey] = v
		if len(v.OngoingTxs) != 0 {
			ctx, _ := context.WithTimeout(context.Background(), 35*time.Minute)
			go watchUserAirdropStatus(v, ctx)
		}
	}
	r := gin.Default()

	r.POST("/requestdrop", APIReqDrop)
	r.POST("/faucet", APIFaucet)

	r.Run("0.0.0.0:" + strconv.Itoa(config.Port))
	select {}
}

type RequestAirdrop struct {
	PaymentAddress string `json:"paymentaddress"`
	Captcha        string `json:"captcha"`
}

func APIFaucet(c *gin.Context) {
	var req RequestAirdrop
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"Error": err.Error()})
		return
	}

	if ok, err := VerifyCaptcha(req.Captcha, config.CaptchaSecret); !ok {
		if err != nil {
			log.Println("VerifyCaptcha", err)
			c.JSON(http.StatusBadRequest, gin.H{"Error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"Error": errors.New("invalid captcha").Error()})
		return
	}

	paymentkey := req.PaymentAddress
	pubkey := ""
	if paymentkey == "" && pubkey == "" {
		c.JSON(http.StatusOK, gin.H{
			"Result": -1,
		})
		return
	}
	key := pubkey
	shardID := 0
	if paymentkey != "" {
		wl, err := wallet.Base58CheckDeserialize(paymentkey)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"Result": 0,
				"Error":  err.Error(),
			})
			return
		}
		if wl.KeySet.PaymentAddress.GetOTAPublicKey() == nil ||
			wl.KeySet.PaymentAddress.GetPublicSpend() == nil ||
			wl.KeySet.PaymentAddress.GetPublicView() == nil {
			c.JSON(http.StatusOK, gin.H{
				"Result": 0,
				"Error":  fmt.Errorf("invalid payment address").Error(),
			})
			return
		}

		shardID = int(common.GetShardIDFromLastByte(wl.KeySet.PaymentAddress.Pk[31]))
		key = base58.Base58Check{}.Encode(wl.KeySet.PaymentAddress.Pk, 0)
	}
	if pubkey != "" {
		pubkeyBytes, _, err := base58.Base58Check{}.Decode(pubkey)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"Result": -1,
				"Error":  err.Error(),
			})
			return
		}
		shardID = int(common.GetShardIDFromLastByte(pubkeyBytes[31]))
	}
	adc.userlock.RLock()
	user, ok := adc.UserAccounts[key]
	adc.userlock.RUnlock()
	if ok {
		_ = user
		t := time.Unix(user.LastAirdropRequest, 0)
		// if time.Since(t) < 30*time.Minute {
		// 	c.JSON(http.StatusOK, gin.H{
		// 		"Result": 0,
		// 	})
		// 	return
		// }
		// go AirdropUser(user)
		r := 0
		if time.Since(t) <= 30*time.Minute && !user.AirdropSuccess {
			r = 1
			c.JSON(http.StatusOK, gin.H{
				"Result": r,
			})
			return
		}
		if user.AirdropSuccess {
			r = 2
			c.JSON(http.StatusOK, gin.H{
				"Result": r,
			})
			return
		}
	}

	newUserAccount := new(UserAccount)
	newUserAccount.PaymentAddress = paymentkey
	newUserAccount.Pubkey = pubkey
	newUserAccount.ShardID = shardID
	newUserAccount.Txs = make(map[string]*AirdropTxDetail)
	adc.userlock.Lock()
	adc.UserAccounts[key] = newUserAccount
	adc.userlock.Unlock()

	go AirdropUser(newUserAccount, false)
	c.JSON(http.StatusOK, gin.H{
		"Result": 1,
	})
}

func APIReqDrop(c *gin.Context) {
	var req RequestAirdrop
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"Error": err.Error()})
		return
	}
	paymentkey := req.PaymentAddress
	pubkey := ""
	forShield := "true"
	if paymentkey == "" && pubkey == "" {
		c.JSON(http.StatusOK, gin.H{
			"Result": -1,
		})
		return
	}
	key := pubkey
	shardID := 0
	if paymentkey != "" {
		wl, err := wallet.Base58CheckDeserialize(paymentkey)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"Result": 0,
				"Error":  err,
			})
			return
		}
		if wl.KeySet.PaymentAddress.GetOTAPublicKey() == nil ||
			wl.KeySet.PaymentAddress.GetPublicSpend() == nil ||
			wl.KeySet.PaymentAddress.GetPublicView() == nil {
			c.JSON(http.StatusOK, gin.H{
				"Result": 0,
				"Error":  fmt.Errorf("invalid payment address"),
			})
			return
		}

		shardID = int(common.GetShardIDFromLastByte(wl.KeySet.PaymentAddress.Pk[31]))
		key = base58.Base58Check{}.Encode(wl.KeySet.PaymentAddress.Pk, 0)
	}
	if pubkey != "" {
		pubkeyBytes, _, err := base58.Base58Check{}.Decode(pubkey)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"Result": -1,
				"Error":  err,
			})
			return
		}
		shardID = int(common.GetShardIDFromLastByte(pubkeyBytes[31]))
	}
	adc.userlock.RLock()
	user, ok := adc.UserAccounts[key]
	adc.userlock.RUnlock()
	if ok {
		_ = user
		t := time.Unix(user.LastAirdropRequest, 0)
		// if time.Since(t) < 30*time.Minute {
		// 	c.JSON(http.StatusOK, gin.H{
		// 		"Result": 0,
		// 	})
		// 	return
		// }
		// go AirdropUser(user)
		r := 0
		if time.Since(t) <= 30*time.Minute && !user.AirdropSuccess {
			r = 1
			c.JSON(http.StatusOK, gin.H{
				"Result": r,
			})
			return
		}
		if user.AirdropSuccess {
			r = 2
			c.JSON(http.StatusOK, gin.H{
				"Result": r,
			})
			return
		}
	}

	newUserAccount := new(UserAccount)
	newUserAccount.PaymentAddress = paymentkey
	newUserAccount.Pubkey = pubkey
	newUserAccount.ShardID = shardID
	newUserAccount.Txs = make(map[string]*AirdropTxDetail)
	adc.userlock.Lock()
	adc.UserAccounts[key] = newUserAccount
	adc.userlock.Unlock()
	shield := false
	if forShield == "true" {
		shield = true
	}
	go AirdropUser(newUserAccount, shield)
	c.JSON(http.StatusOK, gin.H{
		"Result": 1,
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

func AirdropUser(user *UserAccount, forShield bool) {
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
	if forShield {
		totalPRVAmountNeeded = AirdropCoinShieldValue
		totalPRVCoinsNeeded = 1

	} else {
		totalCount := len(user.TotalTokens)
		if totalCount == 0 {
			totalPRVAmountNeeded = uint64(1) * AirdropCoinValue
			totalPRVCoinsNeeded = 1
		} else {
			totalPRVAmountNeeded = uint64(len(user.TotalTokens)) * AirdropCoinValue
			totalPRVCoinsNeeded = len(user.TotalTokens)
		}
	}

	airdropAccount := chooseAirdropAccount(totalPRVAmountNeeded, user.ShardID, user.PaymentAddress)
	airdropAccount.lock.Lock()
	totalTxNeeded := int(math.Ceil(float64(totalPRVCoinsNeeded) / float64(MaxTxOutput)))
	txList := []string{}
	log.Printf("sending txs for user %v: %v %v %v", user.PaymentAddress, totalPRVAmountNeeded, totalPRVCoinsNeeded, totalTxNeeded)
	txsToSend := [][]byte{}
	for i := 0; i < totalTxNeeded; i++ {
		if i+1 == totalTxNeeded {
			txDetail, txBytes, txHash, err := CreateAirDropTx(airdropAccount, user.PaymentAddress, uint64(totalPRVCoinsNeeded-(i*MaxTxOutput)), forShield)
			if err != nil {
				panic(err)
			}
			txsToWatch = append(txsToWatch, txHash)
			txsToSend = append(txsToSend, txBytes)
			user.Txs[txHash] = txDetail
			txList = append(txList, txHash)
		} else {
			txDetail, txBytes, txHash, err := CreateAirDropTx(airdropAccount, user.PaymentAddress, MaxTxOutput, forShield)
			if err != nil {
				panic(err)
			}
			txsToWatch = append(txsToWatch, txHash)
			txsToSend = append(txsToSend, txBytes)
			user.Txs[txHash] = txDetail
			txList = append(txList, txHash)
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
		log.Printf("user %v sent success tx %v", user.PaymentAddress, txList[idx])
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

func CreateAirDropTx(ada *AirdropAccount, paymentAddress string, UTXOamount uint64, forShield bool) (*AirdropTxDetail, []byte, string, error) {
	log.Println("Creating tx with param", ada.Privatekey, paymentAddress, UTXOamount)
	coinValue := AirdropCoinValue
	if forShield {
		coinValue = AirdropCoinShieldValue
	}
	totalPRVNeeded := (UTXOamount * coinValue) + incclient.DefaultPRVFee
	valueList := []uint64{}
	paymentList := []string{}
	coinsToUse := []string{}

	for i := uint64(0); i < UTXOamount; i++ {
		valueList = append(valueList, coinValue)
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
		coinsToUse = append(coinsToUse, cpubkey.String())
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

func chooseAirdropAccount(totalValueNeeded uint64, shardID int, userAddress string) *AirdropAccount {
	var result *AirdropAccount
	var i int
retry:
	if i+1 == len(adc.AirdropAccounts) {
		time.Sleep(20 * time.Second)
		log.Println("can't choose airdrop account", userAddress, shardID)
		i = 0
	}
	i++
	adc.airlock.Lock()
	adc.lastUsedADA = (adc.lastUsedADA + 1) % len(adc.AirdropAccounts)
	result = adc.AirdropAccounts[adc.lastUsedADA]
	if result.ShardID != shardID {
		adc.airlock.Unlock()
		goto retry
	}
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
		msg := fmt.Sprintf("airdrop %v acc %v totalValueNeeded %v > totalADAValue %v \n", result.ShardID, result.PaymentAddress, totalValueNeeded, totalADAValue)
		log.Println(msg)
		go slacknoti.SendSlackNoti(msg)
		adc.airlock.Unlock()
		goto retry
	} else {
		if totalADAValue < 5*1e9 {
			msg := fmt.Sprintf("airdrop %v acc %v totalADAValue %v < %v \n", result.ShardID, result.PaymentAddress, totalADAValue, 5*1e9)
			log.Println(msg)
			go slacknoti.SendSlackNoti(msg)
		}
	}
	adc.airlock.Unlock()
	return result
}

func getAirdropAccountUTXOs(adc *AirdropAccount) {
	uxto, indices, err := incClient.GetUnspentOutputCoins(adc.Privatekey, common.PRVCoinID.String(), 0)
	if err != nil {
		panic(err)
	}
	if len(uxto) == 0 {
		log.Println("no utxo for airdrop account", adc.ShardID)
		return
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
	UTXOInUsePendingList := make(map[string]struct{})
	for _, v := range utxos {
		if _, ok := adc.UTXOInUse[v.Coin.GetPublicKey().String()]; ok {
			UTXOInUsePendingList[v.Coin.GetPublicKey().String()] = struct{}{}
		}
	}
	adc.UTXOInUse = UTXOInUsePendingList
	adc.lock.Unlock()
}
