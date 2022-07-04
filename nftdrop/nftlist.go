package main

import (
	"sync"
	"time"
)

var nftListLock sync.Mutex

func getNFTList() (map[string]uint64, error) {
	nftListLock.Lock()
	defer nftListLock.Unlock()
	result := make(map[string]uint64)

	err := cacheGet("nftlist", &result)
	if err != nil {
		if err.Error() != "item not exist" {
			return nil, err
		}
	} else {
		return result, nil
	}

	nftList, err := fetchNFTList()
	if err != nil {
		return nil, err
	}
	err = cacheStoreCustom("nftlist", nftList, 40*time.Second)
	if err != nil {
		return nil, err
	}
	result = nftList
	return result, nil
}

func fetchNFTList() (map[string]uint64, error) {
	nftTokens, err := incClient.GetListNftIDs(0)
	if err != nil {
		return nil, err
	}
	return nftTokens, nil
}
