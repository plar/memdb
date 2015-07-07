package memdb

import (
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLockIDSeqGenerator(t *testing.T) {
	seqGen := NewLockIDSeqGenerator()
	assert.Equal(t, LockID("1"), seqGen.Next())
	assert.Equal(t, LockID("2"), seqGen.Next())
	assert.Equal(t, LockID("3"), seqGen.Next())
}

func TestMemDBCreate(t *testing.T) {
	memDB := NewMemDB("TestDB", NewLockIDSeqGenerator())
	assert.NotNil(t, memDB)
	assert.Equal(t, "TestDB", memDB.Name())
}

func TestMemDBPut(t *testing.T) {
	memDB := NewMemDB("TestDB", NewLockIDSeqGenerator())
	lid := memDB.Put("key", "unused")
	assert.NotEmpty(t, lid)
	assert.Equal(t, LockID("1"), lid)
}

func TestMemDBGet(t *testing.T) {
	memDB := NewMemDB("TestDB", NewLockIDSeqGenerator())
	lid := memDB.Put("key", "value")
	value, err := memDB.Get(lid, "key")
	assert.Nil(t, err)
	assert.Equal(t, Value("value"), value)
}

func TestMemDBReleaseUnexistsLock(t *testing.T) {
	memDB := NewMemDB("TestDB", NewLockIDSeqGenerator())
	err := memDB.Release("WrongLockId")
	assert.Equal(t, ErrLockIdNotFound, err)
}

func TestMemDBPutMany(t *testing.T) {

	totalConcurrentPuts := 10

	wga := &sync.WaitGroup{}
	wga.Add(totalConcurrentPuts)

	memDB := NewMemDB("TestDB", NewLockIDSeqGenerator())

	for i := 0; i < totalConcurrentPuts; i++ {
		go func(i int, prefix string) {
			key := Key("key" + prefix)
			value := Value("value" + strconv.Itoa(i))
			lid := memDB.Put(key, value)

			go func(lid LockID, key Key, value Value) {
				//fmt.Printf("Simulate work %s...\n", lid)
				time.Sleep(time.Duration(rand.Int31n(300)) * time.Millisecond)

				dbv, err := memDB.Get(lid, key)
				assert.Nil(t, err)
				assert.Equal(t, value, dbv)

				//fmt.Printf("Try to release %s...\n", lid)
				memDB.Release(lid)
				wga.Done()
			}(lid, key, value)
		}(i, strconv.Itoa(i))
	}

	wga.Wait()
}

func TestMemDBPutManySameKey(t *testing.T) {

	totalConcurrentPuts := 10

	wga := &sync.WaitGroup{}
	wga.Add(totalConcurrentPuts)

	memDB := NewMemDB("TestDB", NewLockIDSeqGenerator())

	for i := 0; i < totalConcurrentPuts; i++ {
		go func(i int, prefix string) {
			key := Key("key" + prefix)
			value := Value("value" + strconv.Itoa(i))
			lid := memDB.Put(key, value)

			go func(lid LockID, key Key, value Value) {
				time.Sleep(time.Duration(rand.Int31n(300)) * time.Millisecond)

				dbv, err := memDB.Get(lid, key)
				assert.Nil(t, err)
				assert.Equal(t, value, dbv)

				memDB.Release(lid)
				wga.Done()
			}(lid, key, value)
		}(i, "0")
	}

	wga.Wait()
}

func TestMemDBPutManySameKeyWithRandomRelease(t *testing.T) {

	totalConcurrentPuts := 10

	wga := &sync.WaitGroup{}
	wga.Add(totalConcurrentPuts)

	memDB := NewMemDB("TestDB", NewLockIDSeqGenerator())

	lst := rand.Perm(totalConcurrentPuts)
	for _, i := range lst {
		go func(i int, prefix string) {
			key := Key("key" + prefix)
			value := Value("value" + strconv.Itoa(i))
			lid := memDB.Put(key, value)

			go func(lid LockID, key Key, value Value) {
				//fmt.Printf("Simulate work %s...\n", lid)
				time.Sleep(time.Duration(rand.Int31n(300)) * time.Millisecond)

				dbv, err := memDB.Get(lid, key)
				assert.Nil(t, err)
				assert.Equal(t, value, dbv)

				//fmt.Printf("Try to release %s...\n", lid)
				memDB.Release(lid)
				wga.Done()
			}(lid, key, value)
		}(i, "0")
	}

	wga.Wait()
}

func TestMemDBPutDifferentKey(t *testing.T) {

	wga := &sync.WaitGroup{}
	wga.Add(1)

	memDB := NewMemDB("TestDB", NewLockIDSeqGenerator())

	lockId := memDB.Put(Key("key0"), Value("value0"))
	go func() {
		lockId2 := memDB.Put(Key("key0"), Value("value00"))

		go func() {
			memDB.Release(lockId2)
			wga.Done()
		}()
	}()

	go func() {
		memDB.Release(lockId)
	}()

	// no blocking here...
	memDB.Put(Key("key1"), Value("value1"))
	memDB.Put(Key("key2"), Value("value2"))
	memDB.Put(Key("key3"), Value("value3"))

	wga.Wait()

}

func TestGetKeyLock(t *testing.T) {
	wga := &sync.WaitGroup{}
	wga.Add(1)

	memDB := NewMemDB("TestDB", NewLockIDSeqGenerator())

	lockId := memDB.Put(Key("key0"), Value("value0"))

	go func() {
		k := Key("key0")
		lockId, value, err := memDB.GetAndLock(k)
		assert.Equal(t, LockID("2"), lockId)
		assert.Equal(t, Value("value0"), value)
		assert.Nil(t, err)

		wga.Done()
	}()

	go func() {
		time.Sleep(time.Duration(100+rand.Int31n(300)) * time.Millisecond)
		memDB.Release(lockId)
	}()

	_, _, err := memDB.GetAndLock("WrongKey")
	assert.Equal(t, ErrKeyNotFound, err)

	wga.Wait()
}

func TestUpdate(t *testing.T) {
	wga := &sync.WaitGroup{}
	wga.Add(1)

	memDB := NewMemDB("TestDB", NewLockIDSeqGenerator())

	k := Key("key0")
	lockId := memDB.Put(Key("key0"), Value("value0"))

	go func() {
		lockId, value, _ := memDB.GetAndLock(k)
		assert.Equal(t, LockID("2"), lockId)
		assert.Equal(t, Value("value0"), value)

		memDB.Update(lockId, k, Value("value1"), false)
		v, err := memDB.Get(lockId, k)
		assert.Nil(t, err)
		assert.Equal(t, Value("value1"), v)

		err = memDB.Update(lockId, Key("wrongkey"), Value("value2"), true)
		assert.Equal(t, ErrKeyNotFound, err)

		memDB.Update(lockId, k, Value("value2"), true)
		_, err2 := memDB.Get(lockId, k)
		assert.Equal(t, ErrLockIdNotFound, err2)

		wga.Done()
	}()

	go func() {
		time.Sleep(time.Duration(100+rand.Int31n(300)) * time.Millisecond)
		memDB.Release(lockId)
	}()

	wga.Wait()

}

func TestUpdateWithOldLockId(t *testing.T) {

	memDB := NewMemDB("TestDB", NewLockIDSeqGenerator())

	k := Key("key0")
	lockId0 := memDB.Put(Key("key0"), Value("value0"))
	assert.Equal(t, LockID("1"), lockId0)
	memDB.Release(lockId0)

	lockId1 := memDB.Put(Key("key0"), Value("value1"))
	assert.Equal(t, LockID("2"), lockId1)

	err := memDB.Update(lockId0, k, Value("doesntmatter"), true)
	assert.Equal(t, ErrLockIdNotFound, err)

	_, err2 := memDB.Get(lockId1, Key("unexistskey"))
	assert.Equal(t, ErrKeyNotFound, err2)
}

func TestWebCase(t *testing.T) {
	memDB := NewMemDB("TestDB", NewLockIDSeqGenerator())

	lockId := memDB.Put(Key("key"), Value("value"))
	assert.Equal(t, LockID("1"), lockId)

	go func() {
		time.Sleep(time.Duration(1000) * time.Millisecond)
		memDB.Release(lockId)
	}()

	lockId2, value2, err2 := memDB.GetAndLock(Key("key"))
	assert.NoError(t, err2)
	assert.Equal(t, LockID("2"), lockId2)
	assert.Equal(t, Value("value"), value2)

}
