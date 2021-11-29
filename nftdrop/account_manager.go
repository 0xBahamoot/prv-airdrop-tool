package main

import (
	"fmt"
	"github.com/incognitochain/go-incognito-sdk-v2/common"
	"github.com/incognitochain/go-incognito-sdk-v2/incclient"
	"time"
)

// AccountManager implements a simple management tool for manipulating with Incognito accounts (i.e, privateKey).
type AccountManager struct {
	Accounts map[string]*AccountInfo
}

// NewAccountManager creates a new AccountManager and adds the given accounts to it.
func NewAccountManager(privateKeys []string) (*AccountManager, error) {
	accounts := make(map[string]*AccountInfo)
	for _, privateKey := range privateKeys {
		wl, err := NewAccountFromPrivateKey(privateKey)
		if err != nil {
			return nil, err
		}
		accounts[privateKey] = wl
	}

	return &AccountManager{Accounts: accounts}, nil
}

// GetAccount returns the account given its private key.
func (am *AccountManager) GetAccount(privateKey string) (*AccountInfo, error) {
	acc, ok := am.Accounts[privateKey]
	if !ok {
		return nil, fmt.Errorf("account not found")
	}

	return acc, nil
}

// Sync periodically updates UTXOs of all accounts.
func (am *AccountManager) Sync() {
	for privateKey := range am.Accounts {
		go am.UpdateAccount(privateKey)
		time.Sleep(1 * time.Second)
	}
}

// UpdateAccount updates UTXOs of an account.
func (am *AccountManager) UpdateAccount(privateKey string) {
	account := am.Accounts[privateKey]
	if account == nil {
		logger.Println("account not found")
		return
	}

	for {
		account.Update()
		time.Sleep(60 * time.Second)
	}
}

// GetBalance retrieves balance of a private key without sending this private key to the remote full node.
func (am *AccountManager) GetBalance(privateKey, tokenID string) (uint64, error) {
	if wl, ok := am.Accounts[privateKey]; ok {
		balance := wl.GetBalance(tokenID)
		return balance, nil
	}

	unspentCoins, _, err := incClient.GetUnspentOutputCoins(privateKey, tokenID, 0)
	if err != nil {
		return 0, err
	}

	balance := uint64(0)
	for _, unspentCoin := range unspentCoins {
		balance += unspentCoin.GetValue()
	}

	return balance, nil
}

// GetRandomAirdropAccount returns a random airdrop account for a given shard.
func (am *AccountManager) GetRandomAirdropAccount(shardID byte) (*AccountInfo, error) {
	for _, acc := range am.Accounts {
		nftList, _ := acc.GetMyNFTs()
		if acc.isAvailable() && !acc.isMinting && len(nftList) > 0 { // skip if account not ready
			if shardID >= byte(common.MaxShardNumber) {
				return acc, nil
			}
			if acc.ShardID == shardID {
				return acc, nil
			}
		}
	}
	if shardID < byte(common.MaxShardNumber) {
		return am.GetRandomAirdropAccount(255)
	}
	return nil, fmt.Errorf("no account found for shard %v", shardID)
}

func (am *AccountManager) manageNFTs() {
	for {
		for _, acc := range am.Accounts {
			if !acc.isAvailable() { // skip if account not ready
				continue
			}
			myNFTs, err := acc.GetMyNFTs()
			if err != nil {
				logger.Println(err)
				continue
			}
			logger.Printf("[manageNFTs] Account %v, isMinting %v, #NFTs %v\n", acc.toString(), acc.isMinting, len(myNFTs))
			if len(myNFTs) < 20 && !acc.isMinting { // avoid multiple minting
				acc.updateMintingStatus(true)
				go func() {
					logger.Printf("Minting NFTs for account %v, numNFTs %v\n", acc.toString(), len(myNFTs))
					mintNFTMany(acc, numMintBatchNFTs)
					time.Sleep(time.Duration(defaultSleepTime) * time.Second)
					acc.updateMintingStatus(false)
				}()
				time.Sleep(1 * time.Second)
			}
		}
		time.Sleep(time.Duration(defaultSleepTime) * time.Second)
	}
}

func (am *AccountManager) managePRVUTXOs() {
	for {
		for _, acc := range am.Accounts {
			if !acc.isAvailable() { // skip if account not ready
				continue
			}
			utxoList, err := acc.GetUTXOsByAmount(common.PRVIDStr, incclient.DefaultPRVFee)
			if err != nil {
				logger.Println(err)
				continue
			}
			logger.Printf("[managePRVUTXOs] Account %v, isSplitting %v, #UTXOs %v\n", acc.toString(), acc.isSplitting, len(utxoList))
			if len(utxoList) < 20 && !acc.isSplitting {
				acc.updateSplittingStatus(true)
				go func() {
					logger.Printf("Splitting PRV for account %v, numFeeUTXOs %v\n", acc.toString(), len(utxoList))
					err = splitPRV(acc, 100, numSplitPRVs)
					if err != nil {
						logger.Printf("splitPRV for account %v error: %v\n", acc.toString(), err)
					} else {
						time.Sleep(5 * time.Minute)
					}
					acc.updateSplittingStatus(false)
				}()
				time.Sleep(1 * time.Second)
			}
		}
		time.Sleep(time.Duration(defaultSleepTime) * time.Second)
	}
}
