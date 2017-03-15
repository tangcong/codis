// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package backup

import (
	"encoding/json"
	"io"
	"net"
	"sync"
	"time"

	"fmt"
	"github.com/CodisLabs/codis/pkg/utils"
	"github.com/CodisLabs/codis/pkg/utils/errors"
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

	mwriter    map[string]io.WriteCloser
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
	s.mwriter = make(map[string]io.WriteCloser)
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

var (
	ErrTooManySessions = errors.New("too many sessions")
)

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
		writer, ok := s.mwriter[*r.ProductName]
		if !ok {
			ok, err = utils.IsDirExist(s.config.DataDir)
			if err != nil {
				log.Panicf("backup check dir exist failed:%s", err)
			} else if !ok {
				err = utils.MkDir(s.config.DataDir)
				if err != nil {
					log.Panicf("backup mk dir failed:%s", err)
				}
			}
			file := s.config.DataDir + "/" + *r.ProductName
			if writer, err = log.NewRollingFile(file, log.HourlyRolling); err != nil {
				log.Panicf("backup open roll file failed:%s", file)
			} else {
				s.mwriter[*r.ProductName] = writer
			}
		}
		fmt.Fprintf(writer, "%d\t%d\n", time.Now().Unix(), len(r.MultiCmd))
		var buf string
		for _, cmd := range r.MultiCmd {
			buf += cmd
		}
		fmt.Fprintf(writer, "%s\n", buf)
		s.handleResponse(r)
		incrOpWriteSucc(int64(len(r.MultiCmd)))
	}
	return nil
}

func (s *Session) handleQuit(r *WriteRequest) error {
	s.quit = true
	return nil
}

func (s *Session) handleResponse(r *WriteRequest) error {
	rsp := &WriteResponse{}
	rsp.Cmds = proto.Int32(int32(len(r.MultiCmd)))
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
