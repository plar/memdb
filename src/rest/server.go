package rest

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"memdb"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

var NoLog = log.New(ioutil.Discard, "", log.Ldate|log.Ltime|log.Lshortfile)

type LockResponse struct {
	LockId string `json:"lock_id"`
}

type LockValueResponse struct {
	LockId string `json:"lock_id"`
	Value  string `json:"value"`
}

type Server struct {
	mdb    memdb.MemDB
	router *mux.Router

	logger *log.Logger
}

func (s *Server) Router() *mux.Router {
	return s.router
}

func (s *Server) Run() {
	s.logger.Printf("Welcome to MemDB!")
	s.logger.Printf("Listen on 127.0.0.1:8080...")
	http.ListenAndServe("127.0.0.1:8080", s.router)
}

//
// POST /reservations/{key}
//
// Wait for {key} to be available (ignore cases where the client times out), then acquire a lock on it (and its value).
//
func (s *Server) GetAndLock(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	rawKey, exists := vars["key"]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	s.logger.Printf("key: %v, %v", rawKey, s.mdb)

	key := memdb.Key(rawKey)
	lockId, value, err := s.mdb.GetAndLock(key)
	s.logger.Printf("lockId: %v, Value: %v, Err: %v", lockId, value, err)
	if err == memdb.ErrKeyNotFound {
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	jsonResponse := &LockValueResponse{LockId: string(lockId), Value: string(value)}
	s.logger.Printf("jsonResponse: %v", jsonResponse)

	body, err := json.Marshal(jsonResponse)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

//
// PUT /values/{key}
//
// If {key} already exists, wait until it's available (ignore cases where the client times out) then acquire the lock on it.
// If it doesn't already exist, create it and immediately acquire the lock on it (that operation should never block).
//
func (s *Server) PutAndLock(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	// handle key var
	rawKey, exists := vars["key"]
	if !exists {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	key := memdb.Key(rawKey)

	// handle POST body
	rawValue, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	value := memdb.Value(rawValue)

	// store value into the memdb
	lockid := s.mdb.Put(key, value)

	// create response
	jsonResponse := &LockResponse{LockId: string(lockid)}
	body, err := json.Marshal(&jsonResponse)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

//
// POST /values/{key}/{lock_id}?release={true, false}
//
// Attempt to update the value of {key} to the value given in the POST body according to the following rules:
//
// If {key} doesn't exist, return 404 Not Found
// If {key} exists but {lock_id} doesn't identify the currently held lock, do no action and respond immediately with 401 Unauthorized.
// If {key} exists, {lock_id} identifies the currently held lock and release=true, set the new value, release the lock and invalidate {lock_id}. Return 204 No Content
// If {key} exists, {lock_id} identifies the currently held lock and release=false, set the new value but don't release the lock and keep {lock_id} value. Return 204 No Content
//
func (s *Server) Update(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	// handle key var
	rawKey, exists := vars["key"]
	if !exists {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	key := memdb.Key(rawKey)

	// handle lock_id var
	rawLockId, exists := vars["lock_id"]
	if !exists {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	lockId := memdb.LockID(rawLockId)

	// handle release query param
	release, err := strconv.ParseBool(vars["release"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// read POST Body
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// try to update...
	err = s.mdb.Update(lockId, key, memdb.Value(string(body)), release)
	if err == memdb.ErrKeyNotFound {
		w.WriteHeader(http.StatusNotFound)
		return

	} else if err == memdb.ErrLockIdNotFound {
		w.WriteHeader(http.StatusUnauthorized)
		return

	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func NewRestServerWithLogger(logger *log.Logger) *Server {

	server := &Server{
		mdb:    memdb.NewMemDB("RestDB", memdb.NewLockIDSeqGenerator()),
		logger: logger,
	}

	server.router = mux.NewRouter()
	server.router.HandleFunc("/reservations/{key}", http.HandlerFunc(server.GetAndLock)).Methods("POST")
	server.router.HandleFunc("/values/{key}/{lock_id}", server.Update).Methods("POST").Queries("release", "{release}")
	server.router.HandleFunc("/values/{key}", server.PutAndLock).Methods("PUT")

	return server
}

func NewRestServer() *Server {
	return NewRestServerWithLogger(NoLog)
}
