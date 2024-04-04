package usecase

import (
	"time"

	"github.com/forest33/tapir/business/entity"
)

func (uc *ServerUseCase) addSessionStat(sessionID uint32, stat *entity.Statistic) {
	uc.statCh <- &sessionStatisticRequest{
		ID: sessionID,
		stat: &entity.Statistic{
			IncomingBytes:  stat.IncomingBytes,
			OutgoingBytes:  stat.OutgoingBytes,
			IncomingFrames: stat.IncomingFrames,
			OutgoingFrames: stat.OutgoingFrames,
		},
	}
}

func (uc *ServerUseCase) sessionStat() {
	var (
		stat     = make(map[uint32]*entity.Statistic, len(uc.cfg.Users))
		ticker   = time.NewTicker(time.Duration(uc.cfg.Statistic.Interval) * time.Millisecond)
		interval = (time.Duration(uc.cfg.Statistic.Interval) * time.Millisecond).Seconds()
	)

	go func() {
		for {
			select {
			case s, ok := <-uc.statCh:
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
			case <-ticker.C:
				uc.sessMux.Lock()
				for id := range stat {
					if _, ok := uc.sessions[id]; !ok {
						continue
					}
					uc.sessions[id].Stat.IncomingBytes += stat[id].IncomingBytes
					uc.sessions[id].Stat.OutgoingBytes += stat[id].OutgoingBytes
					uc.sessions[id].Stat.IncomingFrames += stat[id].IncomingFrames
					uc.sessions[id].Stat.OutgoingFrames += stat[id].OutgoingFrames
					uc.sessions[id].Stat.IncomingRateBytes = float64(stat[id].IncomingBytes) / interval
					uc.sessions[id].Stat.OutgoingRateBytes = float64(stat[id].OutgoingBytes) / interval
					uc.sessions[id].Stat.IncomingRateFrames = float64(stat[id].IncomingFrames) / interval
					uc.sessions[id].Stat.OutgoingRateFrames = float64(stat[id].OutgoingFrames) / interval
				}
				uc.sessMux.Unlock()
				clear(stat)
			case <-uc.ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()
}
