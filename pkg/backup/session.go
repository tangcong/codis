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
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	//"github.com/CodisLabs/codis/pkg/utils/math2"
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

	writer     io.WriteCloser
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

func NewSession(sock net.Conn, config *Config, writer io.WriteCloser) *Session {
	log.Warnf("recv buf size:%d,send buf size:%d\n", config.SessionRecvBufsize.Int(), config.SessionSendBufsize.Int())
	c := NewConn(sock,
		config.SessionRecvBufsize.Int(),
		config.SessionSendBufsize.Int(),
	)
	c.ReaderTimeout = config.SessionRecvTimeout.Get()
	c.WriterTimeout = config.SessionSendTimeout.Get()
	c.SetKeepAlivePeriod(config.SessionKeepAlivePeriod.Get())

	s := &Session{
		Conn: c, config: config,
		CreateUnix: time.Now().Unix(),
		writer:     writer,
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

var (
	ErrTooManySessions = errors.New("too many sessions")
	ErrRouterNotOnline = errors.New("router is not online")

	ErrTooManyPipelinedRequests = errors.New("too many pipelined requests")
)

func (s *Session) Start() {
	s.start.Do(func() {

		//reqs := make(chan *WriteRequest, math2.MaxInt(1, s.config.SessionMaxPipeline))

		go func() {
			//s.loopReader(reqs)
			s.loopReader()
			//close(reqs)
		}()
	})
}

//func (s *Session) loopReader(reqs chan<- *WriteRequest) (err error) {
func (s *Session) loopReader() (err error) {
	defer func() {
		s.CloseReaderWithError(err)
	}()

	for !s.quit {
		r, err := s.Conn.DecodeReq()
		if err != nil {
			log.Warnf("decode req failed,%s", err)
			return err
		}
		log.Warnf("recv msg,product name is %s\n", *r.ProductName)
		fmt.Fprintf(s.writer, "%d\t%s\t%d\n", time.Now().Unix(), *r.ProductName, len(r.MultiCmd))
		for _, cmd := range r.MultiCmd {
			fmt.Fprintf(s.writer, "%s\n", cmd)
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
