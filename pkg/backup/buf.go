// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package backup

import (
	"errors"
	"fmt"
	"github.com/CodisLabs/codis/pkg/utils"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"io"
	"sync"
	"time"
)

const (
	ArraySize          = 2
	FlushBufThresholds = 409600
	FlushBufInterval   = 60
)

var wBuf struct {
	sync.Mutex
	buf                []byte
	array              [ArraySize][]byte
	pos                int
	lastPos            int
	bufsize            int
	index              uint64
	lastIndex          uint64
	lastFlushFileStamp int64
	writer             io.Writer
}

var (
	ErrBufFull      = errors.New("Backup Write Buf Is Full,Write Speed Is Too Fast!")
	ErrNotMatchCond = errors.New("Not Match Bufsize Or Time Interval Condition!")
)

func setBufsize(s int) {
	log.Warnf("backup buf size is %d\n", s)
	wBuf.bufsize = s
	for i := 0; i < ArraySize; i++ {
		wBuf.array[i] = make([]byte, wBuf.bufsize)
	}
	wBuf.buf = wBuf.array[wBuf.index&1]
}

func AppendBuf(buf []byte) error {
	wBuf.Lock()
	defer wBuf.Unlock()
	log.Debugf("wbuf pos is %d,append buf size:%d\n", wBuf.pos, len(buf))
	if wBuf.pos+len(buf) >= wBuf.bufsize {
		return ErrBufFull
	}
	wBuf.pos += copy(wBuf.buf[wBuf.pos:], buf)
	return nil
}

func SwitchBuf() error {
	wBuf.Lock()
	defer wBuf.Unlock()
	now := time.Now().Unix()
	if wBuf.pos >= FlushBufThresholds || (now-wBuf.lastFlushFileStamp >= FlushBufInterval && wBuf.pos > 0) {
		wBuf.lastIndex = wBuf.index
		wBuf.lastPos = wBuf.pos
		wBuf.index++
		wBuf.buf = wBuf.array[wBuf.index&1]
		wBuf.pos = 0
		return nil
	} else {
		return ErrNotMatchCond
	}
}

func AsyncFlushFile(config *Config) {
	log.Warnf("Flush Buf Routine Start!")
	setBufsize(config.BackupBufsize)
	ok, err := utils.IsDirExist(config.DataDir)
	if err != nil {
		log.Panicf("backup check dir exist failed:%s", err)
	} else if !ok {
		err = utils.MkDir(config.DataDir)
		if err != nil {
			log.Panicf("backup mk dir failed:%s", err)
		}
	}
	file := config.DataDir + "/" + config.ProductName
	if wBuf.writer, err = log.NewRollingFile(file, log.HourlyRolling); err != nil {
		log.Panicf("backup open roll file failed:%s", file)
	}
	tick := time.Tick(10 * time.Millisecond)
	for {
		select {
		case <-tick:
			err = SwitchBuf()
			if err == nil {
				wBuf.lastFlushFileStamp = time.Now().Unix()
				log.Debugf("wbuf index is %d,last pos is:%d\n", wBuf.lastIndex, wBuf.lastPos)
				fmt.Fprintf(wBuf.writer, "%d\n%s\n", wBuf.lastFlushFileStamp, wBuf.array[wBuf.lastIndex&1][:wBuf.lastPos])
			}
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}
