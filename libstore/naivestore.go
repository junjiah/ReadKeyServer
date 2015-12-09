package libstore

import (
	"errors"
	"sync"
)

type naiveLibstore struct {
	storageLock *sync.RWMutex
	storage     map[string]interface{}
}

// TODO: For testing, use this dump libstore.
func NewLibstore() (*naiveLibstore, error) {
	return &naiveLibstore{
		storageLock: &sync.RWMutex{},
		storage:     make(map[string]interface{}),
	}, nil
}

func (nls *naiveLibstore) Get(key string) (string, error) {
	nls.storageLock.RLock()
	defer nls.storageLock.RUnlock()

	if v, ok := nls.storage[key]; ok {
		return v.(string), nil
	} else {
		return "", errors.New("failed to get value")
	}
}

func (nls *naiveLibstore) GetList(key string) ([]string, error) {
	nls.storageLock.RLock()
	defer nls.storageLock.RUnlock()

	if v, ok := nls.storage[key]; ok {
		return v.([]string), nil
	} else {
		return nil, errors.New("failed to get value")
	}
}

func (nls *naiveLibstore) Put(key, val string) error {
	nls.storageLock.Lock()
	defer nls.storageLock.Unlock()

	nls.storage[key] = val
	return nil
}

func (nls *naiveLibstore) AppendToList(key, newItem string) error {
	nls.storageLock.Lock()
	defer nls.storageLock.Unlock()

	if v, ok := nls.storage[key]; ok {
		ls, _ := v.([]string)
		nls.storage[key] = append(ls, newItem)
	} else {
		nls.storage[key] = []string{newItem}
	}
	return nil
}

func (nls *naiveLibstore) RemoveFromList(key, removeItem string) error {
	nls.storageLock.Lock()
	defer nls.storageLock.Unlock()

	if v, ok := nls.storage[key]; ok {
		ls, _ := v.([]string)
		for i, item := range ls {
			if item == removeItem {
				nls.storage[key] = append(ls[:i], ls[i+1:]...)
				break
			}
		}
	}
	return nil
}
