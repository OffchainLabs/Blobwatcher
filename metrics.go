package main

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

var (
	transactionsObservedGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "transactions_observed",
		Help: "Count the number of blob transactions observed in your local mempool",
	}, []string{"account", "blobCount", "maxBlobBaseFee"})
	transactionInclusionDelay = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "transaction_inclusion_delay",
			Help:    "The number of seconds it takes to include a blob transaction on chain",
			Buckets: []float64{1, 2, 16, 32, 64, 128, 256, 512, 1024},
		},
		[]string{"account", "blobCount"},
	)
	blobTransactionFeesPaid = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "blob_transaction_fees_paid",
			Help:    "The transaction fees paid by the account for the blob transaction(in Gwei)",
			Buckets: []float64{0.001, 0.01, 1, 10, 1000, 100000, 10000000, 1000000000, 10000000000, 1000000000000},
		},
		[]string{"account", "blobCount"},
	)
	blockNumberGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "block_number",
		Help: "The current block number in your execution client",
	})
	executionBaseFeeGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "execution_base_fee",
		Help: "The execution base fee",
	})
	blobBaseFeeGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "blob_base_fee",
		Help: "The blob base fee",
	})
	pendingTransactionGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pending_transactions",
		Help: "The current number of pending transactions in the mempool",
	})
	viableTransactionGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "viable_transaction",
		Help: "The current number of viable transactions in the mempool",
	})
	viableBlobsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "viable_blobs",
		Help: "The current number of viable blobs in the mempool",
	})
	transactionTip = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "transaction_tip",
		Help:    "The execution tip of a blob transaction that was included in a block(in Gwei)",
		Buckets: []float64{0.01, 0.1, 1, 5, 10, 15, 20, 30, 50, 80, 100, 200, 400, 800, 1000},
	}, []string{"account", "blobCount"})
	transactionInclusionCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "transaction_inclusion",
		Help: "The current number of transactions included in a block",
	})
	blobInclusionCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "blob_inclusion",
		Help: "The number of blobs included on chain via a transaction",
	})
	blobInclusionBuilderCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "blob_inclusion_by_builder",
		Help: "The number of blobs included on chain via a transaction by builder",
	}, []string{"builder"})
	builderCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "builder_blocks",
		Help: "The number of blocks built by a builder",
	}, []string{"builder"})
)

func StartMetricsServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
		MaxRequestsInFlight: 5,
		Timeout:             30 * time.Second,
	}))

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: time.Second}
	log.WithField("address", srv.Addr).Debug("Starting prometheus server")
	go func() {
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatalf("Could not listen to host:port :%s", srv.Addr)
		}
	}()
	return srv
}
