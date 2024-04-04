package usecase

import (
	"sync/atomic"
	"time"

	"github.com/forest33/tapir/business/entity"
)

func (uc *ConnectionManagerUseCase) createStatisticHandler(connID int) entity.StatisticHandler {
	return func(_ uint32, stat *entity.Statistic) {
		uc.inStatCh <- &connectionStatisticRequest{
			ID: connID,
			stat: &entity.Statistic{
				IncomingBytes:  stat.IncomingBytes,
				OutgoingBytes:  stat.OutgoingBytes,
				IncomingFrames: stat.IncomingFrames,
				OutgoingFrames: stat.OutgoingFrames,
			},
		}
	}
}

func (uc *ConnectionManagerUseCase) sessionStat() {
	var (
		stat     = make(map[int]*entity.Statistic, len(uc.cfg.Connections))
		ticker   = time.NewTicker(time.Duration(uc.cfg.Statistic.Interval) * time.Millisecond)
		interval = (time.Duration(uc.cfg.Statistic.Interval) * time.Millisecond).Seconds()
		updated  atomic.Bool
	)

	go func() {
		var init bool
		for {
			select {
			case s, ok := <-uc.inStatCh:
				if !ok {
					return
				}
				if _, ok := stat[s.ID]; !ok {
					stat[s.ID] = &entity.Statistic{}
				}
				stat[s.ID].IncomingBytes += s.stat.IncomingBytes
				stat[s.ID].OutgoingBytes += s.stat.OutgoingBytes
				stat[s.ID].IncomingFrames += s.stat.IncomingFrames
				stat[s.ID].OutgoingFrames += s.stat.OutgoingFrames
				updated.Store(true)
			case <-ticker.C:
				if !updated.Load() {
					if init {
						continue
					}
					init = true
				} else {
					init = false
				}
				uc.statMux.Lock()
				for id := range uc.statistic {
					uc.statistic[id].IncomingRateBytes = 0
					uc.statistic[id].OutgoingRateBytes = 0
					uc.statistic[id].IncomingRateFrames = 0
					uc.statistic[id].OutgoingRateFrames = 0
				}
				for id := range stat {
					if _, ok := uc.statistic[id]; !ok {
						uc.statistic[id] = &entity.Statistic{}
					}
					uc.statistic[id].IncomingBytes += stat[id].IncomingBytes
					uc.statistic[id].OutgoingBytes += stat[id].OutgoingBytes
					uc.statistic[id].IncomingFrames += stat[id].IncomingFrames
					uc.statistic[id].OutgoingFrames += stat[id].OutgoingFrames
					uc.statistic[id].IncomingRateBytes = float64(stat[id].IncomingBytes) / interval
					uc.statistic[id].OutgoingRateBytes = float64(stat[id].OutgoingBytes) / interval
					uc.statistic[id].IncomingRateFrames = float64(stat[id].IncomingFrames) / interval
					uc.statistic[id].OutgoingRateFrames = float64(stat[id].OutgoingFrames) / interval
				}
				if uc.outStatCh != nil {
					uc.outStatCh <- uc.statistic
				}
				uc.statMux.Unlock()
				clear(stat)
				updated.Store(false)
			case <-uc.ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}
