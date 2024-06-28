package main

import (
	"fmt"
	"math/big"
	"time"

	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	log "github.com/sirupsen/logrus"
)

func recordTxMetrics(tx *gethtypes.Transaction, chainID *big.Int) {
	acc, err := gethtypes.Sender(gethtypes.NewCancunSigner(chainID), tx)
	if err != nil {
		log.WithError(err).Error("Could not get sender's account address")
		return
	}
	accName := acc.String()
	if name, ok := accountLabels[[20]byte(acc.Bytes())]; ok {
		accName = name
	}
	transactionsObservedGauge.WithLabelValues(accName, fmt.Sprintf("%d", len(tx.BlobHashes())), fmt.Sprintf("%d", tx.BlobGasFeeCap().Uint64())).Inc()
}

func recordTxInclusion(receipt *gethtypes.Receipt, tx *gethtypes.Transaction, chainID *big.Int, inclusionDelay time.Duration) {
	acc, err := gethtypes.Sender(gethtypes.NewCancunSigner(chainID), tx)
	if err != nil {
		log.WithError(err).Error("Could not get sender's account address")
		return
	}
	accName := acc.String()
	if name, ok := accountLabels[[20]byte(acc.Bytes())]; ok {
		accName = name
	}
	gasTip, _ := tx.GasTipCap().Float64()
	gasTipGwei := gasTip / params.GWei
	transactionInclusionDelay.WithLabelValues(accName, fmt.Sprintf("%d", len(tx.BlobHashes()))).Observe(inclusionDelay.Seconds())
	transactionTip.WithLabelValues(accName, fmt.Sprintf("%d", len(tx.BlobHashes()))).Observe(gasTipGwei)
	blobTransactionFeesPaid.WithLabelValues(accName, fmt.Sprintf("%d", len(tx.BlobHashes()))).Observe(costOfTx(receipt, tx))
}

func txData(tx *gethtypes.Transaction, chainID *big.Int) log.Fields {
	acc, err := gethtypes.Sender(gethtypes.NewCancunSigner(chainID), tx)
	if err != nil {
		log.WithError(err).Error("Could not get sender's account address")
		return nil
	}
	accName := acc.String()
	if name, ok := accountLabels[[20]byte(acc.Bytes())]; ok {
		accName = name
	}

	return log.Fields{
		"TxHash":              tx.Hash(),
		"BlobGasFeeCap(Gwei)": float64(tx.BlobGasFeeCap().Uint64()) / params.GWei,
		"BlobGas":             tx.BlobGas(),
		"BlobCount":           len(tx.BlobHashes()),
		"GasFeeCap(Gwei)":     float64(tx.GasFeeCap().Uint64()) / params.GWei,
		"GasTipCap(Gwei)":     float64(tx.GasTipCap().Uint64()) / params.GWei,
		"Gas":                 tx.Gas(),
		"Account":             accName,
	}
}

func costOfTx(receipt *gethtypes.Receipt, tx *gethtypes.Transaction) float64 {
	total := new(big.Int).Mul(receipt.EffectiveGasPrice, new(big.Int).SetUint64(receipt.GasUsed))
	total.Add(total, new(big.Int).Mul(receipt.BlobGasPrice, new(big.Int).SetUint64(receipt.BlobGasUsed)))

	total.Add(total, tx.Value())
	fTotal, _ := total.Float64()
	return fTotal / params.GWei
}
