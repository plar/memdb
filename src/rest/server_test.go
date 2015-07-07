package rest

import (
	"encoding/json"
	"memdb"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCreateRestServer(t *testing.T) {
	server := NewRestServer()
	assert.NotNil(t, server)
}

func TestRestServerPutKeyValue(t *testing.T) {

	server := NewRestServer()

	recorder := httptest.NewRecorder()

	req, err := http.NewRequest("PUT", "http://memdb.devel/values/key0", strings.NewReader("body"))
	assert.Nil(t, err)

	server.Router().ServeHTTP(recorder, req)

	jsonResponse := &LockResponse{}

	err = json.Unmarshal(recorder.Body.Bytes(), &jsonResponse)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "1", jsonResponse.LockId)
}

func TestRestServerGetKeyLock(t *testing.T) {

	server := NewRestServer()

	recorder := httptest.NewRecorder()

	// put new value
	req, err := http.NewRequest("PUT", "http://memdb.devel/values/key0", strings.NewReader("body"))
	assert.Nil(t, err)
	server.Router().ServeHTTP(recorder, req)

	jsonResponse := &LockResponse{}
	err = json.Unmarshal(recorder.Body.Bytes(), &jsonResponse)

	go func() {
		// simulate job
		time.Sleep(time.Duration(1000) * time.Millisecond)
		server.mdb.Release(memdb.LockID(jsonResponse.LockId))
	}()

	// put other value
	req2, err2 := http.NewRequest("POST", "http://rest.devel/reservations/key0", strings.NewReader(""))
	assert.Nil(t, err2)

	recorder2 := httptest.NewRecorder()
	server.Router().ServeHTTP(recorder2, req2)
	assert.Equal(t, http.StatusOK, recorder2.Code)

	jsonLockValueResponse := &LockValueResponse{}

	// //fmt.Printf("Body: %s\n", recorder.Body.String())

	err3 := json.Unmarshal(recorder2.Body.Bytes(), &jsonLockValueResponse)
	assert.NoError(t, err3)

	assert.Equal(t, "2", jsonLockValueResponse.LockId)
	assert.Equal(t, "body", jsonLockValueResponse.Value)
}

func TestRestServerUpdate(t *testing.T) {
	server := NewRestServer()
	// log := log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	// server := NewRestServerWithLogger(log)

	// try to release unexists lock for unexists key
	rec0 := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "http://memdb.devel/values/key0/1?release=true", strings.NewReader(""))
	assert.Nil(t, err)
	server.Router().ServeHTTP(rec0, req)
	assert.Equal(t, http.StatusNotFound, rec0.Code)

	// put new value and get lock
	rec2 := httptest.NewRecorder()
	req2, err2 := http.NewRequest("PUT", "http://memdb.devel/values/key0", strings.NewReader("value"))
	assert.Nil(t, err2)
	server.Router().ServeHTTP(rec2, req2)
	jr2 := &LockResponse{}
	assert.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &jr2))
	assert.Equal(t, "1", jr2.LockId)
	v2, exists2 := server.mdb.DirectGet(memdb.Key("key0"))
	assert.True(t, exists2)
	assert.Equal(t, memdb.Value("value"), v2)

	// try to release unexists lock for exists key
	rec3 := httptest.NewRecorder()
	req3, err3 := http.NewRequest("POST", "http://memdb.devel/values/key0/2?release=true", strings.NewReader(""))
	assert.Nil(t, err3)
	server.Router().ServeHTTP(rec3, req3)
	assert.Equal(t, http.StatusUnauthorized, rec3.Code)

	// set new value for exists key and release lock
	rec4 := httptest.NewRecorder()
	req4, err4 := http.NewRequest("POST", "http://memdb.devel/values/key0/1?release=true", strings.NewReader("newValue"))
	assert.Nil(t, err4)
	server.Router().ServeHTTP(rec4, req4)
	assert.Equal(t, http.StatusNoContent, rec4.Code)
	v, exists := server.mdb.DirectGet(memdb.Key("key0"))
	assert.True(t, exists)
	assert.Equal(t, memdb.Value("newValue"), v)

	// check if lock was released...
	rec5 := httptest.NewRecorder()
	req5, err5 := http.NewRequest("POST", "http://memdb.devel/values/key0/1?release=false", strings.NewReader(""))
	assert.Nil(t, err5)
	server.Router().ServeHTTP(rec5, req5)
	assert.Equal(t, http.StatusUnauthorized, rec5.Code)

	// get new lock for exists key
	//server.router.HandleFunc("/reservations/{key}", http.HandlerFunc(server.GetAndLock)).Methods("POST")
	rec6 := httptest.NewRecorder()
	req6, err6 := http.NewRequest("POST", "http://memdb.devel/reservations/key0", strings.NewReader(""))
	assert.Nil(t, err6)
	server.Router().ServeHTTP(rec6, req6)
	assert.Equal(t, http.StatusOK, rec6.Code)
	jr6 := &LockValueResponse{}
	err6 = json.Unmarshal(rec6.Body.Bytes(), &jr6)
	assert.NoError(t, err6)
	assert.Equal(t, "2", jr6.LockId)
	assert.Equal(t, "newValue", jr6.Value)

	// update value but don't release lock
	rec7 := httptest.NewRecorder()
	req7, err7 := http.NewRequest("POST", "http://memdb.devel/values/key0/2?release=false", strings.NewReader("otherValue"))
	assert.Nil(t, err7)
	server.Router().ServeHTTP(rec7, req7)
	assert.Equal(t, http.StatusNoContent, rec7.Code)
	v7, exists7 := server.mdb.DirectGet(memdb.Key("key0"))
	assert.True(t, exists7)
	assert.Equal(t, memdb.Value("otherValue"), v7)

	// release lock
	rec8 := httptest.NewRecorder()
	req8, err8 := http.NewRequest("POST", "http://memdb.devel/values/key0/2?release=true", strings.NewReader("released"))
	assert.Nil(t, err8)
	server.Router().ServeHTTP(rec8, req8)
	assert.Equal(t, http.StatusNoContent, rec8.Code)
}
