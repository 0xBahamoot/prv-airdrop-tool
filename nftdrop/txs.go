package main

import (
	"context"
	"fmt"
	"github.com/incognitochain/go-incognito-sdk-v2/coin"
	"github.com/incognitochain/go-incognito-sdk-v2/common"
	"github.com/incognitochain/go-incognito-sdk-v2/incclient"
	metadataPdexv3 "github.com/incognitochain/go-incognito-sdk-v2/metadata/pdexv3"
	"github.com/incognitochain/go-incognito-sdk-v2/wallet"
	"math"
	"strings"
	"time"
)

func transferPRV(acc *AccountInfo, addrList []string, amountList []uint64, doneChan chan string, errChan chan error) error {
	requiredAmount := incclient.DefaultPRVFee
	for _, amt := range amountList {
		requiredAmount += amt
	}
	balance := acc.GetBalance(common.PRVIDStr)
	if balance < requiredAmount {
		err := fmt.Errorf("insufficient PRV amount: required %v, got %v", requiredAmount, balance)
		if errChan != nil {
			errChan <- err
		}

		return err
	}
	coinsToSpend, err := acc.ChooseBestUTXOs(common.PRVIDStr, requiredAmount)
	if err != nil {
		if errChan != nil {
			errChan <- err
		}
		return err
	}

	coinList := make([]coin.PlainCoin, 0)
	idxList := make([]uint64, 0)
	for _, c := range coinsToSpend {
		coinList = append(coinList, c.Coin)
		idxList = append(idxList, c.Index)
	}

	txParam := incclient.NewTxParam(acc.PrivateKey, addrList, amountList, 0, nil, nil, nil)
	encodedTx, txHash, err := incClient.CreateRawTransactionWithInputCoins(txParam, coinList, idxList)
	if err != nil {
		if errChan != nil {
			errChan <- err
		}

		return err
	}
	err = incClient.SendRawTx(encodedTx)
	if err != nil {
		if errChan != nil {
			errChan <- err
		}
		return err
	}
	acc.MarkTempUsed(common.PRVIDStr, coinsToSpend)
	logger.Printf("TransferPRV %v TxHash: %v\n", acc.toString(), txHash)

	waitingCheckTxInBlock(acc, txHash, common.PRVIDStr, coinsToSpend)
	if doneChan != nil {
		doneChan <- txHash
	}

	return nil
}

func splitPRV(acc *AccountInfo, amountForEach uint64, numUTXOs int) error {
	logger.Printf("SPLIT PRV FOR ACCOUNT %v WITH AMOUNT %v, NUM %v\n", acc.toString(), amountForEach, numUTXOs)
	if numUTXOs < 0 {
		return nil
	}
	timeOut := 20 * int(math.Ceil(float64(numUTXOs)/float64(incclient.MaxOutputSize*incclient.MaxOutputSize)))
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeOut)*time.Minute)
	defer cancel()

	var err error
	remaining := numUTXOs
	start := time.Now()
	for remaining > 0 {
		logger.Printf("splitPRV %v remaining: %v\n", acc.toString(), remaining)
		if remaining <= incclient.MaxOutputSize {
			addrList := make([]string, 0)
			amountList := make([]uint64, 0)
			for i := 0; i < remaining; i++ {
				addrList = append(addrList, acc.PaymentAddress)
				amountList = append(amountList, amountForEach)
			}
			err = transferPRV(acc, addrList, amountList, nil, nil)
			if err != nil {
				if !strings.Contains(err.Error(), "reject") {
					logger.Printf("transferPRV %v error: %v\n", acc.toString(), err)
				}
				time.Sleep(40 * time.Second)
				continue
			}
			remaining = 0
		} else {
			tmpNumUTXOs := remaining
			if tmpNumUTXOs > incclient.MaxOutputSize*incclient.MaxOutputSize {
				tmpNumUTXOs = incclient.MaxOutputSize * incclient.MaxOutputSize
			}
			numTxs := int(math.Ceil(float64(tmpNumUTXOs) / float64(incclient.MaxOutputSize)))

			addrList := make([]string, 0)
			amountList := make([]uint64, 0)
			for i := 0; i < numTxs; i++ {
				addrList = append(addrList, acc.PaymentAddress)
				amountList = append(amountList, amountForEach*incclient.MaxOutputSize+incclient.DefaultPRVFee)
			}
			err = transferPRV(acc, addrList, amountList, nil, nil)
			if err != nil {
				if !strings.Contains(err.Error(), "reject") && !strings.Contains(err.Error(), "Reject") {
					logger.Printf("transferPRV %v error: %v\n", acc.toString(), err)
				}

				time.Sleep(30 * time.Second)
				continue
			}
			acc.Update()

			doneChan := make(chan string, numTxs)
			errChan := make(chan error, numTxs)

			addrList = make([]string, 0)
			amountList = make([]uint64, 0)
			for i := 0; i < incclient.MaxOutputSize; i++ {
				addrList = append(addrList, acc.PaymentAddress)
				amountList = append(amountList, amountForEach)
			}
			for i := 0; i < numTxs; i++ {
				go func() {
					err = transferPRV(acc, addrList, amountList, doneChan, errChan)
					if err != nil {
						if !strings.Contains(err.Error(), "double spend") && !strings.Contains(err.Error(), "replacement or cancel") {
							logger.Printf("transferPRV %v error: %v\n", acc.toString(), err)
						}
					}
				}()
				time.Sleep(1 * time.Second)
			}

			errCount := 0
			doneCount := 0
			finished := false
			for {
				select {
				case <-ctx.Done():
					logger.Printf("splitPRV timed-out\n")
					return fmt.Errorf("time-out")
				case err := <-errChan:
					errCount++
					if !strings.Contains(err.Error(), "double spend") && !strings.Contains(err.Error(), "replacement or cancel") {
						logger.Printf("transferPRV error: %v\n", err)
					}
					go func() {
						time.Sleep(10 * time.Second)
						err = transferPRV(acc, addrList, amountList, doneChan, errChan)
						if err != nil {
							if !strings.Contains(err.Error(), "double spend") && !strings.Contains(err.Error(), "replacement or cancel") {
								logger.Println(err)
							}
						}
					}()
				case <-doneChan:
					doneCount++
					remaining -= incclient.MaxOutputSize
				default:
					if doneCount == numTxs {
						finished = true
						break
					}
					if errCount == numTxs {
						finished = true
						break
					}
					logger.Printf("splitPRV %v timeElapsed: %v, remaining: %v/%v, doneCount: %v, errCount: %v\n",
						acc.toString(), time.Since(start).Seconds(), remaining, numUTXOs, doneCount, errCount)
					time.Sleep(10 * time.Second)
				}
				if finished {
					time.Sleep(40 * time.Second)
					break
				}
			}
		}
	}
	logger.Printf("FINISHED SPLIT PRV FOR ACCOUNT %v: %v\n", acc.toString(), time.Since(start).Seconds())
	return nil
}

func mintNFT(acc *AccountInfo, doneChan chan string, errChan chan error) {
	logger.Printf("MINT NEW NFT FOR ACCOUNT %v\n", acc.toString())
	if minPRVRequired == 0 {
		minPRVRequired = incClient.GetMinPRVRequiredToMintNFT(0)
	}
	requiredAmount := minPRVRequired + incclient.DefaultPRVFee
	balance := acc.GetBalance(common.PRVIDStr)
	if balance < requiredAmount {
		errChan <- fmt.Errorf("insufficient PRV amount: required %v, got %v", requiredAmount, balance)
		return
	}

	senderWallet, err := wallet.Base58CheckDeserialize(acc.PrivateKey)
	if err != nil {
		errChan <- err
		return
	}
	otaReceiver := coin.OTAReceiver{}
	err = otaReceiver.FromAddress(senderWallet.KeySet.PaymentAddress)
	if err != nil {
		errChan <- err
		return
	}
	otaReceiveStr, err := otaReceiver.String()
	if minPRVRequired == 0 {
		minPRVRequired = incClient.GetMinPRVRequiredToMintNFT(0)
	}
	md := metadataPdexv3.NewUserMintNftRequestWithValue(otaReceiveStr, minPRVRequired)
	txParam := incclient.NewTxParam(acc.PrivateKey, []string{common.BurningAddress2}, []uint64{minPRVRequired}, 0, nil, md, nil)

	coinsToSpend, err := acc.ChooseBestUTXOs(common.PRVIDStr, requiredAmount)
	if err != nil {
		logger.Println(err)
		errChan <- err
		return
	}
	//logger.Printf("%v CoinToSpendIdx: %v, amount: %v, %v\n",
	//	acc.toString(), coinsToSpend[0].Index, coinsToSpend[0].Coin.GetValue(), time.Since(tmpStart).Seconds())

	coinList := make([]coin.PlainCoin, 0)
	idxList := make([]uint64, 0)
	for _, c := range coinsToSpend {
		coinList = append(coinList, c.Coin)
		idxList = append(idxList, c.Index)
	}

	encodedTx, txHash, err := incClient.CreateRawTransactionWithInputCoins(txParam, coinList, idxList)
	if err != nil {
		errChan <- err
		return
	}
	err = incClient.SendRawTx(encodedTx)
	if err != nil {
		errChan <- err
		return
	}
	acc.MarkTempUsed(common.PRVIDStr, coinsToSpend)

	go waitingCheckTxInBlock(acc, txHash, common.PRVIDStr, coinsToSpend)
	doneChan <- txHash
}

func mintNFTMany(acc *AccountInfo, numNFTs int) {
	logger.Printf("MINT %v NFTs FOR ACCOUNT %v\n", numNFTs, acc.toString())
	start := time.Now()
	if numNFTs < 0 {
		return
	}
	if minPRVRequired == 0 {
		minPRVRequired = incClient.GetMinPRVRequiredToMintNFT(0)
	}
	requiredAmountForEach := minPRVRequired + incclient.DefaultPRVFee
	requiredAmount := requiredAmountForEach * uint64(numNFTs)
	balance := acc.GetBalance(common.PRVIDStr)
	if balance < requiredAmount {
		logger.Printf("insufficient PRV amount: required %v, got %v\n", requiredAmount, balance)
		return
	}
	utxoList, err := acc.GetUTXOsByAmount(common.PRVIDStr, requiredAmountForEach)
	if err != nil {
		logger.Println(err)
		return
	}

	// split PRV before minting to boost up parallelized level
	if len(utxoList) < numNFTs {
		err = splitPRV(acc, requiredAmountForEach, numNFTs-len(utxoList))
		if err != nil {
			logger.Println(err)
			return
		}
	}

	timeOut := 30 * int(math.Ceil(float64(numNFTs)/float64(incclient.MaxOutputSize*incclient.MaxOutputSize)))
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeOut)*time.Minute)
	defer cancel()
	doneChan := make(chan string, numNFTs)
	errChan := make(chan error, numNFTs)

	for i := 0; i < numNFTs; i++ {
		go mintNFT(acc, doneChan, errChan)
		time.Sleep(200 * time.Millisecond)
	}

	errCount := 0
	doneCount := 0
	finished := false
	for {
		select {
		case <-ctx.Done():
			logger.Printf("mintNFT many timed-out\n")
			return
		case err := <-errChan:
			errCount++
			if !strings.Contains(err.Error(), "double spend") && !strings.Contains(err.Error(), "replacement or cancel") {
				logger.Printf("mintNFT %v error: %v\n", acc.toString(), err)
			}
			go func() {
				time.Sleep(10 * time.Second)
				mintNFT(acc, doneChan, errChan)
			}()
		case txHash := <-doneChan:
			logger.Printf("new mintNFT txHash for %v: %v\n", acc.toString(), txHash)
			doneCount++
		default:
			if doneCount == numNFTs {
				logger.Printf("Mint all %v NFTs SUCCEEDED\n\n", numNFTs)
				finished = true
				break
			}
			if errCount == numNFTs {
				logger.Printf("Mint NFTs FINISHED: done %v, failed %v\n\n", doneCount, errCount)
				finished = true
				return
			}
			logger.Printf("mintNFTMany %v timeElapsed: %v, doneCount: %v/%v, errCount: %v\n",
				acc.toString(), time.Since(start).Seconds(), doneCount, numNFTs, errCount)
			time.Sleep(5 * time.Second)
		}
		if finished {
			break
		}
	}
	logger.Printf("MINT %v NFTs FOR ACCOUNT %v FINISHED: %v!!!\n\n", numNFTs, acc.toString(), time.Since(start).Seconds())
}

func transferNFT(acc *AccountInfo, paymentAddress string) (string, string, error) {
	var err error
	prvCoinsToSpend, err := acc.ChooseBestUTXOs(common.PRVIDStr, incclient.DefaultPRVFee)
	if err != nil {
		return "", "", err
	}
	nftID, err := acc.GetRandomNFT()
	if err != nil {
		return "", "", err
	}
	//logger.Printf("%v PRVCoinToSpendIdx: %v, amount: %v, %v, NFT: %v\n",
	//	acc.toString(), prvCoinsToSpend[0].Index, prvCoinsToSpend[0].Coin.GetValue(), len(prvCoinsToSpend), nftID)
	nftCoinToSpend, err := acc.ChooseBestUTXOs(nftID, 1)
	if err != nil {
		return "", "", err
	}

	prvCoinList := make([]coin.PlainCoin, 0)
	prvIdxList := make([]uint64, 0)
	nftCoinList := make([]coin.PlainCoin, 0)
	nftIdxList := make([]uint64, 0)
	for _, c := range prvCoinsToSpend {
		prvCoinList = append(prvCoinList, c.Coin)
		prvIdxList = append(prvIdxList, c.Index)
	}
	for _, c := range nftCoinToSpend {
		nftCoinList = append(nftCoinList, c.Coin)
		nftIdxList = append(nftIdxList, c.Index)
	}

	txTokenParam := incclient.NewTxTokenParam(nftID, 1,
		[]string{paymentAddress}, []uint64{1}, false, 0, nil)
	txParam := incclient.NewTxParam(acc.PrivateKey, []string{}, []uint64{}, 0, txTokenParam, nil, nil)

	encodedTx, txHash, err := incClient.CreateRawTokenTransactionWithInputCoins(
		txParam, nftCoinList,
		nftIdxList, prvCoinList, prvIdxList)
	if err != nil {
		return "", "", err
	}
	err = incClient.SendRawTokenTx(encodedTx)
	if err != nil {
		return "", "", err
	}
	acc.MarkTempUsed(common.PRVIDStr, prvCoinsToSpend)
	acc.MarkTempUsed(nftID, nftCoinToSpend)
	logger.Printf("TransferNFT %v to %v TxHash: %v\n", acc.toString(),
		fmt.Sprintf("%v...%v", paymentAddress[:5], paymentAddress[len(paymentAddress)-5:]), txHash)

	go waitingCheckTxInBlock(acc, txHash, common.PRVIDStr, prvCoinsToSpend)
	go waitingCheckTxInBlock(acc, txHash, nftID, nftCoinToSpend)
	return txHash, nftID, nil
}
