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
	"sync"
	"time"

	"github.com/cheggaaa/pb"
	"github.com/etcd-io/dbtester/dbtesterpb"
	"github.com/etcd-io/etcd/pkg/report"
	"golang.org/x/net/context"
)

type benchmark struct {
	bar        *pb.ProgressBar
	report     report.Report
	reportDone <-chan report.Stats
	stats      report.Stats

	reqHandlers []ReqHandler
	reqGen      func(chan<- request)
	reqDone     func()
	wg          sync.WaitGroup

	mu           sync.RWMutex
	inflightReqs chan request
}

// pass totalN in case that 'cfg' is manipulated
func newBenchmark(totalN int64, clientsN int64, reqHandlers []ReqHandler, reqDone func(), reqGen func(chan<- request)) (b *benchmark) {
	b = &benchmark{
		bar:         pb.New(int(totalN)),
		reqHandlers: reqHandlers,
		reqGen:      reqGen,
		reqDone:     reqDone,
		wg:          sync.WaitGroup{},
	}
	b.inflightReqs = make(chan request, clientsN)

	b.bar.Format("Bom !")
	b.bar.Start()
	b.report = report.NewReportSample("%4.4f")
	return
}

// only useful when multiple ranges of requests are run with one report
func (b *benchmark) reset(clientsN int64, reqHandlers []ReqHandler, reqDone func(), reqGen func(chan<- request)) {
	if len(reqHandlers) == 0 {
		panic(fmt.Errorf("got 0 reqHandlers"))
	}
	b.reqHandlers = reqHandlers
	b.reqDone = reqDone
	b.reqGen = reqGen

	// inflight requests will be dropped!
	b.mu.Lock()
	b.inflightReqs = make(chan request, clientsN)
	b.mu.Unlock()
}

func (b *benchmark) getInflightsReqs() (ch chan request) {
	b.mu.RLock()
	ch = b.inflightReqs
	b.mu.RUnlock()
	return
}

func (b *benchmark) startRequests() {
	for i := range b.reqHandlers {
		b.wg.Add(1)
		go func(rh ReqHandler) {
			defer b.wg.Done()
			for req := range b.getInflightsReqs() {
				if rh == nil {
					panic(fmt.Errorf("got nil rh"))
				}
				st := time.Now()
				err := rh(context.Background(), &req)
				b.report.Results() <- report.Result{Err: err, Start: st, End: time.Now()}
				b.bar.Increment()
			}
		}(b.reqHandlers[i])
	}
	go b.reqGen(b.getInflightsReqs())
	b.reportDone = b.report.Stats()
}

func (b *benchmark) waitRequestsEnd() {
	b.wg.Wait()
	if b.reqDone != nil {
		b.reqDone() // cancel connections
	}
}

func (b *benchmark) finishReports() {
	close(b.report.Results())
	b.bar.Finish()
	st := <-b.reportDone
	b.stats = st
}

func (b *benchmark) waitAll() {
	b.waitRequestsEnd()
	b.finishReports()
}

func printStats(st report.Stats) {
	// to be piped to cfg.Log via stdout when dbtester executed
	if len(st.Lats) > 0 {
		fmt.Printf("Total: %v\n", st.Total)
		fmt.Printf("Slowest: %f secs\n", st.Slowest)
		fmt.Printf("Fastest: %f secs\n", st.Fastest)
		fmt.Printf("Average: %f secs\n", st.Average)
		fmt.Printf("Requests/sec: %4.4f\n", st.RPS)
	}
	if len(st.ErrorDist) > 0 {
		for k, v := range st.ErrorDist {
			fmt.Printf("ERROR %q : %d\n", k, v)
		}
	} else {
		fmt.Println("ERRRO: 0")
	}
}

func (cfg *Config) generateReport(gcfg dbtesterpb.ConfigClientMachineAgentControl, h []ReqHandler, reqDone func(), reqGen func(chan<- request)) {
	b := newBenchmark(gcfg.ConfigClientMachineBenchmarkOptions.RequestNumber, gcfg.ConfigClientMachineBenchmarkOptions.ClientNumber, h, reqDone, reqGen)
	b.startRequests()
	b.waitAll()

	printStats(b.stats)
	cfg.saveAllStats(gcfg, b.stats, nil)
}
