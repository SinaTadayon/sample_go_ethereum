package main

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/manifoldco/promptui"
	log "github.com/sirupsen/logrus"
	"os"
	"sampleGoEthereum/src/app"
	"sampleGoEthereum/src/bsc_node"
	"sampleGoEthereum/src/bsc_scan"
	"sampleGoEthereum/src/configs"
	"strconv"
	"strings"
)

func init() {
	// Log as JSON instead of the default ASCII formatter.
	//log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	log.SetLevel(log.DebugLevel)
}

func main() {

	app.Globals.Config = configs.LoadConfig(".env")

	if app.Globals.Config.ContractAddress == "" {
		log.Fatal("CONTRACT_ADDRESS env is empty")
	}

	if app.Globals.Config.ContractCreationTxHash == "" {
		log.Fatal("CONTRACT_CREATION_TX_HASH env is empty")
	}

	if app.Globals.Config.BscNodeAddresses == "" {
		log.Fatal("BSC_NODE_ADDRESSES env is empty")
	}

	var contractAddress = common.HexToAddress(app.Globals.Config.ContractAddress)
	var contractCreationTxHash = common.HexToHash(app.Globals.Config.ContractCreationTxHash)
	var bscApiKey = app.Globals.Config.BscApiKey

	bscNodeAddresses := make([]string, 0, 16)
	addressList := strings.Split(app.Globals.Config.BscNodeAddresses, ",")

	bscNodeAddresses = append(bscNodeAddresses, addressList...)
	for _, address := range addressList {
		for counter := 2; counter <= 4; counter++ {
			bscNodeAddresses = append(bscNodeAddresses, strings.Replace(address, "1", strconv.Itoa(counter), -1))
		}
	}

	nodeConnFactory := bsc_node.NodeConnectionFactory(bscNodeAddresses)
	app.Globals.ConnectionPool = bsc_node.CreateConnectionPool(context.Background(), nodeConnFactory)

	prompt := promptui.Select{
		Label: "Select Get Method",
		Items: []string{"Get TX With Go-Ethereum", "Get Tx With BSCScan API"},
	}
	_, result, err := prompt.Run()

	if err != nil {
		log.Fatal("Prompt failed %v\n", err)
	}

	if result == "Get TX With Go-Ethereum" {
		bsc_node.GetTransactionListWithGoEthereum(contractAddress, contractCreationTxHash)
	} else {
		bsc_scan.GetTxListFromBscScan(contractAddress, bscApiKey)
	}
}
