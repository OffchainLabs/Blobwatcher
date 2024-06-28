package main

import (
	"context"
	"flag"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/misc/eip4844"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/gethclient"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	log "github.com/sirupsen/logrus"
)

var (
	// Required fields
	executionEndpoint = flag.String("execution-endpoint", "ws://localhost:8546", "Path to webscocket endpoint for execution client.")
	wsOrigin          = flag.String("origin-secret", "", "Origin string for websocket connection")
	metricsEndpoint   = flag.String("metrics-endpoint", "localhost:8080", "Path for our metrics server.")
)

func main() {
	flag.Parse()
	log.Info("Starting blob watcher service")
	log.Infof("Using websocket endpoint of %s", *executionEndpoint)
	srv := StartMetricsServer(*metricsEndpoint)
	defer func() {
		if err := srv.Close(); err != nil {
			log.Error(err)
		}
	}()

	client, err := rpc.DialWebsocket(context.Background(), *executionEndpoint, *wsOrigin)
	if err != nil {
		log.Fatal(err)
	}

	ec := ethclient.NewClient(client)
	gc := gethclient.New(client)

	txChan := make(chan *gethtypes.Transaction, 100)
	pSub, err := gc.SubscribeFullPendingTransactions(context.Background(), txChan)
	if err != nil {
		log.Fatal(err)
	}

	hdrChan := make(chan *gethtypes.Header, 100)
	hSub, err := ec.SubscribeNewHead(context.Background(), hdrChan)
	if err != nil {
		log.Fatal(err)
	}
	chainID, err := ec.ChainID(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	currBaseFee := new(big.Int)
	pendingTxs := make(map[common.Hash]*gethtypes.Transaction)
	txTime := make(map[common.Hash]time.Time)

	for {
		select {
		case err := <-pSub.Err():
			log.WithError(err).Error("Pending transaction subscription error")
			ec.Close()
			client.Close()
			close(txChan)
			close(hdrChan)
			hSub.Unsubscribe()
			return

		case <-hSub.Err():
			log.WithError(err).Error("New head subscription error")
			ec.Close()
			client.Close()
			close(txChan)
			close(hdrChan)
			pSub.Unsubscribe()
			return
		case tx := <-txChan:
			if tx.Type() == gethtypes.BlobTxType {
				tHash := tx.Hash()
				log.WithFields(txData(tx, chainID)).Infof("Received new Transaction from Gossip")
				recordTxMetrics(tx, chainID)
				pendingTxs[tHash] = tx
				txTime[tHash] = time.Now()
			}

		case h := <-hdrChan:
			if h.ExcessBlobGas != nil {
				currBaseFee = eip4844.CalcBlobFee(*h.ExcessBlobGas)
			}
			log.Infof("*/-------------------------------------------------------------------------------------------------------------------------------------------------------------------*/")
			log.WithFields(log.Fields{
				"blockHash":        h.Hash(),
				"blockNumber":      h.Number.Uint64(),
				"blockTime":        h.Time,
				"blobBaseFee(wei)": currBaseFee.Uint64(),
				"baseFee(Gwei)":    float64(h.BaseFee.Uint64()) / params.GWei,
				"builder":          strings.ToValidUTF8(string(h.Extra), ""),
			}).Infof("Received new block")
			blockNumberGauge.Set(float64(h.Number.Uint64()))
			executionBaseFeeGauge.Set(float64(h.BaseFee.Uint64()) / params.GWei)
			blobBaseFeeGauge.Set(float64(currBaseFee.Uint64()))

			currentPendingTxs := len(pendingTxs)
			blobsIncluded := 0
			viabletxs := 0
			viableBlobs := 0

			for hash, tx := range pendingTxs {
				r, err := ec.TransactionReceipt(context.Background(), hash)
				if err == nil && r.BlockHash == h.Hash() {
					log.WithFields(txData(tx, chainID)).Infof("Transaction was included in block %d in %s", r.BlockNumber.Uint64(), time.Since(txTime[hash]))
					recordTxInclusion(r, tx, chainID, time.Since(txTime[hash]))
					blobsIncluded += len(tx.BlobHashes())
					delete(pendingTxs, hash)
					delete(txTime, hash)
					continue
				}
				acc, err := gethtypes.Sender(gethtypes.NewCancunSigner(chainID), tx)
				if err != nil {
					log.WithError(err).Error("Could not get sender's account address")
					continue
				}

				currNonce, err := ec.NonceAtHash(context.Background(), acc, h.Hash())
				if err != nil {
					log.WithError(err).Error("Could not get sender's account nonce")
					continue
				}
				if tx.Nonce() < currNonce {
					log.WithFields(txData(tx, chainID)).Infof("Transaction has been successfully replaced and included on chain in %s", time.Since(txTime[hash]))
					delete(pendingTxs, hash)
					delete(txTime, hash)
					continue
				}
				if tx.Nonce() != currNonce {
					// This is not an immediate transaction that can be included.
					continue
				}
				if tx.BlobGasFeeCap().Cmp(currBaseFee) >= 0 {
					viabletxs++
					viableBlobs += len(tx.BlobHashes())
					log.WithFields(txData(tx, chainID)).Infof("Transaction was still not included after %s", time.Since(txTime[hash]))
				}
			}
			pendingTransactionGauge.Set(float64(len(pendingTxs)))
			viableTransactionGauge.Set(float64(viabletxs))
			viableBlobsGauge.Set(float64(viableBlobs))
			transactionInclusionCounter.Add(float64(currentPendingTxs - len(pendingTxs)))
			blobInclusionCounter.Add(float64(blobsIncluded))
			blobInclusionBuilderCounter.WithLabelValues(strings.ToValidUTF8(string(h.Extra), "")).Add(float64(blobsIncluded))
			builderCounter.WithLabelValues(strings.ToValidUTF8(string(h.Extra), "")).Add(1)

			log.WithFields(log.Fields{
				"previousPendingTxs": currentPendingTxs,
				"currentPendingTxs":  len(pendingTxs),
				"viableTxs":          viabletxs,
			}).Infof("Post block Summary for blob transactions")
			log.Infof("*/-------------------------------------------------------------------------------------------------------------------------------------------------------------------*/")
		}
	}
}
