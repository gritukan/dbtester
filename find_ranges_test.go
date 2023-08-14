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
	"reflect"
	"testing"
	"time"

	"github.com/etcd-io/etcd/pkg/report"
)

func TestFindRangesLatency(t *testing.T) {
	var data report.TimeSeries
	for i := int64(0); i < 10; i++ {
		dp := report.DataPoint{
			Timestamp:  i,
			AvgLatency: time.Duration(i + 1),
			ThroughPut: 50,
		}
		data = append(data, dp)
	}

	pss := FindRangesLatency(data, 20, 555)
	expexcted := []CumulativeKeyNumToAvgLatency{
		{CumulativeKeyNum: 20, AvgLatency: 1},
		{CumulativeKeyNum: 40, AvgLatency: 1},
		{CumulativeKeyNum: 60, AvgLatency: 1},
		{CumulativeKeyNum: 80, AvgLatency: 2},
		{CumulativeKeyNum: 100, AvgLatency: 2},
		{CumulativeKeyNum: 120, AvgLatency: 3},
		{CumulativeKeyNum: 140, AvgLatency: 3},
		{CumulativeKeyNum: 160, AvgLatency: 3},
		{CumulativeKeyNum: 180, AvgLatency: 4},
		{CumulativeKeyNum: 200, AvgLatency: 4},
		{CumulativeKeyNum: 220, AvgLatency: 5},
		{CumulativeKeyNum: 240, AvgLatency: 5},
		{CumulativeKeyNum: 260, AvgLatency: 5},
		{CumulativeKeyNum: 280, AvgLatency: 6},
		{CumulativeKeyNum: 300, AvgLatency: 6},
		{CumulativeKeyNum: 320, AvgLatency: 7},
		{CumulativeKeyNum: 340, AvgLatency: 7},
		{CumulativeKeyNum: 360, AvgLatency: 7},
		{CumulativeKeyNum: 380, AvgLatency: 8},
		{CumulativeKeyNum: 400, AvgLatency: 8},
		{CumulativeKeyNum: 420, AvgLatency: 9},
		{CumulativeKeyNum: 440, AvgLatency: 9},
		{CumulativeKeyNum: 460, AvgLatency: 9},
		{CumulativeKeyNum: 480, AvgLatency: 10},
		{CumulativeKeyNum: 500, AvgLatency: 10},
		{CumulativeKeyNum: 520, AvgLatency: 0},
		{CumulativeKeyNum: 540, AvgLatency: 0},
		{CumulativeKeyNum: 555, AvgLatency: 0},
	}
	if len(pss) != len(expexcted) {
		t.Fatalf("expected %+v, got %+v", expexcted, pss)
	}
	for i, elem := range pss {
		if !reflect.DeepEqual(elem, expexcted[i]) {
			t.Fatalf("#%d: processed data point expected %+v, got %+v", i, expexcted[i], elem)
		}
	}
}

func TestFindRangesData(t *testing.T) {
	var data []CumulativeKeyNumAndOtherData
	for i := int64(0); i < 10; i++ {
		dp := CumulativeKeyNumAndOtherData{
			UnixSecond:  i + 1,
			Throughput:  50,
			AvgMemoryMB: float64(i + 1),
		}
		data = append(data, dp)
	}

	pss := FindRangesData(data, 20, 555)
	expexcted := []CumulativeKeyNumAndOtherData{
		{CumulativeKeyNum: 20, AvgMemoryMB: 1},
		{CumulativeKeyNum: 40, AvgMemoryMB: 1},
		{CumulativeKeyNum: 60, AvgMemoryMB: 1},
		{CumulativeKeyNum: 80, AvgMemoryMB: 2},
		{CumulativeKeyNum: 100, AvgMemoryMB: 2},
		{CumulativeKeyNum: 120, AvgMemoryMB: 3},
		{CumulativeKeyNum: 140, AvgMemoryMB: 3},
		{CumulativeKeyNum: 160, AvgMemoryMB: 3},
		{CumulativeKeyNum: 180, AvgMemoryMB: 4},
		{CumulativeKeyNum: 200, AvgMemoryMB: 4},
		{CumulativeKeyNum: 220, AvgMemoryMB: 5},
		{CumulativeKeyNum: 240, AvgMemoryMB: 5},
		{CumulativeKeyNum: 260, AvgMemoryMB: 5},
		{CumulativeKeyNum: 280, AvgMemoryMB: 6},
		{CumulativeKeyNum: 300, AvgMemoryMB: 6},
		{CumulativeKeyNum: 320, AvgMemoryMB: 7},
		{CumulativeKeyNum: 340, AvgMemoryMB: 7},
		{CumulativeKeyNum: 360, AvgMemoryMB: 7},
		{CumulativeKeyNum: 380, AvgMemoryMB: 8},
		{CumulativeKeyNum: 400, AvgMemoryMB: 8},
		{CumulativeKeyNum: 420, AvgMemoryMB: 9},
		{CumulativeKeyNum: 440, AvgMemoryMB: 9},
		{CumulativeKeyNum: 460, AvgMemoryMB: 9},
		{CumulativeKeyNum: 480, AvgMemoryMB: 10},
		{CumulativeKeyNum: 500, AvgMemoryMB: 10},
		{CumulativeKeyNum: 520, AvgMemoryMB: 0},
		{CumulativeKeyNum: 540, AvgMemoryMB: 0},
		{CumulativeKeyNum: 555, AvgMemoryMB: 0},
	}
	if len(pss) != len(expexcted) {
		t.Fatalf("expected %+v, got %+v", expexcted, pss)
	}
	for i, elem := range pss {
		if !reflect.DeepEqual(elem, expexcted[i]) {
			t.Fatalf("#%d: processed data point expected %+v, got %+v", i, expexcted[i], elem)
		}
	}
}
