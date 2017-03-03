// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package backup

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/CodisLabs/codis/pkg/proxy"
	"github.com/CodisLabs/codis/pkg/proxy/redis"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/CodisLabs/codis/pkg/utils/math2"
	"github.com/CodisLabs/codis/pkg/utils/sync2/atomic2"
)

type Session struct {
	Conn *redis.Conn
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
	c := redis.NewConn(sock,
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

var RespOK = redis.NewString([]byte("OK"))

func (s *Session) Start() {
	s.start.Do(func() {

		tasks := make(chan *proxy.Request, math2.MaxInt(1, s.config.SessionMaxPipeline))

		go func() {
			s.loopReader(tasks)
			close(tasks)
		}()
	})
}

func (s *Session) loopReader(tasks chan<- *proxy.Request) (err error) {
	defer func() {
		s.CloseReaderWithError(err)
	}()

	for !s.quit {
		multi, err := s.Conn.DecodeMultiBulk()
		if err != nil {
			return err
		}

		start := time.Now()
		s.LastOpUnix = start.Unix()
		s.Ops++

		r := &proxy.Request{}
		r.Multi = multi
		r.Start = start.UnixNano()
		r.Batch = &sync.WaitGroup{}
		r.Database = s.database

		if len(tasks) == cap(tasks) {
			return ErrTooManyPipelinedRequests
		}

		fmt.Fprintf(s.writer, "%d\t%d\n", time.Now().Unix(), len(r.Multi))
		for _, argv := range r.Multi {
			fmt.Fprintf(s.writer, "%s\n", argv.Value)
		}

		s.handleResponse(r)
	}
	return nil
}

func (s *Session) handleQuit(r *proxy.Request) error {
	s.quit = true
	r.Resp = RespOK
	return nil
}

func (s *Session) handleResponse(r *proxy.Request) error {
	r.Resp = RespOK
	p := s.Conn.FlushEncoder()
	p.MaxInterval = time.Millisecond
	p.MaxBuffered = 256

	if err := p.Encode(r.Resp); err != nil {
		log.Warnf("encode failed:%v", r.Resp)
	}
	if err := p.Flush(true); err != nil {
		log.Warnf("flush failed:%v", r.Resp)
	}
	return nil
}
