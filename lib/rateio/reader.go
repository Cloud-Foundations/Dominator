package rateio

import (
	"fmt"
	"io"
	"time"

	"github.com/Cloud-Foundations/Dominator/lib/format"
)

const (
	DEFAULT_SPEED_PERCENT = 2
)

func newReaderContext(maxIOPerSecond uint64, speedPercent uint64,
	measurer ReadIOMeasurer) *ReaderContext {
	var ctx ReaderContext
	ctx.maxIOPerSecond = maxIOPerSecond
	if speedPercent < 1 {
		speedPercent = DEFAULT_SPEED_PERCENT
	}
	ctx.speedPercent = speedPercent
	ctx.chunklen = ctx.maxIOPerSecond * ctx.speedPercent / 10000
	ctx.measurer = measurer
	ctx.timeOfLastPause = time.Now()
	measurer.Reset()
	return &ctx
}

func (ctx *ReaderContext) disableLimits(disable bool) {
	ctx.disabled = disable
	ctx.measurer.Reset()
}

func (ctx *ReaderContext) initialiseMaximumSpeed(maxSpeed uint64) {
	if ctx.maxIOPerSecond > 0 {
		fmt.Println("Maximum speed already set")
	}
	ctx.maxIOPerSecond = maxSpeed
	ctx.chunklen = ctx.maxIOPerSecond * ctx.speedPercent / 10000
}

func (ctx *ReaderContext) setSpeedPercent(percent uint) {
	if percent > 100 {
		percent = 100
	}
	ctx.speedPercent = uint64(percent)
	ctx.chunklen = ctx.maxIOPerSecond * ctx.speedPercent / 10000
	ctx.timeOfLastPause = time.Now()
	ctx.measurer.Reset()
}

func (ctx *ReaderContext) newReader(rd io.Reader) *Reader {
	var reader Reader
	reader.ctx = ctx
	reader.rd = rd
	return &reader
}

func (ctx *ReaderContext) format() string {
	return fmt.Sprintf("max speed=%s/s limit=%d%% %s/s",
		format.FormatBytes(ctx.maxIOPerSecond),
		ctx.speedPercent,
		format.FormatBytes(ctx.maxIOPerSecond*ctx.speedPercent/100))
}

func (rd *Reader) read(b []byte) (n int, err error) {
	if rd.ctx.disabled { // Limits disabled: go at maximum speed.
		return rd.rd.Read(b)
	}
	if rd.ctx.maxIOPerSecond < 1 { // Unspecified capacity: go at maximum speed.
		return rd.rd.Read(b)
	}
	speedPercent := rd.ctx.speedPercent
	if speedPercent >= 100 {
		// Operate at maximum speed: get out of the way.
		return rd.rd.Read(b)
	}
	if rd.ctx.bytesSinceLastPause >= rd.ctx.chunklen {
		// Need to slow down.
		desiredPerSecond := rd.ctx.maxIOPerSecond * speedPercent / 100
		if desiredPerSecond < 1 {
			desiredPerSecond = rd.ctx.maxIOPerSecond / 1000
		}
		if desiredPerSecond < 1 {
			desiredPerSecond = 1
		}
		readSinceLastPause, err := rd.ctx.measurer.MeasureReadIO(
			rd.ctx.bytesSinceLastPause)
		if err != nil {
			return 0, err
		}
		desiredDuration := time.Duration(uint64(time.Second) *
			readSinceLastPause / desiredPerSecond)
		targetTime := rd.ctx.timeOfLastPause.Add(desiredDuration)
		rd.ctx.timeOfLastPause = time.Now()
		duration := targetTime.Sub(time.Now())
		if duration > 0 {
			if rd.ctx.sleepTimeDistribution != nil {
				rd.ctx.sleepTimeDistribution.Add(duration)
			}
			// Interrupt sleep if configuration speed increased so that it takes
			// effect quickly.
			for sleepTime := time.Second; duration > 0; duration -= sleepTime {
				if rd.ctx.disabled || rd.ctx.speedPercent > speedPercent {
					break
				}
				sleepTime := duration
				if sleepTime > time.Second {
					sleepTime = time.Second
				}
				time.Sleep(sleepTime)
			}
		}
		rd.ctx.bytesSinceLastPause = 0
	}
	n, err = rd.rd.Read(b)
	if n < 1 || err != nil {
		return
	}
	rd.ctx.bytesSinceLastPause += uint64(n)
	return
}
