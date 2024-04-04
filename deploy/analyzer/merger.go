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

func chartMerger(logFile, outFile string) error {
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
	wlSize := make([]opts.LineData, 0, initialEventsCount)
	ttl := make([]opts.LineData, 0, initialEventsCount)

	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		ev := &LogEntity{}
		err := json.Unmarshal(scanner.Bytes(), ev)
		if err != nil {
			zlog.Error().Err(err).Msg("failed to unmarshal log line")
			continue
		}

		if ev.Message != "received by stream merger" {
			continue
		}

		t, err := strToDateTime(ev.Time)
		if err != nil {
			zlog.Error().Err(err).Str("time", ev.Time).Msg("failed to parse time from log line")
			continue
		}
		times = append(times, t)

		wlSize = append(wlSize, opts.LineData{Value: ev.WaitingListSize})
		ttl = append(ttl, opts.LineData{Value: ev.TTL})
	}

	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  types.ThemeWesteros,
			Width:  chartWidth,
			Height: chartHeight,
		}),
		charts.WithTitleOpts(opts.Title{
			Title: "Merger",
		}))

	line.SetXAxis(times).
		AddSeries("Size of waiting list", wlSize).
		AddSeries("TTL of waiting list", ttl).
		SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{Smooth: true}))

	fo, err := os.Create(outFile)
	if err != nil {
		return err
	}

	return line.Render(fo)
}
