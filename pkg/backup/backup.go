// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package backup

import (
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/proxy"
	"github.com/CodisLabs/codis/pkg/utils"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/CodisLabs/codis/pkg/utils/unsafe2"
)

type Backup struct {
	mu sync.Mutex

	model *models.Backup

	exit struct {
		C chan struct{}
	}
	online bool
	closed bool

	config *Config
	ignore []byte

	lbackup net.Listener
	ladmin  net.Listener
	writer  io.WriteCloser
}

var ErrClosedBackup = errors.New("use of closed backup")

func New(config *Config) (*Backup, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Trace(err)
	}

	s := &Backup{}
	s.config = config
	s.exit.C = make(chan struct{})
	s.ignore = make([]byte, config.BackupHeapPlaceholder.Int())

	s.model = &models.Backup{
		StartTime: time.Now().String(),
	}
	//s.model.DataCenter = config.BackupDataCenter
	s.model.Pid = os.Getpid()
	s.model.Pwd, _ = os.Getwd()
	if b, err := exec.Command("uname", "-a").Output(); err != nil {
		log.WarnErrorf(err, "run command uname failed")
	} else {
		s.model.Sys = strings.TrimSpace(string(b))
	}
	s.model.Hostname = utils.Hostname

	if writer, err := log.NewRollingFile(config.DataPath, log.HourlyRolling); err != nil {

		log.Panicf("backup open roll file failed:%s", config.DataPath)
	} else {

		s.writer = writer
	}

	if err := s.setup(config); err != nil {
		s.Close()
		return nil, err
	}

	log.Warnf("[%p] create new backup:\n%s", s, s.model.Encode())

	unsafe2.SetMaxOffheapBytes(config.BackupMaxOffheapBytes.Int())

	//go s.serveAdmin()
	go s.serveBackup()

	return s, nil
}

func (s *Backup) setup(config *Config) error {
	proto := config.ProtoType
	if l, err := net.Listen(proto, config.BackupAddr); err != nil {
		return errors.Trace(err)
	} else {
		s.lbackup = l

		x, err := utils.ReplaceUnspecifiedIP(proto, l.Addr().String(), config.HostBackup)
		if err != nil {
			return err
		}
		s.model.ProtoType = proto
		s.model.BackupAddr = x
	}

	return nil
}

func (s *Backup) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedBackup
	}
	if s.online {
		return nil
	}
	s.online = true
	return nil
}

func (s *Backup) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.exit.C)

	if s.ladmin != nil {
		s.ladmin.Close()
	}
	if s.lbackup != nil {
		s.lbackup.Close()
	}

	s.writer.Close()

	return nil
}

func (s *Backup) Model() *models.Backup {
	return s.model
}

func (s *Backup) Config() *Config {
	return s.config
}

func (s *Backup) IsOnline() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.online && !s.closed
}

func (s *Backup) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Backup) serveBackup() {
	if s.IsClosed() {
		return
	}
	defer s.Close()

	log.Warnf("[%p] backup start service on %s", s, s.lbackup.Addr())

	eh := make(chan error, 1)
	go func(l net.Listener) (err error) {
		defer func() {
			eh <- err
		}()
		for {
			c, err := s.acceptConn(l)
			if err != nil {
				return err
			}
			NewSession(c, s.config, s.writer).Start()
		}
	}(s.lbackup)

	select {
	case <-s.exit.C:
		log.Warnf("[%p] backup shutdown", s)
	case err := <-eh:
		log.ErrorErrorf(err, "[%p] backup exit on error", s)
	}
}

func (s *Backup) acceptConn(l net.Listener) (net.Conn, error) {
	var delay = &proxy.DelayExp2{
		Min: 10, Max: 500,
		Unit: time.Millisecond,
	}
	for {
		c, err := l.Accept()
		if err != nil {
			if e, ok := err.(net.Error); ok && e.Temporary() {
				log.WarnErrorf(err, "[%p] backup accept new connection failed", s)
				delay.Sleep()
				continue
			}
		}
		return c, err
	}
}
