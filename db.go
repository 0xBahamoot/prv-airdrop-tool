package main

import (
	"github.com/pkg/errors"
	"github.com/syndtr/goleveldb/leveldb"
	lvdbErrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

var localdb *leveldb.DB

func initDB() error {
	handles := 256
	cache := 8
	dbPath := "airdrop"
	lvdb, err := leveldb.OpenFile(dbPath, &opt.Options{
		OpenFilesCacheCapacity: handles,
		BlockCacheCapacity:     cache / 2 * opt.MiB,
		WriteBuffer:            cache / 4 * opt.MiB, // Two of these are used internally
		Filter:                 filter.NewBloomFilter(10),
	})
	if _, corrupted := err.(*lvdbErrors.ErrCorrupted); corrupted {
		lvdb, err = leveldb.RecoverFile(dbPath, nil)
	}
	if err != nil {
		return errors.Wrapf(err, "levelvdb.OpenFile %s", dbPath)
	}
	localdb = lvdb
	return nil
}

func UpdateUserKey() {

}
