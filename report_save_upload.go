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
	"path/filepath"
	"strings"
	"time"

	"github.com/etcd-io/dbtester/dbtesterpb"
	"github.com/etcd-io/dbtester/pkg/remotestorage"

	"github.com/etcd-io/etcd/pkg/report"
	humanize "github.com/dustin/go-humanize"
	"github.com/gyuho/dataframe"
)

// DiskSpaceUsageSummaryColumns defines summary columns.
var DiskSpaceUsageSummaryColumns = []string{
	"INDEX",
	"DATABASE-ENDPOINT",
	"DISK-SPACE-USAGE",
	"DISK-SPACE-USAGE-BYTES-NUM",
}

// SaveDiskSpaceUsageSummary saves data size summary.
func (cfg *Config) SaveDiskSpaceUsageSummary(databaseID string, idxToResponse map[int]dbtesterpb.Response) error {
	gcfg, ok := cfg.DatabaseIDToConfigClientMachineAgentControl[databaseID]
	if !ok {
		return fmt.Errorf("%q does not exist", databaseID)
	}

	c1 := dataframe.NewColumn(DiskSpaceUsageSummaryColumns[0])
	c2 := dataframe.NewColumn(DiskSpaceUsageSummaryColumns[1])
	c3 := dataframe.NewColumn(DiskSpaceUsageSummaryColumns[2])
	c4 := dataframe.NewColumn(DiskSpaceUsageSummaryColumns[3])
	for i := range gcfg.DatabaseEndpoints {
		c1.PushBack(dataframe.NewStringValue(i))
		c2.PushBack(dataframe.NewStringValue(gcfg.DatabaseEndpoints[i]))
		c3.PushBack(dataframe.NewStringValue(humanize.Bytes(uint64(idxToResponse[i].DiskSpaceUsageBytes))))
		c4.PushBack(dataframe.NewStringValue(idxToResponse[i].DiskSpaceUsageBytes))
	}

	fr := dataframe.New()
	if err := fr.AddColumn(c1); err != nil {
		return err
	}
	if err := fr.AddColumn(c2); err != nil {
		return err
	}
	if err := fr.AddColumn(c3); err != nil {
		return err
	}
	if err := fr.AddColumn(c4); err != nil {
		return err
	}

	return fr.CSV(cfg.ConfigClientMachineInitial.ServerDiskSpaceUsageSummaryPath)
}

func (cfg *Config) saveDataLatencyDistributionSummary(st report.Stats) {
	fr := dataframe.New()

	c1 := dataframe.NewColumn("TOTAL-SECONDS")
	c1.PushBack(dataframe.NewStringValue(fmt.Sprintf("%4.4f", st.Total.Seconds())))
	if err := fr.AddColumn(c1); err != nil {
		panic(err)
	}

	c2 := dataframe.NewColumn("REQUESTS-PER-SECOND")
	c2.PushBack(dataframe.NewStringValue(fmt.Sprintf("%4.4f", st.RPS)))
	if err := fr.AddColumn(c2); err != nil {
		panic(err)
	}

	c3 := dataframe.NewColumn("SLOWEST-LATENCY-MS")
	c3.PushBack(dataframe.NewStringValue(fmt.Sprintf("%4.4f", 1000*st.Slowest)))
	if err := fr.AddColumn(c3); err != nil {
		panic(err)
	}

	c4 := dataframe.NewColumn("FASTEST-LATENCY-MS")
	c4.PushBack(dataframe.NewStringValue(fmt.Sprintf("%4.4f", 1000*st.Fastest)))
	if err := fr.AddColumn(c4); err != nil {
		panic(err)
	}

	c5 := dataframe.NewColumn("AVERAGE-LATENCY-MS")
	c5.PushBack(dataframe.NewStringValue(fmt.Sprintf("%4.4f", 1000*st.Average)))
	if err := fr.AddColumn(c5); err != nil {
		panic(err)
	}

	c6 := dataframe.NewColumn("STDDEV-LATENCY-MS")
	c6.PushBack(dataframe.NewStringValue(fmt.Sprintf("%4.4f", 1000*st.Stddev)))
	if err := fr.AddColumn(c6); err != nil {
		panic(err)
	}

	if len(st.ErrorDist) > 0 {
		for errName, errN := range st.ErrorDist {
			errcol := dataframe.NewColumn(fmt.Sprintf("ERROR: %q", errName))
			errcol.PushBack(dataframe.NewStringValue(errN))
			if err := fr.AddColumn(errcol); err != nil {
				panic(err)
			}
		}
	} else {
		errcol := dataframe.NewColumn("ERROR")
		errcol.PushBack(dataframe.NewStringValue("0"))
		if err := fr.AddColumn(errcol); err != nil {
			panic(err)
		}
	}

	if err := fr.CSVHorizontal(cfg.ConfigClientMachineInitial.ClientLatencyDistributionSummaryPath); err != nil {
		panic(err)
	}
}

func (cfg *Config) saveDataLatencyDistributionPercentile(st report.Stats) {
	pctls, seconds := report.Percentiles(st.Lats)
	c1 := dataframe.NewColumn("LATENCY-PERCENTILE")
	c2 := dataframe.NewColumn("LATENCY-MS")
	for i := range pctls {
		pct := fmt.Sprintf("p%.1f", pctls[i])
		if strings.HasSuffix(pct, ".0") {
			pct = strings.Replace(pct, ".0", "", -1)
		}

		c1.PushBack(dataframe.NewStringValue(pct))
		c2.PushBack(dataframe.NewStringValue(fmt.Sprintf("%f", 1000*seconds[i])))
	}

	fr := dataframe.New()
	if err := fr.AddColumn(c1); err != nil {
		panic(err)
	}
	if err := fr.AddColumn(c2); err != nil {
		panic(err)
	}
	if err := fr.CSV(cfg.ConfigClientMachineInitial.ClientLatencyDistributionPercentilePath); err != nil {
		panic(err)
	}
}

func (cfg *Config) saveDataLatencyDistributionAll(st report.Stats) {
	min := int64(math.MaxInt64)
	max := int64(-100000)
	rm := make(map[int64]int64)
	for _, lt := range st.Lats {
		// convert second(float64) to millisecond
		ms := lt * 1000

		// truncate all digits below 10ms
		// (e.g. 125.11ms becomes 120ms)
		v := int64(math.Trunc(ms/10) * 10)
		if _, ok := rm[v]; !ok {
			rm[v] = 1
		} else {
			rm[v]++
		}

		if min > v {
			min = v
		}
		if max < v {
			max = v
		}
	}

	c1 := dataframe.NewColumn("LATENCY-MS")
	c2 := dataframe.NewColumn("COUNT")
	cur := min
	for {
		c1.PushBack(dataframe.NewStringValue(fmt.Sprintf("%d", int64(cur))))
		v, ok := rm[cur]
		if ok {
			c2.PushBack(dataframe.NewStringValue(fmt.Sprintf("%d", v)))
		} else {
			c2.PushBack(dataframe.NewStringValue("0"))
		}
		cur += 10
		if cur-10 == max { // was last point
			break
		}
	}
	fr := dataframe.New()
	if err := fr.AddColumn(c1); err != nil {
		panic(err)
	}
	if err := fr.AddColumn(c2); err != nil {
		panic(err)
	}
	if err := fr.CSV(cfg.ConfigClientMachineInitial.ClientLatencyDistributionAllPath); err != nil {
		panic(err)
	}
}

func (cfg *Config) saveDataLatencyThroughputTimeseries(gcfg dbtesterpb.ConfigClientMachineAgentControl, st report.Stats, clientNs []int64) {
	if len(clientNs) == 0 && len(gcfg.ConfigClientMachineBenchmarkOptions.ConnectionClientNumbers) == 0 {
		clientNs = make([]int64, len(st.TimeSeries))
		for i := range clientNs {
			clientNs[i] = gcfg.ConfigClientMachineBenchmarkOptions.ClientNumber
		}
	}
	c1 := dataframe.NewColumn("UNIX-SECOND")
	c2 := dataframe.NewColumn("CONTROL-CLIENT-NUM")
	c3 := dataframe.NewColumn("MIN-LATENCY-MS")
	c4 := dataframe.NewColumn("AVG-LATENCY-MS")
	c5 := dataframe.NewColumn("MAX-LATENCY-MS")
	c6 := dataframe.NewColumn("AVG-THROUGHPUT")
	for i := range st.TimeSeries {
		// this Timestamp is unix seconds
		c1.PushBack(dataframe.NewStringValue(fmt.Sprintf("%d", st.TimeSeries[i].Timestamp)))
		c2.PushBack(dataframe.NewStringValue(fmt.Sprintf("%d", clientNs[i])))
		c3.PushBack(dataframe.NewStringValue(fmt.Sprintf("%f", toMillisecond(st.TimeSeries[i].MinLatency))))
		c4.PushBack(dataframe.NewStringValue(fmt.Sprintf("%f", toMillisecond(st.TimeSeries[i].AvgLatency))))
		c5.PushBack(dataframe.NewStringValue(fmt.Sprintf("%f", toMillisecond(st.TimeSeries[i].MaxLatency))))
		c6.PushBack(dataframe.NewStringValue(fmt.Sprintf("%d", st.TimeSeries[i].ThroughPut)))
	}

	fr := dataframe.New()
	if err := fr.AddColumn(c1); err != nil {
		panic(err)
	}
	if err := fr.AddColumn(c2); err != nil {
		panic(err)
	}
	if err := fr.AddColumn(c3); err != nil {
		panic(err)
	}
	if err := fr.AddColumn(c4); err != nil {
		panic(err)
	}
	if err := fr.AddColumn(c5); err != nil {
		panic(err)
	}
	if err := fr.AddColumn(c6); err != nil {
		panic(err)
	}

	if err := fr.CSV(cfg.ConfigClientMachineInitial.ClientLatencyThroughputTimeseriesPath); err != nil {
		panic(err)
	}

	// aggregate latency by the number of keys
	tss := FindRangesLatency(st.TimeSeries, 1000, gcfg.ConfigClientMachineBenchmarkOptions.RequestNumber)
	ctt1 := dataframe.NewColumn("KEYS")
	ctt2 := dataframe.NewColumn("MIN-LATENCY-MS")
	ctt3 := dataframe.NewColumn("AVG-LATENCY-MS")
	ctt4 := dataframe.NewColumn("MAX-LATENCY-MS")
	for i := range tss {
		ctt1.PushBack(dataframe.NewStringValue(tss[i].CumulativeKeyNum))
		ctt2.PushBack(dataframe.NewStringValue(fmt.Sprintf("%f", toMillisecond(tss[i].MinLatency))))
		ctt3.PushBack(dataframe.NewStringValue(fmt.Sprintf("%f", toMillisecond(tss[i].AvgLatency))))
		ctt4.PushBack(dataframe.NewStringValue(fmt.Sprintf("%f", toMillisecond(tss[i].MaxLatency))))
	}

	frr := dataframe.New()
	if err := frr.AddColumn(ctt1); err != nil {
		panic(err)
	}
	if err := frr.AddColumn(ctt2); err != nil {
		panic(err)
	}
	if err := frr.AddColumn(ctt3); err != nil {
		panic(err)
	}
	if err := frr.AddColumn(ctt4); err != nil {
		panic(err)
	}

	if err := frr.CSV(cfg.ConfigClientMachineInitial.ClientLatencyByKeyNumberPath); err != nil {
		panic(err)
	}
}

func (cfg *Config) saveAllStats(gcfg dbtesterpb.ConfigClientMachineAgentControl, stats report.Stats, clientNs []int64) {
	cfg.saveDataLatencyDistributionSummary(stats)
	cfg.saveDataLatencyDistributionPercentile(stats)
	cfg.saveDataLatencyDistributionAll(stats)
	cfg.saveDataLatencyThroughputTimeseries(gcfg, stats, clientNs)
}

// UploadToGoogle uploads target file to Google Cloud Storage.
func (cfg *Config) UploadToGoogle(databaseID string, targetPath string) error {
	gcfg, ok := cfg.DatabaseIDToConfigClientMachineAgentControl[databaseID]
	if !ok {
		return fmt.Errorf("%q does not exist", databaseID)
	}
	if !exist(targetPath) {
		return fmt.Errorf("%q does not exist", targetPath)
	}
	u, err := remotestorage.NewGoogleCloudStorage(cfg.lg, []byte(cfg.ConfigClientMachineInitial.GoogleCloudStorageKey), cfg.ConfigClientMachineInitial.GoogleCloudProjectName)
	if err != nil {
		return err
	}

	srcPath := targetPath
	dstPath := filepath.Base(targetPath)
	if !strings.HasPrefix(dstPath, gcfg.DatabaseTag) {
		dstPath = fmt.Sprintf("%s-%s", gcfg.DatabaseTag, dstPath)
	}
	dstPath = filepath.Join(cfg.ConfigClientMachineInitial.GoogleCloudStorageSubDirectory, dstPath)

	var uerr error
	for k := 0; k < 30; k++ {
		if uerr = u.UploadFile(cfg.ConfigClientMachineInitial.GoogleCloudStorageBucketName, srcPath, dstPath); uerr != nil {
			cfg.lg.Sugar().Infof("#%d: error %v while uploading %q", k, uerr, targetPath)
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}
	return uerr
}
