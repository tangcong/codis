// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package proxy

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/CodisLabs/codis/pkg/backup"
	"github.com/CodisLabs/codis/pkg/models"
	redisapi "github.com/CodisLabs/codis/pkg/proxy/redis"
	"github.com/CodisLabs/codis/pkg/utils"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/CodisLabs/codis/pkg/utils/math2"
	"github.com/CodisLabs/codis/pkg/utils/redis"
	"github.com/CodisLabs/codis/pkg/utils/rpc"
	"github.com/CodisLabs/codis/pkg/utils/sync2/atomic2"
	"github.com/CodisLabs/codis/pkg/utils/unsafe2"
	"github.com/golang/protobuf/proto"
)

type Proxy struct {
	mu sync.Mutex

	token string
	xauth string
	model *models.Proxy

	exit struct {
		C chan struct{}
	}
	online bool
	closed bool

	config *Config
	router *Router
	ignore []byte

	lproxy net.Listener
	ladmin net.Listener

	cmds          chan *Request
	rsps          chan int32
	backupReq     *backup.WriteRequest
	backupEncoder *backup.FlushEncoder
	redisEncoder  *redisapi.Encoder
	conn          *backup.Conn
	cmdBufsize    int
	curCmdNum     *int32
	broken        atomic2.Bool

	ha struct {
		monitor *redis.Sentinel
		masters map[int]string
		servers []string
	}
	jodis *Jodis
}

var ErrClosedProxy = errors.New("use of closed proxy")

func New(config *Config) (*Proxy, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Trace(err)
	}
	if err := models.ValidateProduct(config.ProductName); err != nil {
		return nil, errors.Trace(err)
	}

	s := &Proxy{}
	s.config = config
	s.exit.C = make(chan struct{})
	s.router = NewRouter(config)
	//s.ignore = make([]byte, config.ProxyHeapPlaceholder.Int64())

	s.model = &models.Proxy{
		StartTime: time.Now().String(),
	}
	s.model.ProductName = config.ProductName
	s.model.DataCenter = config.ProxyDataCenter
	s.model.Pid = os.Getpid()
	s.model.Pwd, _ = os.Getwd()
	if b, err := exec.Command("uname", "-a").Output(); err != nil {
		log.WarnErrorf(err, "run command uname failed")
	} else {
		s.model.Sys = strings.TrimSpace(string(b))
	}
	s.model.Hostname = utils.Hostname
	s.cmds = make(chan *Request, s.config.WriteReqBufsize)
	s.rsps = make(chan int32, s.config.WriteReqBufsize)
	s.backupReq = &backup.WriteRequest{ProductName: &s.config.ProductName}
	s.cmdBufsize = s.config.BackupMaxReq * s.config.RedisBufsize.AsInt()
	s.backupEncoder = nil
	s.conn = nil
	s.curCmdNum = proto.Int32(0)
	s.redisEncoder = redisapi.NewEncoderSize(s, s.cmdBufsize)
	s.broken.Set(true)

	if err := s.setup(config); err != nil {
		s.Close()
		return nil, err
	}

	log.Warnf("[%p] create new proxy:\n%s", s, s.model.Encode())

	unsafe2.SetMaxOffheapBytes(config.ProxyMaxOffheapBytes.Int64())

	go s.serveAdmin()
	go s.serveProxy()
	if !s.config.CloseBackup {
		go s.backupLoopWriter()
		go s.backupLoopReader()
	}

	s.startMetricsJson()
	s.startMetricsInfluxdb()
	s.startMetricsStatsd()

	return s, nil
}

func (s *Proxy) setup(config *Config) error {
	proto := config.ProtoType
	if l, err := net.Listen(proto, config.ProxyAddr); err != nil {
		return errors.Trace(err)
	} else {
		s.lproxy = l

		x, err := utils.ReplaceUnspecifiedIP(proto, l.Addr().String(), config.HostProxy)
		if err != nil {
			return err
		}
		s.model.ProtoType = proto
		s.model.ProxyAddr = x
	}

	proto = "tcp"
	if l, err := net.Listen(proto, config.AdminAddr); err != nil {
		return errors.Trace(err)
	} else {
		s.ladmin = l

		x, err := utils.ReplaceUnspecifiedIP(proto, l.Addr().String(), config.HostAdmin)
		if err != nil {
			return err
		}
		s.model.AdminAddr = x
	}

	s.model.Token = rpc.NewToken(
		config.ProductName,
		s.lproxy.Addr().String(),
		s.ladmin.Addr().String(),
	)
	s.xauth = rpc.NewXAuth(
		config.ProductName,
		config.ProductAuth,
		s.model.Token,
	)

	if config.JodisAddr != "" {
		c, err := models.NewClient(config.JodisName, config.JodisAddr, config.JodisTimeout.Duration())
		if err != nil {
			return err
		}
		if config.JodisCompatible {
			s.model.JodisPath = filepath.Join("/zk/codis", fmt.Sprintf("db_%s", config.ProductName), "proxy", s.model.Token)
		} else {
			s.model.JodisPath = models.JodisPath(config.ProductName, s.model.Token)
		}
		s.jodis = NewJodis(c, s.model)
	}

	return nil
}

func (s *Proxy) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedProxy
	}
	if s.online {
		return nil
	}
	s.online = true
	s.router.Start()
	if s.jodis != nil {
		s.jodis.Start()
	}
	return nil
}

func (s *Proxy) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.exit.C)

	if s.jodis != nil {
		s.jodis.Close()
	}
	if s.ladmin != nil {
		s.ladmin.Close()
	}
	if s.lproxy != nil {
		s.lproxy.Close()
	}
	if s.router != nil {
		s.router.Close()
	}
	if s.ha.monitor != nil {
		s.ha.monitor.Cancel()
	}

	close(s.cmds)
	close(s.rsps)

	return nil
}

func (s *Proxy) XAuth() string {
	return s.xauth
}

func (s *Proxy) Model() *models.Proxy {
	return s.model
}

func (s *Proxy) Config() *Config {
	return s.config
}

func (s *Proxy) IsOnline() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.online && !s.closed
}

func (s *Proxy) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *Proxy) HasSwitched() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.router.HasSwitched()
}

func (s *Proxy) Slots() []*models.Slot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.router.GetSlots()
}

func (s *Proxy) FillSlot(m *models.Slot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedProxy
	}
	return s.router.FillSlot(m)
}

func (s *Proxy) FillSlots(slots []*models.Slot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedProxy
	}
	for _, m := range slots {
		if err := s.router.FillSlot(m); err != nil {
			return err
		}
	}
	return nil
}

func (s *Proxy) SwitchMasters(masters map[int]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedProxy
	}
	s.ha.masters = masters

	if len(masters) != 0 {
		s.router.SwitchMasters(masters)
	}
	return nil
}

func (s *Proxy) GetSentinels() ([]string, map[int]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, nil
	}
	return s.ha.servers, s.ha.masters
}

func (s *Proxy) SetSentinels(servers []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedProxy
	}
	s.ha.servers = servers
	log.Warnf("[%p] set sentinels = %v", s, s.ha.servers)

	s.rewatchSentinels(s.ha.servers)
	return nil
}

func (s *Proxy) RewatchSentinels() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosedProxy
	}
	log.Warnf("[%p] rewatch sentinels = %v", s, s.ha.servers)

	s.rewatchSentinels(s.ha.servers)
	return nil
}

func (s *Proxy) rewatchSentinels(servers []string) {
	if s.ha.monitor != nil {
		s.ha.monitor.Cancel()
		s.ha.monitor = nil
		s.ha.masters = nil
	}
	if len(servers) != 0 {
		s.ha.monitor = redis.NewSentinel(s.config.ProductName, s.config.ProductAuth)
		s.ha.monitor.LogFunc = log.Warnf
		s.ha.monitor.ErrFunc = log.WarnErrorf
		go func(p *redis.Sentinel) {
			var trigger = make(chan struct{}, 1)
			delayUntil := func(deadline time.Time) {
				for !p.IsCanceled() {
					var d = deadline.Sub(time.Now())
					if d <= 0 {
						return
					}
					time.Sleep(math2.MinDuration(d, time.Second))
				}
			}
			go func() {
				defer close(trigger)
				callback := func() {
					select {
					case trigger <- struct{}{}:
					default:
					}
				}
				for !p.IsCanceled() {
					timeout := time.Minute * 15
					retryAt := time.Now().Add(time.Second * 10)
					if !p.Subscribe(servers, timeout, callback) {
						delayUntil(retryAt)
					} else {
						callback()
					}
				}
			}()
			go func() {
				for _ = range trigger {
					var success int
					for i := 0; i != 10 && !p.IsCanceled() && success != 2; i++ {
						timeout := time.Second * 5
						masters, err := p.Masters(servers, timeout)
						if err != nil {
							log.WarnErrorf(err, "[%p] fetch group masters failed", s)
						} else {
							if !p.IsCanceled() {
								s.SwitchMasters(masters)
							}
							success += 1
						}
						delayUntil(time.Now().Add(time.Second * 5))
					}
				}
			}()
		}(s.ha.monitor)
	}
}

func (s *Proxy) serveAdmin() {
	if s.IsClosed() {
		return
	}
	defer s.Close()

	log.Warnf("[%p] admin start service on %s", s, s.ladmin.Addr())

	eh := make(chan error, 1)
	go func(l net.Listener) {
		h := http.NewServeMux()
		h.Handle("/", newApiServer(s))
		hs := &http.Server{Handler: h}
		eh <- hs.Serve(l)
	}(s.ladmin)

	select {
	case <-s.exit.C:
		log.Warnf("[%p] admin shutdown", s)
	case err := <-eh:
		log.ErrorErrorf(err, "[%p] admin exit on error", s)
	}
}

func (s *Proxy) serveProxy() {
	if s.IsClosed() {
		return
	}
	defer s.Close()

	log.Warnf("[%p] proxy start service on %s", s, s.lproxy.Addr())

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
			NewSession(c, s.config, s.cmds).Start(s.router)
		}
	}(s.lproxy)

	if d := s.config.BackendPingPeriod.Duration(); d != 0 {
		go s.keepAlive(d)
	}

	select {
	case <-s.exit.C:
		log.Warnf("[%p] proxy shutdown", s)
	case err := <-eh:
		log.ErrorErrorf(err, "[%p] proxy exit on error", s)
	}
}

func (s *Proxy) keepAlive(d time.Duration) {
	var ticker = time.NewTicker(math2.MaxDuration(d, time.Second))
	defer ticker.Stop()
	for {
		select {
		case <-s.exit.C:
			return
		case <-ticker.C:
			s.router.KeepAlive()
		}
	}
}

func (s *Proxy) acceptConn(l net.Listener) (net.Conn, error) {
	var delay = &DelayExp2{
		Min: 10, Max: 500,
		Unit: time.Millisecond,
	}
	for {
		c, err := l.Accept()
		if err != nil {
			if e, ok := err.(net.Error); ok && e.Temporary() {
				log.WarnErrorf(err, "[%p] proxy accept new connection failed", s)
				delay.Sleep()
				continue
			}
		}
		return c, err
	}
}

type Overview struct {
	Version string         `json:"version"`
	Compile string         `json:"compile"`
	Config  *Config        `json:"config,omitempty"`
	Model   *models.Proxy  `json:"model,omitempty"`
	Stats   *Stats         `json:"stats,omitempty"`
	Slots   []*models.Slot `json:"slots,omitempty"`
}

type Stats struct {
	Online bool `json:"online"`
	Closed bool `json:"closed"`

	Sentinels struct {
		Servers  []string          `json:"servers,omitempty"`
		Masters  map[string]string `json:"masters,omitempty"`
		Switched bool              `json:"switched,omitempty"`
	} `json:"sentinels"`

	Ops struct {
		Total     int64 `json:"total"`
		Fails     int64 `json:"fails"`
		WriteSucc int64 `json:"writesucc"`
		WriteFail int64 `json:"writefail"`
		Redis     struct {
			Errors int64 `json:"errors"`
		} `json:"redis"`
		QPS int64      `json:"qps"`
		Cmd []*OpStats `json:"cmd,omitempty"`
	} `json:"ops"`

	Sessions struct {
		Total int64 `json:"total"`
		Alive int64 `json:"alive"`
	} `json:"sessions"`

	Rusage struct {
		Now string       `json:"now"`
		CPU float64      `json:"cpu"`
		Mem int64        `json:"mem"`
		Raw *utils.Usage `json:"raw,omitempty"`
	} `json:"rusage"`

	Backend struct {
		PrimaryOnly bool `json:"primary_only"`
	} `json:"backend"`

	Runtime *RuntimeStats `json:"runtime,omitempty"`
}

type RuntimeStats struct {
	General struct {
		Alloc   uint64 `json:"alloc"`
		Sys     uint64 `json:"sys"`
		Lookups uint64 `json:"lookups"`
		Mallocs uint64 `json:"mallocs"`
		Frees   uint64 `json:"frees"`
	} `json:"general"`

	Heap struct {
		Alloc   uint64 `json:"alloc"`
		Sys     uint64 `json:"sys"`
		Idle    uint64 `json:"idle"`
		Inuse   uint64 `json:"inuse"`
		Objects uint64 `json:"objects"`
	} `json:"heap"`

	GC struct {
		Num          uint32  `json:"num"`
		CPUFraction  float64 `json:"cpu_fraction"`
		TotalPauseMs uint64  `json:"total_pausems"`
	} `json:"gc"`

	NumProcs      int   `json:"num_procs"`
	NumGoroutines int   `json:"num_goroutines"`
	NumCgoCall    int64 `json:"num_cgo_call"`
	MemOffheap    int64 `json:"mem_offheap"`
}

type StatsFlags uint32

func (s StatsFlags) HasBit(m StatsFlags) bool {
	return (s & m) != 0
}

const (
	StatsCmds = StatsFlags(1 << iota)
	StatsSlots
	StatsRuntime

	StatsFull = StatsFlags(^uint32(0))
)

func (s *Proxy) Overview(flags StatsFlags) *Overview {
	o := &Overview{
		Version: utils.Version,
		Compile: utils.Compile,
		Config:  s.Config(),
		Model:   s.Model(),
		Stats:   s.Stats(flags),
	}
	if flags.HasBit(StatsSlots) {
		o.Slots = s.Slots()
	}
	return o
}

func (s *Proxy) Stats(flags StatsFlags) *Stats {
	stats := &Stats{}
	stats.Online = s.IsOnline()
	stats.Closed = s.IsClosed()

	servers, masters := s.GetSentinels()
	if servers != nil {
		stats.Sentinels.Servers = servers
	}
	if masters != nil {
		stats.Sentinels.Masters = make(map[string]string)
		for gid, addr := range masters {
			stats.Sentinels.Masters[strconv.Itoa(gid)] = addr
		}
	}
	stats.Sentinels.Switched = s.HasSwitched()

	stats.Ops.Total = OpTotal()
	stats.Ops.Fails = OpFails()
	stats.Ops.WriteSucc = OpWriteSucc()
	stats.Ops.WriteFail = OpWriteFail()
	stats.Ops.Redis.Errors = OpRedisErrors()
	stats.Ops.QPS = OpQPS()

	if flags.HasBit(StatsCmds) {
		stats.Ops.Cmd = GetOpStatsAll()
	}

	stats.Sessions.Total = SessionsTotal()
	stats.Sessions.Alive = SessionsAlive()

	if u := GetSysUsage(); u != nil {
		stats.Rusage.Now = u.Now.String()
		stats.Rusage.CPU = u.CPU
		stats.Rusage.Mem = u.MemTotal()
		stats.Rusage.Raw = u.Usage
	}

	stats.Backend.PrimaryOnly = s.Config().BackendPrimaryOnly

	if flags.HasBit(StatsRuntime) {
		var r runtime.MemStats
		runtime.ReadMemStats(&r)

		stats.Runtime = &RuntimeStats{}
		stats.Runtime.General.Alloc = r.Alloc
		stats.Runtime.General.Sys = r.Sys
		stats.Runtime.General.Lookups = r.Lookups
		stats.Runtime.General.Mallocs = r.Mallocs
		stats.Runtime.General.Frees = r.Frees
		stats.Runtime.Heap.Alloc = r.HeapAlloc
		stats.Runtime.Heap.Sys = r.HeapSys
		stats.Runtime.Heap.Idle = r.HeapIdle
		stats.Runtime.Heap.Inuse = r.HeapInuse
		stats.Runtime.Heap.Objects = r.HeapObjects
		stats.Runtime.GC.Num = r.NumGC
		stats.Runtime.GC.CPUFraction = r.GCCPUFraction
		stats.Runtime.GC.TotalPauseMs = r.PauseTotalNs / uint64(time.Millisecond)
		stats.Runtime.NumProcs = runtime.GOMAXPROCS(0)
		stats.Runtime.NumGoroutines = runtime.NumGoroutine()
		stats.Runtime.NumCgoCall = runtime.NumCgoCall()
		stats.Runtime.MemOffheap = unsafe2.OffheapBytes()
	}
	return stats
}

func (s *Proxy) makeBackupConn() (*backup.Conn, error) {
	c, err := backup.DialTimeout(s.config.BackupAddr, time.Second*5,
		s.config.BackupRecvBufsize.AsInt(),
		s.config.BackupSendBufsize.AsInt())
	if err != nil {
		log.Warnf("proxy connect backup(%s) failed:%s\n", s.config.BackupAddr, err)
		return nil, err
	}
	log.Infof("proxy(%s) connected backup(%s)\n", c.LocalAddr(), s.config.BackupAddr)
	c.ReaderTimeout = s.config.BackupRecvTimeout.Duration()
	c.WriterTimeout = s.config.BackupSendTimeout.Duration()
	return c, nil
}

var ErrSendCmdFail = errors.New("Send Cmd Fail!")
var ErrCmdBlock = errors.New("Cmd Block,Send Fail!")

func (s *Proxy) backupLoopWriter() {
	tick := time.Tick(500 * time.Millisecond)
	for {
		if s.IsClosed() {
			return
		}
		defer s.Close()
		select {
		case <-tick:
			err := s.redisEncoder.Flush()
			if err != nil {
				log.Warnf("send cmd failed,may lost messages!")
			}
		case cmd := <-s.cmds:
			err := s.pushCmd(cmd)
			if err != nil {
				log.Warnf("send cmd failed,may lost messages!")
			}
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (s *Proxy) Write(buf []byte) (int, error) {
	s.backupReq.MultiCmd = buf
	err := s.sendCmds()
	if err == nil {
		return len(buf), nil
	} else {
		return 0, ErrSendCmdFail
	}
}

func (s *Proxy) pushCmd(cmd *Request) error {
	cmds := int(s.backupReq.GetCmds())
	if cmds >= s.config.BackupMaxReq {
		incrOpWriteFail(1)
		return ErrCmdBlock
	}
	s.redisEncoder.EncodeMultiBulk(cmd.Multi, false)
	cmds++
	*s.curCmdNum = int32(cmds)
	s.backupReq.Cmds = s.curCmdNum
	if cmds >= s.config.BackupMaxReq {
		s.redisEncoder.Flush()
	}
	return nil
}

func (s *Proxy) sendCmds() error {
	cmds := s.backupReq.GetCmds()
	if cmds == 0 {
		return nil
	}
	err := ErrSendCmdFail
	for i := 0; i < 3; i++ {
		if s.broken.IsFalse() && s.conn.WriterTimeout < time.Since(s.conn.LastWrite) {
			s.conn.Close()
			s.broken.Set(true)
		}
		if s.broken.IsTrue() {
			s.conn, err = s.makeBackupConn()
			if err != nil {
				time.Sleep(time.Second)
				continue
			} else {
				s.backupEncoder = s.conn.FlushEncoder()
				s.backupEncoder.MaxInterval = time.Millisecond
				s.backupEncoder.MaxBuffered = math2.MinInt(s.config.BackupMaxReq, cap(s.cmds))
				s.broken.Set(false)
			}
		}
		if err := s.backupEncoder.EncodeReq(s.backupReq); err != nil {
			log.Warnf("backup conn encode failure(%p),err is %s\n", s, err)
			s.conn.Close()
			s.broken.Set(true)
			continue
		}
		if err := s.backupEncoder.Flush(len(s.cmds) == 0); err != nil {
			log.Warnf("backup conn flush failure(%p),err is %s\n", s, err)
			s.conn.Close()
			s.broken.Set(true)
			continue
		}
		err = nil
		break
	}
	if err == nil && len(s.rsps) < s.config.WriteReqBufsize {
		s.rsps <- cmds
		s.backupReq.Cmds = proto.Int32(0)
	} else {
		incrOpWriteFail(int64(cmds))
	}
	return err
}

func (s *Proxy) backupLoopReader() {
	log.Infof("create backup loop reader goroutine(%p)!\n", s)
	for cmds := range s.rsps {
		if !s.broken.IsTrue() {
			if resp, err := s.conn.DecodeRes(); err != nil {
				incrOpWriteFail(int64(cmds))
				s.conn.Close()
				s.broken.Set(true)
				log.Warnf("command num = %d,decode failure %s!\n", cmds, err)
			} else {
				incrOpWriteSucc(int64(resp.GetCmds()))
			}
		}
	}
}
