package main

import (
	"fmt"
	"github.com/incognitochain/go-incognito-sdk-v2/coin"
	"github.com/incognitochain/go-incognito-sdk-v2/common"
	"github.com/incognitochain/go-incognito-sdk-v2/common/base58"
	"github.com/incognitochain/go-incognito-sdk-v2/incclient"
	"log"
	"sort"
	"sync"
	"time"
)

// Coin combines a PlainCoin and its index.
type Coin struct {
	Coin  coin.PlainCoin
	Index uint64
}

// TokenInfo manages UTXOs for a token.
type TokenInfo struct {
	UTXOList map[string]Coin

	// UTXOState keeps track up the status of UTXOs:
	//	-1: not found
	//	0: unspent
	//  1: temporarily used (in pool)
	//	2: spent (in block)
	UTXOState *sync.Map
	IsNFT     bool
}

func NewTokenInfo(isNFT bool) *TokenInfo {
	usedList := new(sync.Map)
	utxoList := make(map[string]Coin)

	return &TokenInfo{
		UTXOList:  utxoList,
		UTXOState: usedList,
		IsNFT:     isNFT,
	}
}

// getUTXOState returns the state of a UTXO:
//	-1: not found
//	0: unspent
//  1: temporarily used (in pool)
//	2: spent (in block)
func (t *TokenInfo) getUTXOState(snStr string) int {
	val, ok := t.UTXOState.Load(snStr)
	if !ok {
		return -1
	}
	state, ok := val.(int)
	if !ok {
		return -1
	}
	return state
}

func (t *TokenInfo) updateUTXOState(snStr string, state int) {
	t.UTXOState.Store(snStr, state)
}

// AccountInfo manages UTXOs for an Incognito account.
type AccountInfo struct {
	available   bool
	mtx         *sync.RWMutex
	isMinting   bool
	isSplitting bool

	TokenList      map[string]*TokenInfo
	PaymentAddress string
	PublicKey      string
	ReadonlyKey    string
	PrivateOTAKey  string
	PrivateKey     string
	ShardID        byte
}

// NewAccountFromPrivateKey creates a new AccountInfo given a private key.
func NewAccountFromPrivateKey(privateKey string) (*AccountInfo, error) {
	addr := incclient.PrivateKeyToPaymentAddress(privateKey, -1)
	pubKey := incclient.PrivateKeyToPublicKey(privateKey)
	readonlyKey := incclient.PrivateKeyToReadonlyKey(privateKey)
	otaKey := incclient.PrivateKeyToPrivateOTAKey(privateKey)
	shardID := incclient.GetShardIDFromPrivateKey(privateKey)
	mtx := new(sync.RWMutex)

	account := &AccountInfo{
		mtx:            mtx,
		TokenList:      nil,
		PaymentAddress: addr,
		PublicKey:      base58.Base58Check{}.Encode(pubKey, 0x00),
		ReadonlyKey:    readonlyKey,
		PrivateOTAKey:  otaKey,
		PrivateKey:     privateKey,
		ShardID:        shardID,
	}

	//go account.Update()

	return account, nil
}

func (account AccountInfo) toString() string {
	return fmt.Sprintf("%v...%v", account.PrivateKey[:5], account.PrivateKey[len(account.PrivateKey)-5:])
}

func (account AccountInfo) isAvailable() bool {
	res := false
	account.mtx.Lock()
	res = account.available
	account.mtx.Unlock()

	return res
}

func (account *AccountInfo) updateAvailableStatus(status bool) {
	account.mtx.Lock()
	if account.available != status {
		account.available = status
		logger.Printf("Account %v: available %v\n", account.toString(), account.available)
	}
	account.mtx.Unlock()
}

func (account *AccountInfo) updateMintingStatus(status bool) {
	account.mtx.Lock()
	account.isMinting = status
	logger.Printf("Account %v: isMinting %v\n", account.toString(), account.isMinting)
	account.mtx.Unlock()
}

func (account *AccountInfo) updateSplittingStatus(status bool) {
	account.mtx.Lock()
	account.isSplitting = status
	logger.Printf("Account %v: isSplitting %v\n", account.toString(), account.isSplitting)
	account.mtx.Unlock()
}

func (account AccountInfo) clone() *AccountInfo {
	account.mtx.Lock()
	newAccount := &AccountInfo{
		available:      account.available,
		isSplitting:    account.isSplitting,
		isMinting:      account.isMinting,
		mtx:            new(sync.RWMutex),
		PaymentAddress: account.PaymentAddress,
		PublicKey:      account.PublicKey,
		ReadonlyKey:    account.ReadonlyKey,
		PrivateOTAKey:  account.PrivateOTAKey,
		PrivateKey:     account.PrivateKey,
		ShardID:        account.ShardID,
	}
	tokenList := make(map[string]*TokenInfo)
	for tokenID, tokenInfo := range account.TokenList {
		utxoList := make(map[string]Coin)
		for snStr, utxo := range tokenInfo.UTXOList {
			utxoList[snStr] = utxo
		}
		tmpTokenInfo := TokenInfo{
			UTXOList:  utxoList,
			UTXOState: tokenInfo.UTXOState,
			IsNFT:     tokenInfo.IsNFT,
		}
		tokenList[tokenID] = &tmpTokenInfo
	}
	account.mtx.Unlock()
	newAccount.TokenList = tokenList

	return newAccount
}

// GetTokenInfo returns the TokenInfo of an account given the tokenID.
func (account AccountInfo) GetTokenInfo(tokenID string) *TokenInfo {
	clonedAcc := account.clone()
	tokenInfo, _ := clonedAcc.TokenList[tokenID]

	return tokenInfo
}

// GetBalance returns the balance of a tokenID.
func (account AccountInfo) GetBalance(tokenID string) uint64 {
	balance := uint64(0)
	utxoList, err := account.GetListUnspentOutput(tokenID)
	if err != nil {
		return 0
	}
	for _, utxo := range utxoList {
		balance += utxo.Coin.GetValue()
	}
	return balance
}

// GetListUnspentOutput returns the UTXOs (and their indices) of an AccountInfo w.r.t the given tokenID.
// The returned order is descending in terms of amount.
func (account AccountInfo) GetListUnspentOutput(tokenID string) ([]Coin, error) {
	tokenInfo := account.GetTokenInfo(tokenID)
	utxoResult := make([]Coin, 0)
	if tokenInfo != nil {
		for snStr, outCoin := range tokenInfo.UTXOList {
			if tokenInfo.getUTXOState(snStr) <= 0 {
				utxoResult = append(utxoResult, outCoin)
			}
		}
		return sortUTXOs(utxoResult), nil
	}
	return nil, fmt.Errorf("no UTXO found")
}

// GetUTXOsByAmount returns the list of UTXOs whose amount equals to the given amount.
func (account AccountInfo) GetUTXOsByAmount(tokenID string, amount uint64) ([]Coin, error) {
	utxoList, err := account.GetListUnspentOutput(tokenID)
	if err != nil {
		return nil, err
	}
	res := make([]Coin, 0)
	for _, utxo := range utxoList {
		if utxo.Coin.GetValue() == amount {
			res = append(res, utxo)
		}
	}
	return res, nil
}

// Update re-scans the UTXOs of the account.
func (account *AccountInfo) Update() {
	var err error
	defer func() {
		account.updateAvailableStatus(err == nil)
	}()
	accName := account.toString()
	logger.Printf("RE-SYNC ACCOUNT %v\n", accName)

	start := time.Now()
	tokenInfoList := make(map[string]*TokenInfo, 0)
	nftTokens, err := incClient.GetListNftIDs(0)
	if err != nil {
		logger.Printf("%v: GetListNftIDs error: %v\n", accName, err)
		return
	}
	cloneAccount := account.clone()

	allUTXOs, allIndices, err := incClient.GetAllUTXOsV2(cloneAccount.PrivateKey)
	if err != nil {
		logger.Printf("%v: GetAllUTXOsV2 error: %v\n", accName, err)
		return
	}
	nftCount := 0
	for tokenID, utxoList := range allUTXOs {
		if tokenID == common.ConfidentialAssetID.String() {
			continue
		}
		tokenInfo := cloneAccount.GetTokenInfo(tokenID)
		if tokenInfo == nil {
			isNFT := false
			if nftTokens != nil {
				_, isNFT = nftTokens[tokenID]
			}

			tokenInfo = NewTokenInfo(isNFT)
		}
		balance := uint64(0)
		listUnspent := tokenInfo.UTXOList
		tmpMapSNStr := make(map[string]interface{})
		for i, utxo := range utxoList {
			if utxo.GetVersion() != 2 {
				continue
			}
			snStr := base58.Base58Check{}.Encode(utxo.GetKeyImage().ToBytesS(), common.ZeroByte)
			tmpMapSNStr[snStr] = true
			if _, ok := listUnspent[snStr]; ok {
				if tokenInfo.getUTXOState(snStr) <= 0 {
					tokenInfo.updateUTXOState(snStr, 0)
					balance += utxo.GetValue()
				}
			} else {
				listUnspent[snStr] = Coin{Coin: utxo, Index: allIndices[tokenID][i].Uint64()}
				tokenInfo.updateUTXOState(snStr, 0)
				balance += utxo.GetValue()
			}
		}
		for snStr, _ := range listUnspent {
			if _, ok := tmpMapSNStr[snStr]; !ok {
				tokenInfo.updateUTXOState(snStr, 2)
				delete(listUnspent, snStr)
			}
		}

		tokenInfo.UTXOList = listUnspent
		if balance > 0 {
			tokenInfoList[tokenID] = tokenInfo
			if tokenID == common.PRVIDStr {
				logger.Printf("%v: balancePRV %v, numUTXOs: %v\n", accName, balance, len(listUnspent))
			}
			if tokenInfo.IsNFT {
				nftCount++
			}
		}
	}
	logger.Printf("%v: numNFTs %v\n", accName, nftCount)
	logger.Printf("RE-SYNC ACCOUNT %v FINISHED, TIME %v\n\n", accName, time.Since(start).Seconds())

	account.mtx.Lock()
	account.TokenList = tokenInfoList
	account.mtx.Unlock()
}

// ChooseBestUTXOs chooses a list of UTXOs to spend depending on the given amount.
func (account AccountInfo) ChooseBestUTXOs(tokenID string, requiredAmount uint64) ([]Coin, error) {
	balance := account.GetBalance(tokenID)
	if balance < requiredAmount {
		return nil, fmt.Errorf("insufficient balance, got %v, need %v", balance, requiredAmount)
	}

	// try to get the best fit first
	tmpUTXOsList, err := account.GetUTXOsByAmount(tokenID, requiredAmount)
	if err != nil {
		return nil, err
	}
	if len(tmpUTXOsList) > 0 {
		return []Coin{tmpUTXOsList[common.RandInt()%len(tmpUTXOsList)]}, nil
	}

	utxoList, err := account.GetListUnspentOutput(tokenID)
	if err != nil {
		return nil, err
	}
	if len(utxoList) == 0 {
		return nil, fmt.Errorf("no UTXO found for %v", tokenID)
	}

	if balance == requiredAmount {
		return utxoList, nil
	}

	coinsToSpend := make([]Coin, 0)
	remainAmount := requiredAmount
	totalChosenAmount := uint64(0)

	for i := 0; i < len(utxoList)-1; i++ {
		if utxoList[i].Coin.GetValue() > remainAmount {
			if utxoList[i+1].Coin.GetValue() >= remainAmount {
				continue
			} else {
				coinsToSpend = append(coinsToSpend, utxoList[i])
				totalChosenAmount += utxoList[i].Coin.GetValue()
				break
			}
		} else {
			coinsToSpend = append(coinsToSpend, utxoList[i])
			remainAmount -= utxoList[i].Coin.GetValue()
			totalChosenAmount += utxoList[i].Coin.GetValue()
		}
	}

	if totalChosenAmount < requiredAmount && len(utxoList) > 0 {
		totalChosenAmount += utxoList[len(utxoList)-1].Coin.GetValue()
		coinsToSpend = append(coinsToSpend, utxoList[len(utxoList)-1])
		if totalChosenAmount < requiredAmount {
			return nil, fmt.Errorf("not enough coin to spend")
		}
	}

	return coinsToSpend, nil
}

// GetMyNFTs returns all NFTs owned by an account.
func (account AccountInfo) GetMyNFTs() (map[string]*TokenInfo, error) {
	res := make(map[string]*TokenInfo)
	clonedAcc := account.clone()
	for tokenID, tokenInfo := range clonedAcc.TokenList {
		if !tokenInfo.IsNFT {
			continue
		}
		if clonedAcc.GetBalance(tokenID) == 1 {
			res[tokenID] = tokenInfo
		}
	}
	return res, nil
}

// GetRandomNFT returns a random unspent NFT.
func (account AccountInfo) GetRandomNFT() (string, error) {
	myNFTs, err := account.GetMyNFTs()
	if err != nil {
		return "", err
	}
	for tokenID, _ := range myNFTs {
		return tokenID, nil
	}

	return "", fmt.Errorf("no NFT available")
}

// ClearTempUsed clears the temporarily used status of a list of TXOs.
func (account *AccountInfo) ClearTempUsed(tokenID string, coinList []Coin) {
	logger.Printf("[ClearTempUsed] account %v: %v - %v\n", account.toString(), tokenID, coinList[0].Index)
	account.mtx.Lock()
	if tokenInfo, ok := account.TokenList[tokenID]; ok {
		for _, pCoin := range coinList {
			snStr := base58.Base58Check{}.Encode(pCoin.Coin.GetKeyImage().ToBytesS(), common.ZeroByte)
			val, ok := tokenInfo.UTXOState.Load(snStr)
			if !ok {
				continue
			}
			state, ok := val.(int)
			if !ok || state == 2 {
				continue
			}
			tokenInfo.updateUTXOState(snStr, 0)
		}
	}
	account.mtx.Unlock()
}

// MarkTempUsed temporarily marks a list of TXOs as used.
func (account *AccountInfo) MarkTempUsed(tokenID string, coinList []Coin) {
	//logger.Printf("[MarkTempUsed] account %v: %v - %v\n", account.toString(), tokenID, coinList[0].Index)
	account.mtx.Lock()
	if tokenInfo, ok := account.TokenList[tokenID]; ok {
		for _, pCoin := range coinList {
			snStr := base58.Base58Check{}.Encode(pCoin.Coin.GetKeyImage().ToBytesS(), common.ZeroByte)
			tokenInfo.updateUTXOState(snStr, 1)
		}
	}
	account.mtx.Unlock()
}

// MarkUsed marks a list of TXOs as used.
func (account *AccountInfo) MarkUsed(tokenID string, coinList []Coin) {
	account.mtx.Lock()
	if tokenInfo, ok := account.TokenList[tokenID]; ok {
		for _, pCoin := range coinList {
			snStr := base58.Base58Check{}.Encode(pCoin.Coin.GetKeyImage().ToBytesS(), common.ZeroByte)
			tokenInfo.updateUTXOState(snStr, 2)
			delete(tokenInfo.UTXOList, snStr)
		}
	}
	account.mtx.Unlock()
}

//// SyncAllUTXOs calls the remote node and update the list of UTXOs.
//func (account *AccountInfo) SyncAllUTXOs() (map[string][]coin.PlainCoin, map[string][]*big.Int, error) {
//	outCoinKey, err := incclient.NewOutCoinKeyFromPrivateKey(account.PrivateOTAKey)
//	if err != nil {
//		return nil, nil, err
//	}
//
//	prvOutputCoins := make([]jsonresult.ICoinInfo, 0)
//	prvIndices := make([]*big.Int, 0)
//	tmpPRVOutputCoins, tmpPRVIndices, err := incClient.GetOutputCoins(outCoinKey, common.PRVIDStr, 0)
//	if err != nil {
//		return nil, nil, err
//	}
//	prvTokenInfo := account.GetTokenInfo(common.PRVIDStr)
//	for i, outCoin := range tmpPRVOutputCoins {
//		if
//	}
//	prvDecryptedOutputCoins, listKeyImages, err := incclient.GetListDecryptedCoins(account.PrivateKey, prvOutputCoins)
//	if err != nil {
//		return nil, nil, err
//	}
//
//	tokenOutputCoins, tokenIndices, err := incClient.GetOutputCoins(outCoinKey, common.PRVIDStr, 0)
//	if err != nil {
//		return nil, nil, err
//	}
//}

// sortUTXOs sorts the given utxoList and idxList by their value in descending order.
func sortUTXOs(utxoList []Coin) []Coin {
	sort.Slice(utxoList, func(i, j int) bool {
		return utxoList[i].Coin.GetValue() > utxoList[j].Coin.GetValue()
	})

	return utxoList
}
