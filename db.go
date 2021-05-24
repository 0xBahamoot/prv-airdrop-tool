package main

import (
	"encoding/json"

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
	dbPath := "airdropdb"
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

func UpdateUserAirdropInfo(user *UserAccount) error {
	userBytes, err := json.Marshal(user)
	if err != nil {
		return err
	}
	err = localdb.Put([]byte(user.PaymentAddress), userBytes, nil)
	return err
}

func LoadUserAirdropInfo() ([]*UserAccount, error) {
	var result []*UserAccount
	iter := localdb.NewIterator(nil, nil)
	for iter.Next() {
		// Remember that the contents of the returned slice should not be modified, and
		// only valid until the next call to Next.
		// key := iter.Key()
		userAcc := new(UserAccount)
		value := iter.Value()
		err := json.Unmarshal(value, userAcc)
		if err != nil {
			return nil, err
		}
		result = append(result, userAcc)
	}
	iter.Release()
	err := iter.Error()
	return result, err
}
