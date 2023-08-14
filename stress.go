// Copyright 2017 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dbtester

import (
	"fmt"
	"math"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/etcd-io/dbtester/dbtesterpb"

	"go.etcd.io/etcd/client/v3"

	"github.com/etcd-io/etcd/pkg/report"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"golang.org/x/time/rate"
)

type values struct {
	bytes      [][]byte
	strings    []string
	sampleSize int
}

func newValues(gcfg dbtesterpb.ConfigClientMachineAgentControl) (v values, rerr error) {
	v.bytes = [][]byte{randBytes(gcfg.ConfigClientMachineBenchmarkOptions.ValueSizeBytes)}
	v.strings = []string{string(v.bytes[0])}
	v.sampleSize = 1
	return
}

// Stress stresses the database.
func (cfg *Config) Stress(databaseID string) error {
	gcfg, ok := cfg.DatabaseIDToConfigClientMachineAgentControl[databaseID]
	if !ok {
		return fmt.Errorf("%q does not exist", databaseID)
	}

	vals, err := newValues(gcfg)
	if err != nil {
		return err
	}

	switch gcfg.ConfigClientMachineBenchmarkOptions.Type {
	case "write":
		cfg.lg.Info("write generateReport is started...")

		// fixed number of client numbers
		if len(gcfg.ConfigClientMachineBenchmarkOptions.ConnectionClientNumbers) == 0 {
			h, done := newWriteHandlers(cfg.lg, gcfg)
			reqGen := func(inflightReqs chan<- request) { generateWrites(gcfg, 0, vals, inflightReqs) }
			cfg.generateReport(gcfg, h, done, reqGen)

		} else {
			// variable client numbers
			rs := assignRequest(gcfg.ConfigClientMachineBenchmarkOptions.ConnectionClientNumbers, gcfg.ConfigClientMachineBenchmarkOptions.RequestNumber)

			var stats []report.Stats
			reqCompleted := int64(0)
			for i := 0; i < len(rs); i++ {
				copied := gcfg
				copied.ConfigClientMachineBenchmarkOptions.ConnectionNumber = gcfg.ConfigClientMachineBenchmarkOptions.ConnectionClientNumbers[i]
				copied.ConfigClientMachineBenchmarkOptions.ClientNumber = gcfg.ConfigClientMachineBenchmarkOptions.ConnectionClientNumbers[i]
				copied.ConfigClientMachineBenchmarkOptions.RequestNumber = rs[i]
				ncfg := *cfg
				ncfg.DatabaseIDToConfigClientMachineAgentControl[databaseID] = copied

				go func() {
					cfg.lg.Sugar().Infof("signaling agent with client number %d", copied.ConfigClientMachineBenchmarkOptions.ClientNumber)
					if _, err := (&ncfg).BroadcaseRequest(databaseID, dbtesterpb.Operation_Heartbeat); err != nil {
						panic(err)
					}
				}()

				h, done := newWriteHandlers(cfg.lg, copied)
				reqGen := func(inflightReqs chan<- request) { generateWrites(copied, reqCompleted, vals, inflightReqs) }
				b := newBenchmark(copied.ConfigClientMachineBenchmarkOptions.RequestNumber, copied.ConfigClientMachineBenchmarkOptions.ClientNumber, h, done, reqGen)

				// wait until rs[i] requests are finished
				// do not end reports yet
				b.startRequests()
				b.waitRequestsEnd()

				cfg.lg.Info("finishing reports...")
				now := time.Now()
				b.finishReports()
				cfg.lg.Sugar().Infof("finished reports... took %v", time.Since(now))

				reqCompleted += rs[i]
				stats = append(stats, b.stats)
			}
			cfg.lg.Info("combining all reports")

			combined := report.Stats{ErrorDist: make(map[string]int)}
			combinedClientNumber := make([]int64, 0, gcfg.ConfigClientMachineBenchmarkOptions.RequestNumber)
			for i, st := range stats {
				combined.AvgTotal += st.AvgTotal
				combined.Total += st.Total
				combined.Lats = append(combined.Lats, st.Lats...)
				combined.TimeSeries = append(combined.TimeSeries, st.TimeSeries...)
				//
				// Need to handle duplicate unix second timestamps when two ranges are merged.
				// This can happen when the following run happens within the same unix timesecond,
				// since finishing up the previous report and restarting the next range of requests
				// with different number of clients takes only 100+/- ms.
				//
				// For instance, we have the following raw data:
				//
				//   unix-second, client-number, throughput
				//   1486389257,       700,         30335  === ending of previous combined.TimeSeries
				//   1486389258,      "700",        23188  === ending of previous combined.TimeSeries
				//   1486389258,       1000,         5739  === beginning of current st.TimeSeries
				//
				// So now we have two duplicate unix time seconds.
				// This will be handled in aggregating by keys.
				//
				clientN := gcfg.ConfigClientMachineBenchmarkOptions.ConnectionClientNumbers[i]
				clientNs := make([]int64, len(st.TimeSeries))
				for i := range st.TimeSeries {
					clientNs[i] = clientN
				}
				combinedClientNumber = append(combinedClientNumber, clientNs...)

				for k, v := range st.ErrorDist {
					if _, ok := combined.ErrorDist[k]; !ok {
						combined.ErrorDist[k] = v
					} else {
						combined.ErrorDist[k] += v
					}
				}
			}
			if len(combined.TimeSeries) != len(combinedClientNumber) {
				return fmt.Errorf("len(combined.TimeSeries) %d != len(combinedClientNumber) %d", len(combined.TimeSeries), len(combinedClientNumber))
			}

			combined.Average = combined.AvgTotal / float64(len(combined.Lats))
			combined.RPS = float64(len(combined.Lats)) / combined.Total.Seconds()
			cfg.lg.Sugar().Infof("got total %d data points and total %f seconds (RPS %f)", len(combined.Lats), combined.Total.Seconds(), combined.RPS)

			for i := range combined.Lats {
				dev := combined.Lats[i] - combined.Average
				combined.Stddev += dev * dev
			}
			combined.Stddev = math.Sqrt(combined.Stddev / float64(len(combined.Lats)))

			sort.Float64s(combined.Lats)
			if len(combined.Lats) > 0 {
				combined.Fastest = combined.Lats[0]
				combined.Slowest = combined.Lats[len(combined.Lats)-1]
			}

			cfg.lg.Info("combined all reports")
			printStats(combined)
			cfg.saveAllStats(gcfg, combined, combinedClientNumber)
		}

		cfg.lg.Info("write generateReport is finished...")

		cfg.lg.Info("checking total keys on", zap.Strings("endpoints", gcfg.DatabaseEndpoints))
		var totalKeysFunc func(*zap.Logger, []string) map[string]int64
		switch gcfg.DatabaseID {
		case "etcd__other", "etcd__tip", "etcd__v3_2", "etcd__v3_3":
			totalKeysFunc = getTotalKeysEtcdv3
		case "zookeeper__r3_5_3_beta", "zetcd__beta":
			totalKeysFunc = getTotalKeysZk
		case "consul__v1_0_2", "cetcd__beta":
			totalKeysFunc = getTotalKeysConsul
		default:
			cfg.lg.Fatal("unknown database ID", zap.String("database", gcfg.DatabaseID))
		}
		for k, v := range totalKeysFunc(cfg.lg, gcfg.DatabaseEndpoints) {
			cfg.lg.Sugar().Infof("expected write total results [expected_total: %d | database: %q | endpoint: %q | number_of_keys: %d]",
				gcfg.ConfigClientMachineBenchmarkOptions.RequestNumber, gcfg.DatabaseID, k, v)
		}

	case "read":
		key, value := sameKey(gcfg.ConfigClientMachineBenchmarkOptions.KeySizeBytes), vals.strings[0]

		switch gcfg.DatabaseID {
		case "etcd__other", "etcd__tip", "etcd__v3_2", "etcd__v3_3":
			cfg.lg.Sugar().Infof("write started [request: PUT | key: %q | database: %q]", key, gcfg.DatabaseID)
			var err error
			for i := 0; i < 7; i++ {
				clients := mustCreateClientsEtcdv3(gcfg.DatabaseEndpoints, etcdv3ClientCfg{
					totalConns:   1,
					totalClients: 1,
				})
				_, err = clients[0].Do(context.Background(), clientv3.OpPut(key, value))
				if err != nil {
					continue
				}
				cfg.lg.Sugar().Infof("write done [request: PUT | key: %q | database: %q]", key, gcfg.DatabaseID)
				break
			}
			if err != nil {
				cfg.lg.Sugar().Fatalf("write error [request: PUT | key: %q | database: %q]", key, gcfg.DatabaseID)
				os.Exit(1)
			}

		case "zookeeper__r3_5_3_beta", "zetcd__beta":
			cfg.lg.Sugar().Infof("write started [request: PUT | key: %q | database: %q]", key, gcfg.DatabaseID)
			var err error
			for i := 0; i < 7; i++ {
				conns := mustCreateConnsZk(gcfg.DatabaseEndpoints, gcfg.ConfigClientMachineBenchmarkOptions.ConnectionNumber)
				_, err = conns[0].Create("/"+key, vals.bytes[0], zkCreateFlags, zkCreateACL)
				if err != nil {
					continue
				}
				for j := range conns {
					conns[j].Close()
				}
				cfg.lg.Sugar().Infof("write done [request: PUT | key: %q | database: %q]", key, gcfg.DatabaseID)
				break
			}
			if err != nil {
				cfg.lg.Sugar().Fatalf("write error [request: PUT | key: %q | database: %q]", key, gcfg.DatabaseID)
				os.Exit(1)
			}

		case "consul__v1_0_2", "cetcd__beta":
			cfg.lg.Sugar().Infof("write started [request: PUT | key: %q | database: %q]", key, gcfg.DatabaseID)
			var err error
			for i := 0; i < 7; i++ {
				clients := mustCreateConnsConsul(gcfg.DatabaseEndpoints, gcfg.ConfigClientMachineBenchmarkOptions.ConnectionNumber)
				writer := newPutConsul(clients[0])
				err = writer(context.Background(), &request{consulOp: consulOp{key: key, value: vals.bytes[0]}})
				if err != nil {
					continue
				}
				cfg.lg.Sugar().Infof("write done [request: PUT | key: %q | database: %q]", key, gcfg.DatabaseID)
				break
			}
			if err != nil {
				cfg.lg.Sugar().Fatalf("write done [request: PUT | key: %q | database: %q]", key, gcfg.DatabaseID)
				os.Exit(1)
			}

		default:
			panic(fmt.Sprintf("%q is unknown database ID", gcfg.DatabaseID))
		}

		h, done := newReadHandlers(gcfg)
		reqGen := func(inflightReqs chan<- request) { generateReads(gcfg, key, inflightReqs) }
		cfg.generateReport(gcfg, h, done, reqGen)
		cfg.lg.Info("read generateReport is finished...")

	case "read-oneshot":
		key, value := sameKey(gcfg.ConfigClientMachineBenchmarkOptions.KeySizeBytes), vals.strings[0]
		cfg.lg.Sugar().Infof("writing key for read-oneshot [key: %q | database: %q]", key, gcfg.DatabaseID)
		var err error
		switch gcfg.DatabaseID {
		case "etcd__other", "etcd__tip", "etcd__v3_2", "etcd__v3_3":
			clients := mustCreateClientsEtcdv3(gcfg.DatabaseEndpoints, etcdv3ClientCfg{
				totalConns:   1,
				totalClients: 1,
			})
			_, err = clients[0].Do(context.Background(), clientv3.OpPut(key, value))
			clients[0].Close()

		case "zookeeper__r3_5_3_beta", "zetcd__beta":
			conns := mustCreateConnsZk(gcfg.DatabaseEndpoints, 1)
			_, err = conns[0].Create("/"+key, vals.bytes[0], zkCreateFlags, zkCreateACL)
			conns[0].Close()

		case "consul__v1_0_2", "cetcd__beta":
			clients := mustCreateConnsConsul(gcfg.DatabaseEndpoints, 1)
			writer := newPutConsul(clients[0])
			err = writer(context.Background(), &request{consulOp: consulOp{key: key, value: vals.bytes[0]}})

		default:
			panic(fmt.Sprintf("%q is unknown database ID", gcfg.DatabaseID))
		}
		if err != nil {
			cfg.lg.Sugar().Fatalf("write error on read-oneshot (%v)", err)
			os.Exit(1)
		}

		h := newReadOneshotHandlers(cfg.lg, gcfg)
		reqGen := func(inflightReqs chan<- request) { generateReads(gcfg, key, inflightReqs) }
		cfg.generateReport(gcfg, h, nil, reqGen)
		cfg.lg.Info("read-oneshot generateReport is finished...")
	}

	return nil
}

func newReadHandlers(gcfg dbtesterpb.ConfigClientMachineAgentControl) (rhs []ReqHandler, done func()) {
	rhs = make([]ReqHandler, gcfg.ConfigClientMachineBenchmarkOptions.ClientNumber)
	switch gcfg.DatabaseID {
	case "etcd__other", "etcd__tip", "etcd__v3_2", "etcd__v3_3":
		clients := mustCreateClientsEtcdv3(gcfg.DatabaseEndpoints, etcdv3ClientCfg{
			totalConns:   gcfg.ConfigClientMachineBenchmarkOptions.ConnectionNumber,
			totalClients: gcfg.ConfigClientMachineBenchmarkOptions.ClientNumber,
		})
		for i := range clients {
			rhs[i] = newGetEtcd3(clients[i].KV)
		}
		done = func() {
			for i := range clients {
				clients[i].Close()
			}
		}

	case "zookeeper__r3_5_3_beta", "zetcd__beta":
		conns := mustCreateConnsZk(gcfg.DatabaseEndpoints, gcfg.ConfigClientMachineBenchmarkOptions.ConnectionNumber)
		for i := range conns {
			rhs[i] = newGetZK(conns[i])
		}
		done = func() {
			for i := range conns {
				conns[i].Close()
			}
		}

	case "consul__v1_0_2", "cetcd__beta":
		conns := mustCreateConnsConsul(gcfg.DatabaseEndpoints, gcfg.ConfigClientMachineBenchmarkOptions.ConnectionNumber)
		for i := range conns {
			rhs[i] = newGetConsul(conns[i])
		}

	default:
		panic(fmt.Sprintf("%q is unknown database ID", gcfg.DatabaseID))
	}
	return rhs, done
}

func newWriteHandlers(lg *zap.Logger, gcfg dbtesterpb.ConfigClientMachineAgentControl) (rhs []ReqHandler, done func()) {
	rhs = make([]ReqHandler, gcfg.ConfigClientMachineBenchmarkOptions.ClientNumber)
	switch gcfg.DatabaseID {
	case "etcd__other", "etcd__tip", "etcd__v3_2", "etcd__v3_3":
		etcdClients := mustCreateClientsEtcdv3(gcfg.DatabaseEndpoints, etcdv3ClientCfg{
			totalConns:   gcfg.ConfigClientMachineBenchmarkOptions.ConnectionNumber,
			totalClients: gcfg.ConfigClientMachineBenchmarkOptions.ClientNumber,
		})
		for i := range etcdClients {
			rhs[i] = newPutEtcd3(etcdClients[i])
		}

		done = func() {
			// this "done" function may decrease throughput at the end
			for i := range etcdClients {
				go func(idx int) {
					etcdClients[idx].Close()
				}(i)
			}
		}

	case "zookeeper__r3_5_3_beta", "zetcd__beta":
		if gcfg.ConfigClientMachineBenchmarkOptions.SameKey {
			key := sameKey(gcfg.ConfigClientMachineBenchmarkOptions.KeySizeBytes)
			valueBts := randBytes(gcfg.ConfigClientMachineBenchmarkOptions.ValueSizeBytes)
			lg.Sugar().Infof("write started [request: PUT | key: %q | database: %q]", key, gcfg.DatabaseID)
			var err error
			for i := 0; i < 7; i++ {
				conns := mustCreateConnsZk(gcfg.DatabaseEndpoints, gcfg.ConfigClientMachineBenchmarkOptions.ConnectionNumber)
				_, err = conns[0].Create("/"+key, valueBts, zkCreateFlags, zkCreateACL)
				if err != nil {
					continue
				}
				for j := range conns {
					conns[j].Close()
				}
				lg.Sugar().Infof("write done [request: PUT | key: %q | database: %q]", key, gcfg.DatabaseID)
				break
			}
			if err != nil {
				lg.Sugar().Fatalf("write error [request: PUT | key: %q | database: %q]", key, gcfg.DatabaseID)
				os.Exit(1)
			}
		}

		conns := mustCreateConnsZk(gcfg.DatabaseEndpoints, gcfg.ConfigClientMachineBenchmarkOptions.ConnectionNumber)
		for i := range conns {
			if gcfg.ConfigClientMachineBenchmarkOptions.SameKey {
				rhs[i] = newPutOverwriteZK(conns[i])
			} else {
				rhs[i] = newPutCreateZK(conns[i])
			}
		}
		done = func() {
			for i := range conns {
				go func(idx int) {
					conns[idx].Close()
				}(i)
			}
		}

	case "consul__v1_0_2", "cetcd__beta":
		conns := mustCreateConnsConsul(gcfg.DatabaseEndpoints, gcfg.ConfigClientMachineBenchmarkOptions.ConnectionNumber)
		for i := range conns {
			rhs[i] = newPutConsul(conns[i])
		}

	default:
		lg.Sugar().Fatalf("%q is unknown database ID", gcfg.DatabaseID)
	}

	for k := range rhs {
		if rhs[k] == nil {
			lg.Sugar().Fatalf("%d-th write handler is nil (out of %d)", k, len(rhs))
		}
	}
	return
}

func newReadOneshotHandlers(lg *zap.Logger, gcfg dbtesterpb.ConfigClientMachineAgentControl) []ReqHandler {
	rhs := make([]ReqHandler, gcfg.ConfigClientMachineBenchmarkOptions.ClientNumber)
	switch gcfg.DatabaseID {
	case "etcd__other", "etcd__tip", "etcd__v3_2", "etcd__v3_3":
		for i := range rhs {
			rhs[i] = func(ctx context.Context, req *request) error {
				conns := mustCreateClientsEtcdv3(gcfg.DatabaseEndpoints, etcdv3ClientCfg{
					totalConns:   1,
					totalClients: 1,
				})
				defer conns[0].Close()
				return newGetEtcd3(conns[0])(ctx, req)
			}
		}

	case "zookeeper__r3_5_3_beta", "zetcd__beta":
		for i := range rhs {
			rhs[i] = func(ctx context.Context, req *request) error {
				conns := mustCreateConnsZk(gcfg.DatabaseEndpoints, gcfg.ConfigClientMachineBenchmarkOptions.ConnectionNumber)
				defer conns[0].Close()
				return newGetZK(conns[0])(ctx, req)
			}
		}

	case "consul__v1_0_2", "cetcd__beta":
		for i := range rhs {
			rhs[i] = func(ctx context.Context, req *request) error {
				conns := mustCreateConnsConsul(gcfg.DatabaseEndpoints, 1)
				return newGetConsul(conns[0])(ctx, req)
			}
		}
	default:
		lg.Sugar().Fatalf("%q is unknown database ID", gcfg.DatabaseID)
	}
	return rhs
}

func generateReads(gcfg dbtesterpb.ConfigClientMachineAgentControl, key string, inflightReqs chan<- request) {
	defer close(inflightReqs)

	var rateLimiter *rate.Limiter
	if gcfg.ConfigClientMachineBenchmarkOptions.RateLimitRequestsPerSecond > 0 {
		rateLimiter = rate.NewLimiter(
			rate.Limit(gcfg.ConfigClientMachineBenchmarkOptions.RateLimitRequestsPerSecond),
			int(gcfg.ConfigClientMachineBenchmarkOptions.RateLimitRequestsPerSecond),
		)
	}

	for i := int64(0); i < gcfg.ConfigClientMachineBenchmarkOptions.RequestNumber; i++ {
		if rateLimiter != nil {
			rateLimiter.Wait(context.TODO())
		}

		switch gcfg.DatabaseID {
		case "etcd__other", "etcd__tip", "etcd__v3_2", "etcd__v3_3":
			opts := []clientv3.OpOption{clientv3.WithRange("")}
			if gcfg.ConfigClientMachineBenchmarkOptions.StaleRead {
				opts = append(opts, clientv3.WithSerializable())
			}
			inflightReqs <- request{etcdv3Op: clientv3.OpGet(key, opts...)}

		case "zookeeper__r3_5_3_beta", "zetcd__beta":
			op := zkOp{key: key}
			if gcfg.ConfigClientMachineBenchmarkOptions.StaleRead {
				op.staleRead = true
			}
			inflightReqs <- request{zkOp: op}

		case "consul__v1_0_2", "cetcd__beta":
			op := consulOp{key: key}
			if gcfg.ConfigClientMachineBenchmarkOptions.StaleRead {
				op.staleRead = true
			}
			inflightReqs <- request{consulOp: op}
		default:
			panic(fmt.Sprintf("%q is unknown database ID", gcfg.DatabaseID))
		}
	}
}

func generateWrites(gcfg dbtesterpb.ConfigClientMachineAgentControl, startIdx int64, vals values, inflightReqs chan<- request) {
	var rateLimiter *rate.Limiter
	if gcfg.ConfigClientMachineBenchmarkOptions.RateLimitRequestsPerSecond > 0 {
		rateLimiter = rate.NewLimiter(
			rate.Limit(gcfg.ConfigClientMachineBenchmarkOptions.RateLimitRequestsPerSecond),
			int(gcfg.ConfigClientMachineBenchmarkOptions.RateLimitRequestsPerSecond),
		)
	}

	var wg sync.WaitGroup
	defer func() {
		close(inflightReqs)
		wg.Wait()
	}()

	for i := int64(0); i < gcfg.ConfigClientMachineBenchmarkOptions.RequestNumber; i++ {
		k := sequentialKey(gcfg.ConfigClientMachineBenchmarkOptions.KeySizeBytes, i+startIdx)
		if gcfg.ConfigClientMachineBenchmarkOptions.SameKey {
			k = sameKey(gcfg.ConfigClientMachineBenchmarkOptions.KeySizeBytes)
		}

		v := vals.bytes[i%int64(vals.sampleSize)]
		vs := vals.strings[i%int64(vals.sampleSize)]

		if rateLimiter != nil {
			rateLimiter.Wait(context.TODO())
		}

		switch gcfg.DatabaseID {
		case "etcd__other", "etcd__tip", "etcd__v3_2", "etcd__v3_3":
			inflightReqs <- request{etcdv3Op: clientv3.OpPut(k, vs)}

		case "zookeeper__r3_5_3_beta", "zetcd__beta":
			inflightReqs <- request{zkOp: zkOp{key: "/" + k, value: v}}

		case "consul__v1_0_2", "cetcd__beta":
			inflightReqs <- request{consulOp: consulOp{key: k, value: v}}

		default:
			panic(fmt.Sprintf("%q is unknown database ID", gcfg.DatabaseID))
		}
	}
}
