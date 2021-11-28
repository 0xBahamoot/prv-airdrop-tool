package main

import (
	"fmt"
	"github.com/incognitochain/go-incognito-sdk-v2/common"
	"github.com/incognitochain/go-incognito-sdk-v2/incclient"
	"log"
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
		time.Sleep(5 * time.Second)
	}
}

// UpdateAccount updates UTXOs of an account.
func (am *AccountManager) UpdateAccount(privateKey string) {
	account := am.Accounts[privateKey]
	if account == nil {
		log.Println("account not found")
		return
	}

	for {
		account.Update()
		time.Sleep(120 * time.Second)
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

// GetRandomAccount returns a random account for a given shard.
func (am *AccountManager) GetRandomAccount(shardID byte) (*AccountInfo, error) {
	for _, acc := range am.Accounts {
		if acc.available { // skip if account not ready
			if shardID >= byte(common.MaxShardNumber) {
				return acc, nil
			}
			if acc.ShardID == shardID {
				return acc, nil
			}
		}
	}
	return nil, fmt.Errorf("no account found for shard %v", shardID)
}

func (am *AccountManager) manageNFTs() {
	for {
		for _, acc := range am.Accounts {
			if !acc.available { // skip if account not ready
				continue
			}
			myNFTs, err := acc.GetMyNFTs()
			if err != nil {
				log.Println(err)
				continue
			}
			if len(myNFTs) < 20 && !acc.isMinting { // avoid multiple minting
				go func() {
					log.Printf("Minting NFTs for account %v, numNFTs %v\n", acc.toString(), len(myNFTs))
					acc.updateMintingStatus(true)
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
			if !acc.available { // skip if account not ready
				continue
			}
			utxoList, err := acc.GetUTXOsByAmount(common.PRVIDStr, incclient.DefaultPRVFee)
			if err != nil {
				log.Println(err)
				continue
			}
			if len(utxoList) < 20 && !acc.isSplitting {
				go func() {
					acc.updateSplittingStatus(true)
					log.Printf("Splitting PRV for account %v, numFeeUTXOs %v\n", acc.toString(), len(utxoList))
					err = splitPRV(acc, 100, numSplitPRVs)
					if err != nil {
						log.Printf("splitPRV for account %v error: %v\n", acc.toString(), err)
					}
					acc.updateSplittingStatus(false)
				}()
				time.Sleep(1 * time.Second)
			}
		}
		time.Sleep(time.Duration(defaultSleepTime) * time.Second)
	}
}
