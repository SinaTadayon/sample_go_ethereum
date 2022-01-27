package bsc_node

import (
	"context"
	"encoding/json"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
	"math/big"
	"sampleGoEthereum/src/app"
	"sync"
	"time"
)

type NodeClient struct {
	url  		string
	client      *ethclient.Client
}

type Pipeline struct {
	contractAddress        common.Address
	contractCreationTxHash common.Hash
	chainID                *big.Int
	node                   *NodeClient
	blockRangeStream       BlockRangeStream
}

type TxData struct {
	block 		*types.Block
	transaction *types.Transaction
	message 	*types.Message
	txReceipt   *types.Receipt
	txDto		*TxDto
}

type TxDto struct {
	Hash              string 		`json:"hash"`
	BlockNumber       string 		`json:"blockNumber"`
	TimeStamp         uint64 		`json:"timeStamp"`
	From              string 		`json:"from"`
	To                string 		`json:"to"`
	Value             string 		`json:"value"`
	TxStatus   		  uint64 		`json:"status"`
	TxFee 			  string  		`json:"txFee"`
}

type PipelineInStream <-chan *Pipeline
type TxDataReaderStream <-chan *TxData
type TxDtoReaderStream <-chan TxDto

type BlockRangeStream chan *big.Int
type BlockRangeReaderStream <-chan *big.Int
type BlockNumberReaderStream <-chan *big.Int

func GetTransactionListWithGoEthereum(contractAddress common.Address,
	contractCreationTxHash common.Hash) {
	txDtoList := make([]TxDto, 0, 1024)

	ctx, _ := context.WithCancel(context.Background())

	obj, err := app.Globals.ConnectionPool.BorrowObject(ctx)
	if err != nil {
		log.Fatal("connectionPool.BorrowObject failed", err)
	}
	node := obj.(*NodeClient)
	chainID, err := node.client.NetworkID(ctx)
	if err != nil {
		log.Fatal("bsc_node.NetworkID failed", err)
	}
	if err = app.Globals.ConnectionPool.ReturnObject(ctx, node); err != nil {
		log.Fatal("connectionPool.ReturnObject failed", err)
	}

	blockRangeStream := blockNumberRangeGenerator(ctx, contractCreationTxHash)
	pipelineStream := fanOutPipelines(ctx, contractAddress, contractCreationTxHash, chainID, blockRangeStream)
	txDtoStream := fanInPipelines(ctx, pipelineStream)

	for txDtoStream != nil {
		select {
		case <-ctx.Done(): txDtoStream = nil; break
		case txDto, ok := <- txDtoStream:
			if ok {
				txDtoList = append(txDtoList, txDto)
			} else {
				txDtoStream = nil
			}
		}
	}

	result, err := json.Marshal(txDtoList)
	if err != nil {
		log.Fatal("json.marshal failed", err)
		return
	}

	log.Debug("tx count: ", len(txDtoList))
	log.Debugf("%s",result)
}

func blockNumberRangeGenerator(ctx context.Context,
	contractCreationTxHash common.Hash) BlockRangeReaderStream {

	blockRangeStream := make(chan *big.Int)
	generatorTask := func() {
		var contractBlockNumber *big.Int
		var latestBlockNumber *big.Int
		var wg sync.WaitGroup

		defer close(blockRangeStream)

		wg.Add(1)
		workerFactory().run(func() {
			obj, err := app.Globals.ConnectionPool.BorrowObject(ctx)
			if err != nil {
				log.Fatal("connectionPool.BorrowObject failed", err)
			}

			node := obj.(*NodeClient)
			receipt, err := node.client.TransactionReceipt(ctx, contractCreationTxHash)
			if err != nil {
				log.Fatal("get bsc_node.TransactionReceipt failed", err)
			}

			contractBlockNumber = receipt.BlockNumber
			if err = app.Globals.ConnectionPool.ReturnObject(ctx, node); err != nil {
				log.Fatal("connectionPool.ReturnObject failed", err)
			}
			wg.Done()
		})

		wg.Add(1)
		workerFactory().run(func() {
			obj, err := app.Globals.ConnectionPool.BorrowObject(ctx)
			if err != nil {
				log.Fatal("connectionPool.BorrowObject failed", err)
			}

			node := obj.(*NodeClient)
			header, err := node.client.HeaderByNumber(ctx, nil)
			if err != nil {
				log.Fatal("bsc_node.HeaderByNumber failed", err)
			}

			latestBlockNumber = header.Number
			if err = app.Globals.ConnectionPool.ReturnObject(ctx, node); err != nil {
				log.Fatal("connectionPool.ReturnObject failed", err)
			}
			wg.Done()
		})

		wg.Wait()
		log.Debug("contract creation block number: ", contractBlockNumber)
		log.Debug("latest block number: ", latestBlockNumber)

		var fromBlock = contractBlockNumber
		for fromBlock.Cmp(latestBlockNumber) <= 0 {
			select {
			case <-ctx.Done(): break
			default:
			}
			fromBlock.Add(fromBlock, big.NewInt(5000))
			newBlock, _ := new(big.Int).SetString(fromBlock.String(), 10)
			blockRangeStream <- newBlock
		}
	}

	workerFactory().run(generatorTask)
	return blockRangeStream
}

// TODO cleanup pipeline
func fanOutPipelines(ctx context.Context,
					contractAddress common.Address,
					contractCreationTxHash common.Hash,
					chainID *big.Int,
					blockRangeStream BlockRangeReaderStream) PipelineInStream {
	pipelines := make([]*Pipeline, 0, app.Globals.ConnectionPool.Config.MaxTotal)
	pipelineStream := make(chan *Pipeline, app.Globals.ConnectionPool.Config.MaxTotal)

	fanOutTask := func() {
		defer func() {
			for _, pipeline := range pipelines {
				close(pipeline.blockRangeStream)
			}

			close(pipelineStream)
		}()

		index := 0
		for fromBlock := range blockRangeStream {
			select {
			case <-ctx.Done(): return
			default:
			}

			if len(pipelines) < cap(pipelines) {
				obj, err := app.Globals.ConnectionPool.BorrowObject(ctx)
				if err != nil {
					log.Error("connectionPool.BorrowObject failed", err)
					continue
				}

				newPipeline := &Pipeline{
					contractAddress:        contractAddress,
					contractCreationTxHash: contractCreationTxHash,
					node:                   obj.(*NodeClient),
					chainID: 				chainID,
					blockRangeStream:       make(chan *big.Int, 128),
				}
				pipelines = append(pipelines, newPipeline)
				pipelineStream <- newPipeline
			}

			if index >= len(pipelines) {
				index = 0
			}

			pipelines[index].blockRangeStream <- fromBlock
			index++
		}
	}

	workerFactory().run(fanOutTask)
	return pipelineStream
}

func fanInPipelines(ctx context.Context, pipelineStream PipelineInStream) TxDtoReaderStream {
	multiplexedTxDtoStream := make(chan TxDto)

	var wg sync.WaitGroup
	fanInTask := func() {
		defer close(multiplexedTxDtoStream)
		for pipeline := range pipelineStream {
			select {
			case <-ctx.Done(): break
			default:
			}

			txDtoStream, pipelineTask := pipeline.buildPipeline(ctx)
			workerFactory().run(pipelineTask)

			fanInMultiplexTask := func() {
				defer wg.Done()
				for txDto := range txDtoStream {
					select {
					case <-ctx.Done(): break
					case multiplexedTxDtoStream <- txDto:
					}
				}
			}

			wg.Add(1)
			workerFactory().run(fanInMultiplexTask)
		}

		wg.Wait()
	}

	workerFactory().run(fanInTask)
	return multiplexedTxDtoStream
}

func (pipeline Pipeline) findBlockStage(ctx context.Context) (BlockNumberReaderStream, Task) {

	blockNumberStream := make(chan *big.Int)
	findBlockTask := func() {
		defer close(blockNumberStream)
		for fromBlock := range pipeline.blockRangeStream {
			select {
			case <-ctx.Done(): return
			default:
			}

			var toBlock, _ = new(big.Int).SetString(fromBlock.String(), 10)
			query := ethereum.FilterQuery{
				FromBlock: fromBlock,
				ToBlock:   toBlock.Add(toBlock, big.NewInt(5000)),
				Addresses: []common.Address{
					pipeline.contractAddress,
				},
			}

			var retry = 0
			for {
				select {
				case <-ctx.Done(): return
				default:
				}

				logs, err := pipeline.node.client.FilterLogs(ctx, query)
				if err != nil {
					if retry >= 5 {
						log.Errorf("bsc_node.FilterLogs failed, checking block from %s to %s, node: %s, err: %s",
							fromBlock.String(), toBlock.String(), pipeline.node.url, err)
						break
					}

					log.Infof("retry %d in checking block from %s to %s, node: %s",
						retry, fromBlock.String(), toBlock.String(), pipeline.node.url)
					time.Sleep(1 * time.Second)
					retry++
					continue
				}
				log.Debugf("node: %s, Checking blocks from %s to %s . . .",
					pipeline.node.url, fromBlock.String(), toBlock.String())
				for _, txLog := range logs {
					blockNumberStream <- big.NewInt(int64(txLog.BlockNumber))
				}
				break
			}
		}
	}

	return blockNumberStream, findBlockTask
}

func (pipeline Pipeline) fetchBlockStage(ctx context.Context,
			blockNumberStream BlockNumberReaderStream) (TxDataReaderStream, Task) {
	txDataStream := make(chan *TxData)
	fetchBlockTask := func() {
		defer close(txDataStream)
		for blockNumber := range blockNumberStream {
			select {
			case <-ctx.Done():
				return
			default:
			}

			var retry = 0
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				block, err := pipeline.node.client.BlockByNumber(ctx, blockNumber)
				if err != nil {
					if retry >= 5 {
						log.Errorf("bsc_node.BlockByNumber failed, fetching block %s, node: %s, err: %s",
							blockNumber.String(), pipeline.node.url, err)
						break
					}

					log.Infof("retry %d in fetching block %s, node: %s",
						retry, blockNumber.String(), pipeline.node.url)
					time.Sleep(1 * time.Second)
					retry++
					continue
				}
				log.Debugf("node: %s, fetch block #%s success",
					pipeline.node.url, block.Number().String())
				txData := &TxData{
					block:       block,
					transaction: nil,
					message:     nil,
					txReceipt:   nil,
					txDto:       nil,
				}
				txDataStream <- txData
				break
			}
		}
	}

	return txDataStream, fetchBlockTask
}

func (pipeline Pipeline) fetchTxStage(ctx context.Context,
	txDataInputStream TxDataReaderStream) (TxDataReaderStream, Task) {
	txDataStream := make(chan *TxData)

	fetchTxTask := func() {
		defer close(txDataStream)
		for txData := range txDataInputStream {
			select {
			case <-ctx.Done():
				return
			default:
			}

			for _, tx := range txData.block.Transactions() {
				if tx.To() != nil && *tx.To() == pipeline.contractAddress {
					msg, err := tx.AsMessage(types.NewEIP155Signer(pipeline.chainID), nil)
					if err != nil {
						log.Errorf("tx.AsMessage failed, txHash: %s, err: %s", tx.Hash(), err)
						continue
					}

					var retry = 0
					for {
						select {
						case <-ctx.Done():
							return
						default:
						}

						receipt, err := pipeline.node.client.TransactionReceipt(ctx, tx.Hash())
						if err != nil {
							if retry >= 5 {
								log.Errorf("bsc_node.TransactionReceipt failed, receipt tx hash: %s, node: %s, err: %s",
									tx.Hash(), pipeline.node.url, err)
								break
							}

							log.Infof("retry %d in get receipt tx hash %s, node: %s",
								retry, tx.Hash(), pipeline.node.url)
							time.Sleep(1 * time.Second)
							retry++
							continue
						}

						log.Debugf("node: %s, fetch receipt #%s success",
							pipeline.node.url, receipt.TxHash)
						txData.transaction = tx
						txData.message = &msg
						txData.txReceipt = receipt
						txData.txDto = &TxDto{
							Hash:            tx.Hash().String(),
							BlockNumber:     txData.block.Number().String(),
							TimeStamp:       txData.block.Time(),
							From:            msg.From().String(),
							To:              tx.To().String(),
							Value:           tx.Value().String(),
							TxFee:           tx.GasPrice().Mul(tx.GasPrice(), big.NewInt(int64(tx.Gas()))).String(),
							TxStatus: 		 receipt.Status,
						}

						txDataStream <- txData
						break
					}
				}
			}
		}
	}

	return txDataStream, fetchTxTask
}

func (pipeline Pipeline) buildPipeline(ctx context.Context)  (TxDtoReaderStream, Task) {
	txDtoOutStream := make(chan TxDto)

	pipelineTask := func() {
		defer close(txDtoOutStream)
		blockNumberStream, findBlockTask := pipeline.findBlockStage(ctx)
		txDataSteam, fetchBlockTask := pipeline.fetchBlockStage(ctx, blockNumberStream)
		txOutStream, fetchTxTask := pipeline.fetchTxStage(ctx, txDataSteam)

		workerFactory().run(findBlockTask)
		workerFactory().run(fetchBlockTask)
		workerFactory().run(fetchTxTask)

		for txData := range txOutStream {
			select {
			case <-ctx.Done():
				return
			case txDtoOutStream <- *txData.txDto:
			}
		}
	}

	return txDtoOutStream, pipelineTask
}