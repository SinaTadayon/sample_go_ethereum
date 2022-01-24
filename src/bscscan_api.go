package main

import (
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
)

type ResponseApi struct {
	Status 		string      `json:"status"`
	Message 	string      `json:"message"`
	Result 		[]*TxDetails `json:"result"`
}

type TxDetails struct {
	BlockNumber       string `json:"blockNumber"`
	TimeStamp         string `json:"timeStamp"`
	Hash              string `json:"hash"`
	Nonce             string `json:"nonce"`
	BlockHash         string `json:"blockHash"`
	TransactionIndex  string `json:"transactionIndex"`
	From              string `json:"from"`
	To                string `json:"to"`
	Value             string `json:"value"`
	Gas               string `json:"gas"`
	GasPrice          string `json:"gasPrice"`
	IsError           string `json:"isError"`
	TxreceiptStatus   string `json:"txreceipt_status"`
	Input             string `json:"input"`
	ContractAddress   string `json:"contractAddress"`
	CumulativeGasUsed string `json:"cumulativeGasUsed"`
	GasUsed           string `json:"gasUsed"`
	Confirmations     string `json:"confirmations"`
}

func getTxListFromBscScan(contractAddress common.Address, apiKey string) {

	var txDetailList = make([]*TxDetails, 0, 1024)
	var offset = 25
	var page = 1
	var sort = "asc"
	var module = "account"
	var action = "txlist"
	var baseUrl = "https://api.bscscan.com/api"

	for {
		v := url.Values{}
		v.Set("module", module)
		v.Set("action", action)
		v.Set("address", contractAddress.String())
		v.Set("startblock", "1")
		v.Set("endblock", "20000000")
		v.Set("page", strconv.Itoa(page))
		v.Set("offset", strconv.Itoa(offset))
		v.Set("sort", sort)
		v.Set("apikey", apiKey)

		bscUrl := baseUrl + "?" + v.Encode()
		res, err := http.Get(bscUrl)

		// check for response error
		if err != nil {
			log.Fatal("http get failed", err)
		}

		// read all response body
		data, _ := ioutil.ReadAll(res.Body)

		// close response body
		err = res.Body.Close()
		if err != nil {
			log.Fatal("Body.close failed", err)
		}

		var txResponseApi ResponseApi
		err = json.Unmarshal(data, &txResponseApi)
		if err != nil {
			log.Fatal("json.Unmarshal failed", err)
		}

		if txResponseApi.Status == "0" {
			break
		}

		txDetailList = append(txDetailList, txResponseApi.Result...)
		page++
	}

	var result []byte
	result, err := json.Marshal(txDetailList)
	if err != nil {
		log.Fatal("json.marshal failed", err)
	}

	log.Println("tx count: ", len(txDetailList))
	log.Printf("%s\n", result)
}