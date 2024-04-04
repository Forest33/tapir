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

func chartAcknowledgement(logFile, outFile string) error {
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
	ackReceived := make([]opts.LineData, 0, initialEventsCount)
	ackSent := make([]opts.LineData, 0, initialEventsCount)

	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		ev := &LogEntity{}
		err := json.Unmarshal(scanner.Bytes(), ev)
		if err != nil {
			zlog.Error().Err(err).Msg("failed to unmarshal log line")
			continue
		}

		if !ev.Ack || (ev.Message != "sent to socket" && ev.Message != "received from socket") {
			continue
		}

		if ev.Message == "sent to socket" {
			ackSent = append(ackSent, opts.LineData{Value: ev.AckSize})
		} else if ev.Message == "received from socket" {
			ackReceived = append(ackReceived, opts.LineData{Value: ev.AckSize})

			t, err := strToDateTime(ev.Time)
			if err != nil {
				zlog.Error().Err(err).Str("time", ev.Time).Msg("failed to parse time from log line")
				continue
			}
			times = append(times, t)
		}
	}

	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  types.ThemeWesteros,
			Width:  chartWidth,
			Height: chartHeight,
		}),
		charts.WithTitleOpts(opts.Title{
			Title: "Acknowledgements",
		}))

	line.SetXAxis(times).
		AddSeries("Size of received acknowledgement block", ackReceived).
		AddSeries("Size of sent acknowledgement block", ackSent).
		SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{Smooth: true}))

	fo, err := os.Create(outFile)
	if err != nil {
		return err
	}

	return line.Render(fo)
}
