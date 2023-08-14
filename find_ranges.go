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
	"sort"
	"time"

	"github.com/etcd-io/etcd/pkg/report"
)

// CumulativeKeyNumToAvgLatency wraps the cumulative number of keys
// and according latency data. So the higher 'CumulativeKeyNum' is,
// the later the data points are in the time series.
type CumulativeKeyNumToAvgLatency struct {
	CumulativeKeyNum int64

	MinLatency time.Duration
	AvgLatency time.Duration
	MaxLatency time.Duration
}

// CumulativeKeyNumToAvgLatencySlice is a slice of CumulativeKeyNumToAvgLatency to sort by CumulativeKeyNum.
type CumulativeKeyNumToAvgLatencySlice []CumulativeKeyNumToAvgLatency

func (t CumulativeKeyNumToAvgLatencySlice) Swap(i, j int) { t[i], t[j] = t[j], t[i] }
func (t CumulativeKeyNumToAvgLatencySlice) Len() int      { return len(t) }
func (t CumulativeKeyNumToAvgLatencySlice) Less(i, j int) bool {
	return t[i].CumulativeKeyNum < t[j].CumulativeKeyNum
}

// FindRangesLatency sorts all data points by its timestamp.
// And then aggregate by the cumulative throughput,
// in order to map the number of keys to the average latency.
//
//	type DataPoint struct {
//		Timestamp  int64
//		MinLatency time.Duration
//		AvgLatency time.Duration
//		MaxLatency time.Duration
//		ThroughPut int64
//	}
//
// If unit is 1000 and the average throughput per second is 30,000
// and its average latency is 10ms, it will have 30 data points with
// latency 10ms.
func FindRangesLatency(data report.TimeSeries, unit int64, totalRequests int64) CumulativeKeyNumToAvgLatencySlice {
	// need to sort by timestamps because we want the 'cumulative'
	// trends as we write more keys, 'report.TimeSeries' already implements
	// sort interface, so just sort.Sort(data)
	sort.Sort(data)

	cumulKeyN := int64(0)
	maxKey := int64(0)

	rm := make(map[int64]CumulativeKeyNumToAvgLatency)

	// this data is aggregated by second
	// and we want to map number of keys to latency
	// so the range is the key
	// and the value is the cumulative throughput
	for _, ts := range data {
		cumulKeyN += ts.ThroughPut
		if cumulKeyN < unit {
			// not enough data points yet
			continue
		}

		// cumulKeyN >= unit
		for cumulKeyN > maxKey {
			maxKey += unit
			rm[maxKey] = CumulativeKeyNumToAvgLatency{
				MinLatency: ts.MinLatency,
				AvgLatency: ts.AvgLatency,
				MaxLatency: ts.MaxLatency,
			}
		}
	}

	// fill-in empty rows
	for i := maxKey; i < totalRequests; i += unit {
		if _, ok := rm[i]; !ok {
			rm[i] = CumulativeKeyNumToAvgLatency{}
		}
	}
	if _, ok := rm[totalRequests]; !ok {
		rm[totalRequests] = CumulativeKeyNumToAvgLatency{}
	}

	kss := []CumulativeKeyNumToAvgLatency{}
	delete(rm, 0) // drop data at beginning

	for k, v := range rm {
		// make sure to use 'k' as CumulativeKeyNum
		kss = append(kss, CumulativeKeyNumToAvgLatency{
			CumulativeKeyNum: k,
			MinLatency:       v.MinLatency,
			AvgLatency:       v.AvgLatency,
			MaxLatency:       v.MaxLatency,
		})
	}

	// sort by cumulative throughput (number of keys) in ascending order
	sort.Sort(CumulativeKeyNumToAvgLatencySlice(kss))
	return kss
}

// CumulativeKeyNumAndOtherData wraps the cumulative number of keys
// and according memory data. So the higher 'CumulativeKeyNum' is,
// the later the data points are in the time series.
type CumulativeKeyNumAndOtherData struct {
	UnixSecond int64
	Throughput int64

	CumulativeKeyNum int64

	MinMemoryMB float64
	AvgMemoryMB float64
	MaxMemoryMB float64

	AvgReadBytesDelta  float64
	AvgWriteBytesDelta float64
}

// CumulativeKeyNumAndOtherDataSlice is a slice of CumulativeKeyNumAndOtherData to sort by CumulativeKeyNum.
type CumulativeKeyNumAndOtherDataSlice []CumulativeKeyNumAndOtherData

func (t CumulativeKeyNumAndOtherDataSlice) Swap(i, j int) { t[i], t[j] = t[j], t[i] }
func (t CumulativeKeyNumAndOtherDataSlice) Len() int      { return len(t) }
func (t CumulativeKeyNumAndOtherDataSlice) Less(i, j int) bool {
	return t[i].CumulativeKeyNum < t[j].CumulativeKeyNum
}

// CumulativeKeyNumAndOtherDataByUnixSecond is a slice of CumulativeKeyNumAndOtherData to sort by UnixSecond.
type CumulativeKeyNumAndOtherDataByUnixSecond []CumulativeKeyNumAndOtherData

func (t CumulativeKeyNumAndOtherDataByUnixSecond) Swap(i, j int) { t[i], t[j] = t[j], t[i] }
func (t CumulativeKeyNumAndOtherDataByUnixSecond) Len() int      { return len(t) }
func (t CumulativeKeyNumAndOtherDataByUnixSecond) Less(i, j int) bool {
	return t[i].UnixSecond < t[j].UnixSecond
}

// FindRangesData sorts all data points by its timestamp.
// And then aggregate by the cumulative throughput,
// in order to map the number of keys to the average memory usage.
func FindRangesData(data []CumulativeKeyNumAndOtherData, unit int64, totalRequests int64) CumulativeKeyNumAndOtherDataSlice {
	// need to sort by timestamps because we want the 'cumulative'
	// trends as we write more keys, 'report.TimeSeries' already implements
	// sort interface, so just sort.Sort(data)
	sort.Sort(CumulativeKeyNumAndOtherDataByUnixSecond(data))

	cumulKeyN := int64(0)
	maxKey := int64(0)

	rm := make(map[int64]CumulativeKeyNumAndOtherData)

	// this data is aggregated by second
	// and we want to map number of keys to memory usage
	// so the range is the key
	// and the value is the cumulative throughput
	for _, ts := range data {
		cumulKeyN += ts.Throughput
		if cumulKeyN < unit {
			// not enough data points yet
			continue
		}

		// cumulKeyN >= unit
		for cumulKeyN > maxKey {
			maxKey += unit
			rm[maxKey] = ts
		}
	}

	// fill-in empty rows
	for i := maxKey; i < int64(totalRequests); i += unit {
		if _, ok := rm[i]; !ok {
			rm[i] = CumulativeKeyNumAndOtherData{}
		}
	}
	if _, ok := rm[int64(totalRequests)]; !ok {
		rm[int64(totalRequests)] = CumulativeKeyNumAndOtherData{}
	}

	kss := []CumulativeKeyNumAndOtherData{}
	delete(rm, 0) // drop data at beginning

	for k, v := range rm {
		// make sure to use 'k' as keyNum
		kss = append(kss, CumulativeKeyNumAndOtherData{
			CumulativeKeyNum:   k,
			MinMemoryMB:        v.MinMemoryMB,
			AvgMemoryMB:        v.AvgMemoryMB,
			MaxMemoryMB:        v.MaxMemoryMB,
			AvgReadBytesDelta:  v.AvgReadBytesDelta,
			AvgWriteBytesDelta: v.AvgWriteBytesDelta,
		})
	}

	// sort by cumulative throughput (number of keys) in ascending order
	sort.Sort(CumulativeKeyNumAndOtherDataSlice(kss))
	return kss
}
