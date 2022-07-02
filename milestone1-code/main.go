package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

func main() {
	dataFilePath := os.Getenv("DATA_FILE_PATH")

	if 0 == len(dataFilePath) {
		panic("Missing env value DATA_FILE_PATH")
	}

	if _, err := os.Stat(dataFilePath); errors.Is(err, os.ErrNotExist) {
		file, err := os.Create(dataFilePath)
		if nil != err {
			panic("Failed to create secret file")
		}

		_ = file.Close()
	} else {
		file, err := os.Open(dataFilePath)
		if nil != err {
			panic("Failed to open secret file")
		}

		_ = file.Close()
	}

	controller := NewController(dataFilePath)
	controller.Listen(":3000")
}

type Controller struct {
	serveMux            *http.ServeMux
	mutex               *sync.Mutex
	persistenceFilePath string
	items               map[string]string
}

func NewController(
	filePath string,
) Controller {
	var initItems map[string]string
	fileContent, err := ioutil.ReadFile(filePath)
	if nil == err && 0 < len(fileContent) {
		if err = json.Unmarshal(fileContent, &initItems); nil != err {
			fmt.Println("Failed to read ", filePath, err)
		}
	}

	if nil == initItems {
		initItems = make(map[string]string)
	}

	mux := http.NewServeMux()
	sc := Controller{
		serveMux:            mux,
		mutex:               &sync.Mutex{},
		persistenceFilePath: filePath,
		items:               initItems,
	}

	mux.HandleFunc("/healthcheck", sc.handleHealthCheck)
	mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			sc.handleGetSecret(res, req)
		} else if http.MethodPost == req.Method {
			sc.handlePostSecret(res, req)
		} else {
			http.Error(res, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	return sc
}

func (sc Controller) Listen(addr string) {
	err := http.ListenAndServe(addr, sc.serveMux)
	if nil != err {
		panic("Failed to start server")
	}
}

func (sc *Controller) handleHealthCheck(
	res http.ResponseWriter,
	req *http.Request,
) {
	if http.MethodGet != req.Method {
		http.Error(res, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if _, err := res.Write([]byte("ok")); err != nil {
		http.Error(res, "Failed to write response", http.StatusInternalServerError)
		return
	}
}

func (sc *Controller) handleGetSecret(
	res http.ResponseWriter,
	req *http.Request,
) {
	responseCode := http.StatusOK
	secret := ""
	id := strings.TrimPrefix(req.URL.Path, "/")
	if 0 < len(id) {
		sc.mutex.Lock()
		defer sc.mutex.Unlock()

		if item, ok := sc.items[id]; ok {
			secret = item
			delete(sc.items, id)
		} else {
			responseCode = http.StatusNotFound
		}
	} else {
		responseCode = http.StatusBadRequest
	}

	responseData := make(map[string]string)
	responseData["data"] = secret

	responseJson, err := json.Marshal(responseData)
	if nil != err {
		http.Error(res, "Failed to create response", http.StatusInternalServerError)
		return
	}

	res.Header().Set("Content-Length", strconv.Itoa(len(responseJson)))
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(responseCode)

	if _, err := res.Write(responseJson); err != nil {
		http.Error(res, "Failed to write response", http.StatusInternalServerError)
		return
	}
}

func (sc *Controller) handlePostSecret(
	res http.ResponseWriter,
	req *http.Request,
) {
	bodyData, err := ioutil.ReadAll(req.Body)
	if nil != err {
		http.Error(res, "Failed to read data", http.StatusInternalServerError)
		return
	}

	var bodyMap map[string]string
	if err := json.Unmarshal(bodyData, &bodyMap); err != nil {
		http.Error(res, "Failed to parse body", http.StatusInternalServerError)
		return
	}

	secret := bodyMap["plain_text"]
	hash := md5.Sum([]byte(secret))
	secretHash := hex.EncodeToString(hash[:])

	sc.mutex.Lock()
	defer sc.mutex.Unlock()
	sc.items[secretHash] = secret

	if err := sc.persistItems(); nil != err {
		_ = fmt.Errorf("Failed to write persitence file to %s", sc.persistenceFilePath)
	}

	responseData := make(map[string]string)
	responseData["id"] = secretHash

	responseJson, err := json.Marshal(responseData)
	if nil != err {
		http.Error(res, "Failed to create response", http.StatusInternalServerError)
		return
	}

	responseHeader := res.Header()
	responseHeader.Add("Content-Length", strconv.Itoa(len(responseJson)))
	responseHeader.Add("Content-Type", "application/json")

	if _, err := res.Write(responseJson); err != nil {
		http.Error(res, "Failed to write response", http.StatusInternalServerError)
		return
	}
}

func (sc *Controller) persistItems() error {
	itemsJson, err := json.MarshalIndent(sc.items, "", " ")
	if nil != err {
		return err
	}

	return ioutil.WriteFile(sc.persistenceFilePath, itemsJson, 0644)
}
