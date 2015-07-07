package memdb

import (
	"errors"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

var (
	ErrLockIdNotFound = errors.New("LockID not found")
	ErrKeyNotFound    = errors.New("Key not found")
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type LockID string
type Key string
type Value string

type LockIDGenerator interface {
	Next() LockID
}

type lockIdSeqGenerator struct {
	currentId uint64
}

func (g *lockIdSeqGenerator) Next() LockID {
	g.currentId++
	return LockID(strconv.FormatUint(g.currentId, 10))
}

func NewLockIDSeqGenerator() LockIDGenerator {
	return &lockIdSeqGenerator{}
}

var EmptyValue = Value("")

type lock struct {
	sync.Mutex
	lockId LockID
}

type memDB struct {
	sync.RWMutex

	name       string
	lockIdGen  LockIDGenerator
	storage    map[Key]Value
	key2Lock   map[Key]*lock
	lockId2Key map[LockID]Key
}

func (mdb *memDB) Name() string {
	return mdb.name
}

func (mdb *memDB) getLockByKey(key Key) (*lock, bool) {
	mdb.RLock()
	defer mdb.RUnlock()
	v, exists := mdb.key2Lock[key]
	return v, exists
}

func (mdb *memDB) getKeyByLockID(lockId LockID) (Key, bool) {
	mdb.RLock()
	defer mdb.RUnlock()
	v, exists := mdb.lockId2Key[lockId]
	return v, exists
}

func (mdb *memDB) Put(key Key, value Value) LockID {
	keyLock, hasLock := mdb.getLockByKey(key)
	if hasLock {
		keyLock.Lock()
	}

	mdb.Lock()
	defer mdb.Unlock()

	lockId := mdb.lockIdGen.Next()
	mdb.lockId2Key[lockId] = key

	if hasLock {
		// update exists keyLock
		keyLock.lockId = lockId
	} else {
		// create a new keyLock for the key
		mdb.key2Lock[key] = &lock{lockId: lockId}
		mdb.key2Lock[key].Lock()
	}

	mdb.storage[key] = value

	return lockId

}

func (mdb *memDB) Get(lockId LockID, key Key) (Value, error) {
	mdb.RLock()
	defer mdb.RUnlock()

	lockKey, exists := mdb.getKeyByLockID(lockId)
	if !exists {
		return EmptyValue, ErrLockIdNotFound
	}

	if key != lockKey {
		return EmptyValue, ErrKeyNotFound
	}

	value, exists := mdb.storage[key]
	if !exists {
		// Dead code
		// TBD: Delete should be implemented
		return EmptyValue, ErrKeyNotFound
	}

	return value, nil
}

func (mdb *memDB) Update(lockId LockID, key Key, value Value, releaseLock bool) error {
	keyLock, exists := mdb.getLockByKey(key)
	if !exists {
		return ErrKeyNotFound
	}

	lockKey, exists := mdb.getKeyByLockID(lockId)
	if !exists || key != lockKey {
		return ErrLockIdNotFound
	}

	mdb.Lock()
	defer mdb.Unlock()

	if releaseLock {
		keyLock.Unlock()
		delete(mdb.lockId2Key, lockId)
	}

	mdb.storage[key] = value

	return nil
}

func (mdb *memDB) Release(lockId LockID) error {
	key, exists := mdb.getKeyByLockID(lockId)
	if !exists {
		return ErrLockIdNotFound
	}

	keyLock, exists := mdb.getLockByKey(key)
	if exists {
		keyLock.Unlock()
	}

	mdb.Lock()
	defer mdb.Unlock()

	delete(mdb.lockId2Key, lockId)
	return nil
}

func (mdb *memDB) GetAndLock(key Key) (LockID, Value, error) {
	keyLock, exists := mdb.getLockByKey(key)
	if !exists {
		return "", EmptyValue, ErrKeyNotFound
	}

	keyLock.Lock()

	mdb.Lock()
	defer mdb.Unlock()

	lockId := mdb.lockIdGen.Next()
	keyLock.lockId = lockId
	mdb.lockId2Key[lockId] = key
	return lockId, mdb.storage[key], nil
}

func (mdb *memDB) DirectGet(key Key) (Value, bool) {
	mdb.RLock()
	defer mdb.RUnlock()
	value, hasKey := mdb.storage[key]
	return value, hasKey
}

type MemDB interface {
	Name() string
	Put(key Key, value Value) LockID
	Get(lockId LockID, key Key) (Value, error)
	Update(lockId LockID, key Key, value Value, releaseLock bool) error
	Release(lockId LockID) error

	GetAndLock(key Key) (LockID, Value, error)

	// for tests
	DirectGet(key Key) (Value, bool)
}

func NewMemDB(name string, lockIdGen LockIDGenerator) MemDB {
	return &memDB{
		name:       name,
		lockIdGen:  lockIdGen,
		storage:    make(map[Key]Value),
		key2Lock:   make(map[Key]*lock),
		lockId2Key: make(map[LockID]Key)}
}
