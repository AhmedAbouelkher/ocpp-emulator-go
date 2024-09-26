package main

import (
	"errors"
	"strconv"

	"github.com/dgraph-io/badger/v4"
)

func KeyExists(key string) (bool, error) {
	txn := db.NewTransaction(true)
	defer txn.Discard()
	_, err := txn.Get([]byte(key))
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func GetKeyValue(key string) (string, error) {
	value := ""
	err := db.View(func(txn *badger.Txn) error {
		val, err := GetKeyValueTX(txn, key)
		if err != nil {
			return err
		}
		value = val
		return nil
	})
	return value, err
}

func GetKeyValueTX(txn *badger.Txn, key string) (string, error) {
	val, err := txn.Get([]byte(key))
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return "", nil
		}
		return "", err
	}
	v, err := val.ValueCopy(nil)
	if err != nil {
		return "", err
	}
	return string(v), nil
}

func MustGetIntKey(key string) int {
	val, _ := GetIntKey(key)
	return val
}

func GetIntKey(key string) (int, error) {
	txn := db.NewTransaction(true)
	defer txn.Discard()
	val, err := txn.Get([]byte(key))
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return 0, nil
		}
		return 0, err
	}
	v, err := val.ValueCopy(nil)
	if err != nil {
		return 0, err
	}
	i, err := strconv.Atoi(string(v))
	return i, err
}

func MustGetIntKeyTX(txn *badger.Txn, key string) int {
	val, _ := GetIntKeyTX(txn, key)
	return val
}

func GetIntKeyTX(txn *badger.Txn, key string) (int, error) {
	val, err := txn.Get([]byte(key))
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return 0, nil
		}
		return 0, err
	}
	v, err := val.ValueCopy(nil)
	if err != nil {
		return 0, err
	}
	i, err := strconv.Atoi(string(v))
	return i, err
}

func IncrementKeyTX(txn *badger.Txn, key string, val int) error {
	if val == 0 {
		val = 1
	}
	item, err := txn.Get([]byte(key))
	if err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
		return err
	}
	if errors.Is(err, badger.ErrKeyNotFound) {
		return txn.Set([]byte(key), []byte(strconv.Itoa(val)))
	}
	v, err := item.ValueCopy(nil)
	if err != nil {
		return err
	}
	i, err := strconv.Atoi(string(v))
	if err != nil {
		return err
	}
	return txn.Set([]byte(key), []byte(strconv.Itoa(i+val)))
}

func SetIfNotExistsTX(txn *badger.Txn, key, value string) error {
	_, err := txn.Get([]byte(key))
	if err == nil {
		return nil
	}
	return txn.Set([]byte(key), []byte(value))
}
