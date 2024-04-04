package main

import (
	"bufio"
	"encoding/json"
	"os"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
)

func chartRetry(logFile, outFile string) error {
	f, err := os.Open(logFile)
	if err != nil {
		return err
	}
	defer func() {
		if err := f.Close(); err != nil {
			zlog.Error().Err(err).Msg("failed to close log file")
		}
	}()

	times := make([]time.Time, 0, initialEventsCount)
	rtt := make([]opts.LineData, 0, initialEventsCount)
	rto := make([]opts.LineData, 0, initialEventsCount)
	srtt := make([]opts.LineData, 0, initialEventsCount)
	rttvar := make([]opts.LineData, 0, initialEventsCount)

	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		ev := &LogEntity{}
		err := json.Unmarshal(scanner.Bytes(), ev)
		if err != nil {
			zlog.Error().Err(err).Msg("failed to unmarshal log line")
			continue
		}

		if ev.Message != "timer stopped" {
			continue
		}

		t, err := strToDateTime(ev.Time)
		if err != nil {
			zlog.Error().Err(err).Str("time", ev.Time).Msg("failed to parse time from log line")
			continue
		}
		times = append(times, t)

		rtt = append(rtt, opts.LineData{Value: float64(ev.Rtt) / 1_000_000_000})
		srtt = append(srtt, opts.LineData{Value: float64(ev.Srtt) / 1_000_000_000})
		rttvar = append(rttvar, opts.LineData{Value: float64(ev.Rttvar) / 1_000_000_000})
		rto = append(rto, opts.LineData{Value: ev.Rto})
	}

	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  types.ThemeWesteros,
			Width:  chartWidth,
			Height: chartHeight,
		}),
		charts.WithTitleOpts(opts.Title{
			Title: "RTT & RTO",
		}),
	)

	line.SetXAxis(times).
		AddSeries("RTT", rtt).
		AddSeries("SRTT", srtt).
		AddSeries("RTTVAR", rttvar).
		AddSeries("RTO", rto).
		SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{Smooth: true}))

	fo, err := os.Create(outFile)
	if err != nil {
		return err
	}

	line.PageTitle = "RTT & RTO"

	return line.Render(fo)
}
