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

package analyze

import (
	"fmt"

	"github.com/etcd-io/dbtester/dbtesterpb"

	"github.com/gyuho/dataframe"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

var (
	plotWidth  = 12 * vg.Inch
	plotHeight = 8 * vg.Inch
)

func init() {
	//plot.DefaultFont = "Helvetica"
	plotter.DefaultLineStyle.Width = vg.Points(1.5)
	plotter.DefaultGlyphStyle.Radius = vg.Points(2.0)
}

type pair struct {
	x dataframe.Column
	y dataframe.Column
}

type triplet struct {
	x      dataframe.Column
	minCol dataframe.Column
	avgCol dataframe.Column
	maxCol dataframe.Column
}

func (all *allAggregatedData) draw(cfg dbtesterpb.ConfigAnalyzeMachinePlot, pairs ...pair) error {
	// frame now contains
	// AVG-LATENCY-MS-etcd-v3.1-go1.7.4, AVG-LATENCY-MS-zookeeper-r3.4.9-java8, AVG-LATENCY-MS-consul-v0.7.2-go1.7.4
	plt := plot.New()
	plt.Title.Text = fmt.Sprintf("%s, %s", all.title, cfg.YAxis)
	plt.X.Label.Text = cfg.XAxis
	plt.Y.Label.Text = cfg.YAxis
	plt.Legend.Top = true

	var ps []plot.Plotter
	for i, p := range pairs {
		pt, err := points(p.y)
		if err != nil {
			return err
		}

		l, err := plotter.NewLine(pt)
		if err != nil {
			return err
		}
		l.Color = dbtesterpb.GetRGBI(all.headerToDatabaseID[p.y.Header()], i)
		l.Dashes = plotutil.Dashes(i)
		ps = append(ps, l)

		plt.Legend.Add(all.headerToDatabaseDescription[p.y.Header()], l)
	}
	plt.Add(ps...)

	for _, outputPath := range cfg.OutputPathList {
		if err := plt.Save(plotWidth, plotHeight, outputPath); err != nil {
			return err
		}
	}
	return nil
}

func (all *allAggregatedData) drawXY(cfg dbtesterpb.ConfigAnalyzeMachinePlot, pairs ...pair) error {
	// frame now contains
	// KEYS-DB-TAG-X, AVG-LATENCY-MS-DB-TAG-Y, ...
	plt := plot.New()
	plt.Title.Text = fmt.Sprintf("%s, %s", all.title, cfg.YAxis)
	plt.X.Label.Text = cfg.XAxis
	plt.Y.Label.Text = cfg.YAxis
	plt.Legend.Top = true

	var ps []plot.Plotter
	for i, p := range pairs {
		pt, err := pointsXY(p.x, p.y)
		if err != nil {
			return err
		}

		l, err := plotter.NewLine(pt)
		if err != nil {
			return err
		}
		l.Color = dbtesterpb.GetRGBI(all.headerToDatabaseID[p.y.Header()], i)
		l.Dashes = plotutil.Dashes(i)
		ps = append(ps, l)

		plt.Legend.Add(all.headerToDatabaseDescription[p.y.Header()], l)
	}
	plt.Add(ps...)

	for _, outputPath := range cfg.OutputPathList {
		if err := plt.Save(plotWidth, plotHeight, outputPath); err != nil {
			return err
		}
	}
	return nil
}

func (all *allAggregatedData) drawXYWithErrorPoints(cfg dbtesterpb.ConfigAnalyzeMachinePlot, triplets ...triplet) error {
	// frame now contains
	// KEYS-DB-TAG-X, MIN-LATENCY-MS-DB-TAG-Y, AVG-LATENCY-MS-DB-TAG-Y, MAX-LATENCY-MS-DB-TAG-Y, ...
	plt := plot.New()
	plt.Title.Text = fmt.Sprintf("%s, %s", all.title, cfg.YAxis)
	plt.X.Label.Text = cfg.XAxis
	plt.Y.Label.Text = cfg.YAxis
	plt.Legend.Top = true

	var ps []plot.Plotter
	for i, triplet := range triplets {
		{
			pt, err := pointsXY(triplet.x, triplet.minCol)
			if err != nil {
				return err
			}
			l, err := plotter.NewLine(pt)
			if err != nil {
				return err
			}
			l.Color = dbtesterpb.GetRGBII(all.headerToDatabaseID[triplet.avgCol.Header()], i)
			l.Dashes = plotutil.Dashes(i)
			ps = append(ps, l)
			plt.Legend.Add(all.headerToDatabaseDescription[triplet.avgCol.Header()]+" MIN", l)
		}
		{
			pt, err := pointsXY(triplet.x, triplet.avgCol)
			if err != nil {
				return err
			}
			l, err := plotter.NewLine(pt)
			if err != nil {
				return err
			}
			l.Color = dbtesterpb.GetRGBI(all.headerToDatabaseID[triplet.avgCol.Header()], i)
			l.Dashes = plotutil.Dashes(i)
			ps = append(ps, l)
			plt.Legend.Add(all.headerToDatabaseDescription[triplet.avgCol.Header()], l)
		}
		{
			pt, err := pointsXY(triplet.x, triplet.maxCol)
			if err != nil {
				return err
			}
			l, err := plotter.NewLine(pt)
			if err != nil {
				return err
			}
			l.Color = dbtesterpb.GetRGBIII(all.headerToDatabaseID[triplet.avgCol.Header()], i)
			l.Dashes = plotutil.Dashes(i)
			ps = append(ps, l)
			plt.Legend.Add(all.headerToDatabaseDescription[triplet.avgCol.Header()]+" MAX", l)
		}
	}
	plt.Add(ps...)

	for _, outputPath := range cfg.OutputPathList {
		if err := plt.Save(plotWidth, plotHeight, outputPath); err != nil {
			return err
		}
	}
	return nil
}

func points(col dataframe.Column) (plotter.XYs, error) {
	bv, ok := col.BackNonNil()
	if !ok {
		return nil, fmt.Errorf("BackNonNil not found")
	}
	rowN, ok := col.FindLast(bv)
	if !ok {
		return nil, fmt.Errorf("not found %v", bv)
	}
	pts := make(plotter.XYs, rowN)
	for i := range pts {
		v, err := col.Value(i)
		if err != nil {
			return nil, err
		}
		n, _ := v.Float64()
		pts[i].X = float64(i)
		pts[i].Y = n
	}
	return pts, nil
}

func pointsXY(colX, colY dataframe.Column) (plotter.XYs, error) {
	bv, ok := colX.BackNonNil()
	if !ok {
		return nil, fmt.Errorf("BackNonNil not found")
	}
	rowN, ok := colX.FindLast(bv)
	if !ok {
		return nil, fmt.Errorf("not found %v", bv)
	}
	pts := make(plotter.XYs, rowN)
	for i := range pts {
		vx, err := colX.Value(i)
		if err != nil {
			return nil, err
		}
		x, _ := vx.Float64()

		vy, err := colY.Value(i)
		if err != nil {
			return nil, err
		}
		y, _ := vy.Float64()

		pts[i].X = x
		pts[i].Y = y
	}
	return pts, nil
}
