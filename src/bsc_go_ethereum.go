package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	"github.com/manifoldco/promptui"
	"log"
	"math/big"
	"os"
)

type Transaction struct {
	Hash              string 		`json:"hash"`
	BlockNumber       string 		`json:"blockNumber"`
	TimeStamp         uint64 		`json:"timeStamp"`
	From              string 		`json:"from"`
	To                string 		`json:"to"`
	Value             string 		`json:"value"`
	TxreceiptStatus   uint64 		`json:"txreceipt_status"`
	TxFee 			  string  		`json:"txFee"`
}

func main() {

	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Some error occured. Err: %s", err)
	}

	var contractAddress = common.HexToAddress(os.Getenv("CONTRACT_ADDRESS"))
	var contractCreationTxHash = common.HexToHash(os.Getenv("CONTRACT_CREATION_TX_HASH"))
	var bscNodeAddress = os.Getenv("BSC_NODE_ADDRESS")
	var bscApiKey = os.Getenv("BSC_API_KEY")

	prompt := promptui.Select{
		Label: "Select Get Method",
		Items: []string{"Get TX With Go-Ethereum", "Get Tx With BSCScan API"},
	}
	_, result, err := prompt.Run()

	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

	if result == "Get TX With Go-Ethereum" {
		getTransactionListWithGoEthereum(contractAddress, contractCreationTxHash, bscNodeAddress)
	} else {
		getTxListFromBscScan(contractAddress, bscApiKey)
	}
}

func getTransactionListWithGoEthereum(contractAddress common.Address,
	contractCreationTxHash common.Hash, bscNodeAddress string) {

	client, err := ethclient.Dial(bscNodeAddress)
	if err != nil {
		log.Fatal("ethclient.Dial failed", err)
	}

	log.Printf("connecting to %s success . . . ", bscNodeAddress)

	receipt, err := client.TransactionReceipt(context.Background(), contractCreationTxHash)
	if err != nil {
		log.Fatal("client.TransactionReceipt: ", err)
	}
	log.Println("Contract Creation blockNumber: ", receipt.BlockNumber)

	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		log.Fatal("client.HeaderByNumber failed", err)
	}
	log.Println("latest block number: ", header.Number.String())

	var blockNumberList = findBlockNumbers(client, receipt.BlockNumber, header.Number, contractAddress)
	log.Printf("blocks found: %v\n", blockNumberList)
	var txList = getTransactions(client, blockNumberList, contractAddress)
	result, err := json.Marshal(txList)
	if err != nil {
		log.Fatal("json.marshal failed", err)
		return
	}

	log.Println("tx count: ", len(txList))
	log.Printf("%s",result)
}

func findBlockNumbers(client *ethclient.Client,
		creationContractBlockNumber *big.Int,
		latestBlockNumber *big.Int,
		contractAddress common.Address) []*big.Int {
	var blockNumberList = make([]*big.Int, 0, 1024)
	var fromBlock = creationContractBlockNumber
	var toBlock, _ = new(big.Int).SetString(creationContractBlockNumber.String(), 10)
	for toBlock.Cmp(latestBlockNumber) <= 0 {
		toBlock.Add(toBlock, big.NewInt(5000))
		query := ethereum.FilterQuery {
			FromBlock: fromBlock,
			ToBlock:   toBlock,
			Addresses: []common.Address{
				contractAddress,
			},
		}

		logs, err := client.FilterLogs(context.Background(), query)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Checking blocks from %s till %s . . .", fromBlock.String(), toBlock.String())

		for _, txLog := range logs {
			blockNumberList = append(blockNumberList, big.NewInt(int64(txLog.BlockNumber)))
		}

		fromBlock.Add(fromBlock, big.NewInt(5000))
	}

	return blockNumberList
}

func getTransactions(client *ethclient.Client,
	blockNumberList []*big.Int,
	contractAddress common.Address) []*Transaction {
	var txList = make([]*Transaction, 0, 1024)
	for _, blockNumber := range blockNumberList {
		block, err := client.BlockByNumber(context.Background(), blockNumber)
		if err != nil {
			log.Fatal("client.BlockByNumber failed", err)
		}

		for _, tx := range block.Transactions() {
			if tx.To() != nil && *tx.To() == contractAddress {
				chainID, err := client.NetworkID(context.Background())
				if err != nil {
					log.Fatal("client.NetworkID failed", err)
				}

				msg, err := tx.AsMessage(types.NewEIP155Signer(chainID), nil)
				if err != nil {
					log.Fatal("tx.AsMessage failed", err)
				}

				receipt, err := client.TransactionReceipt(context.Background(), tx.Hash())
				if err != nil {
					log.Fatal("client.TransactionReceipt failed", err)
				}

				var transaction = &Transaction{
					Hash: tx.Hash().String(),
					BlockNumber: block.Number().String(),
					TimeStamp: block.Time(),
					From: msg.From().String(),
					To: tx.To().String(),
					Value: tx.Value().String(),
					TxFee: tx.GasPrice().Mul(tx.GasPrice(), big.NewInt(int64(tx.Gas()))).String(),
					TxreceiptStatus: receipt.Status,
				}

				txList = append(txList, transaction)
			}
		}
	}

	return txList
}