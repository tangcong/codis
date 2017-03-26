// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package backup

import (
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/CodisLabs/codis/pkg/utils/sync2/atomic2"
	"github.com/golang/protobuf/proto"
)

type Session struct {
	Conn *Conn
	Ops  int64

	CreateUnix int64
	LastOpUnix int64

	database int32

	quit bool
	exit sync.Once

	start sync.Once

	broken atomic2.Bool
	config *Config

	authorized bool
}

func (s *Session) String() string {
	o := &struct {
		Ops        int64  `json:"ops"`
		CreateUnix int64  `json:"create"`
		LastOpUnix int64  `json:"lastop,omitempty"`
		RemoteAddr string `json:"remote"`
	}{
		s.Ops, s.CreateUnix, s.LastOpUnix,
		s.Conn.RemoteAddr(),
	}
	b, _ := json.Marshal(o)
	return string(b)
}

func NewSession(sock net.Conn, config *Config) *Session {
	c := NewConn(sock,
		config.SessionRecvBufsize.AsInt(),
		config.SessionSendBufsize.AsInt(),
	)
	c.ReaderTimeout = config.SessionRecvTimeout.Duration()
	c.WriterTimeout = config.SessionSendTimeout.Duration()
	c.SetKeepAlivePeriod(config.SessionKeepAlivePeriod.Duration())

	s := &Session{
		Conn: c, config: config,
		CreateUnix: time.Now().Unix(),
	}
	log.Infof("session [%p] create: %s", s, s)
	return s
}

func (s *Session) CloseReaderWithError(err error) error {
	s.exit.Do(func() {
		if err != nil {
			log.Infof("session [%p] closed: %s, error: %s", s, s, err)
		} else {
			log.Infof("session [%p] closed: %s, quit", s, s)
		}
	})
	return s.Conn.CloseReader()
}

func (s *Session) CloseWithError(err error) error {
	s.exit.Do(func() {
		if err != nil {
			log.Infof("session [%p] closed: %s, error: %s", s, s, err)
		} else {
			log.Infof("session [%p] closed: %s, quit", s, s)
		}
	})
	s.broken.Set(true)
	return s.Conn.Close()
}

func (s *Session) Start() {
	s.start.Do(func() {

		go func() {
			s.loopReader()
		}()
	})
}

func (s *Session) loopReader() (err error) {
	defer func() {
		s.CloseReaderWithError(err)
	}()

	for !s.quit {
		r, err := s.Conn.DecodeReq()
		if err != nil {
			log.Warnf("decode req failed,%s", err)
			incrOpWriteFail(1)
			return err
		}
		index := int(r.GetUsedIndex())
		cmds := int(r.GetCmds())
		err = AppendBuf(r.MultiCmd[:index])
		if err != nil {
			incrOpWriteFail(int64(cmds))
		} else {
			incrOpWriteSucc(int64(cmds))
		}
		s.handleResponse(r)
	}
	return nil
}

func (s *Session) handleQuit(r *WriteRequest) error {
	s.quit = true
	return nil
}

func (s *Session) handleResponse(r *WriteRequest) error {
	rsp := &WriteResponse{}
	rsp.Cmds = proto.Int32(r.GetCmds())
	rsp.Status = WriteResponse_WritingStatus(WriteResponse_OK).Enum()
	p := s.Conn.FlushEncoder()
	p.MaxInterval = time.Millisecond
	p.MaxBuffered = 256

	if err := p.EncodeRes(rsp); err != nil {
		log.Warnf("encode failed:%v", rsp)
	}
	if err := p.Flush(true); err != nil {
		log.Warnf("flush failed:%v", rsp)
	}
	return nil
}
