// Copyright 2016 CodisLabs. All Rights Reserved.
// Licensed under the MIT (MIT-LICENSE.txt) license.

package backup

import (
	"bytes"

	"github.com/BurntSushi/toml"

	"github.com/CodisLabs/codis/pkg/utils/bytesize"
	"github.com/CodisLabs/codis/pkg/utils/errors"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/CodisLabs/codis/pkg/utils/timesize"
)

const DefaultConfig = `
##################################################
#                                                #
#                  Codis-Backup                   #
#                                                #
##################################################

# Set bind address for admin(rpc), tcp only.
admin_addr = "0.0.0.0:11081"

# Set bind ethernet interface
bind_itf = "eth0"

# Set bind address for proxy, proto_type can be "tcp", "tcp4", "tcp6", "unix" or "unixpacket".
proto_type = "tcp4"
backup_addr = "0.0.0.0:20000"

# Set backup server port
backup_port = 20001
admin_port = 11081 

# Set data file dir 
data_dir = "/data/redis"

product_name = ""

product_from = "coordinator"

# Set max number of alive sessions.
backup_max_clients = 1000

# Set max offheap memory size. (0 to disable)
backup_max_offheap_size = "1024mb"

# Set heap placeholder to reduce GC frequency.
backup_heap_placeholder = "256mb"

# Set backend recv buffer size & timeout.
backend_recv_bufsize = "128kb"
backend_recv_timeout = "30s"

# Set backend send buffer & timeout.
backend_send_bufsize = "128kb"
backend_send_timeout = "30s"

# If there is no request from client for a long time, the connection will be closed. (0 to disable)
# Set session recv buffer size & timeout.
session_recv_bufsize = "128kb"
session_recv_timeout = "30m"

# Set session send buffer size & timeout.
session_send_bufsize = "64kb"
session_send_timeout = "30s"

# Make sure this is higher than the max number of requests for each pipeline request, or your client may be blocked.
# Set session pipeline buffer size.
session_max_pipeline = 1024

# Set session tcp keepalive period. (0 to disable)
session_keepalive_period = "75s"

# Set session to be sensitive to failures. Default is false, instead of closing socket, proxy will send an error response to client.
session_break_on_failure = false

backup_bufsize = 8024000

`

type Config struct {
	ProtoType  string `toml:"proto_type" json:"proto_type"`
	BackupAddr string `toml:"backup_addr" json:"backup_addr"`
	AdminAddr  string `toml:"admin_addr" json:"admin_addr"`

	HostBackup string `toml:"-" json:"-"`

	BindItf               string         `toml:"bind_itf" json:"bind_itf"`
	ProductName           string         `toml:"product_name" json:"product_name"`
	ProductFrom           string         `toml:"product_from" json:"product_from"`
	BackupPort            int            `toml:"backup_port" json:"backup_port"`
	AdminPort             int            `toml:"admin_port" json:"admin_port"`
	DataDir               string         `toml:"data_dir" json:"data_dir"`
	BackupMaxClients      int            `toml:"backup_max_clients" json:"backup_max_clients"`
	BackupMaxOffheapBytes bytesize.Int64 `toml:"backup_max_offheap_size" json:"backup_max_offheap_size"`
	BackupHeapPlaceholder bytesize.Int64 `toml:"backup_heap_placeholder" json:"backup_heap_placeholder"`

	BackendRecvBufsize     bytesize.Int64    `toml:"backend_recv_bufsize" json:"backend_recv_bufsize"`
	BackendRecvTimeout     timesize.Duration `toml:"backend_recv_timeout" json:"backend_recv_timeout"`
	BackendSendBufsize     bytesize.Int64    `toml:"backend_send_bufsize" json:"backend_send_bufsize"`
	BackendSendTimeout     timesize.Duration `toml:"backend_send_timeout" json:"backend_send_timeout"`
	BackendMaxPipeline     int               `toml:"backend_max_pipeline" json:"backend_max_pipeline"`
	BackendPrimaryOnly     bool              `toml:"backend_primary_only" json:"backend_primary_only"`
	BackendPrimaryParallel int               `toml:"backend_primary_parallel" json:"backend_primary_parallel"`
	BackendReplicaParallel int               `toml:"backend_replica_parallel" json:"backend_replica_parallel"`
	BackendKeepAlivePeriod timesize.Duration `toml:"backend_keepalive_period" json:"backend_keepalive_period"`
	BackendNumberDatabases int32             `toml:"backend_number_databases" json:"backend_number_databases"`

	SessionRecvBufsize     bytesize.Int64    `toml:"session_recv_bufsize" json:"session_recv_bufsize"`
	SessionRecvTimeout     timesize.Duration `toml:"session_recv_timeout" json:"session_recv_timeout"`
	SessionSendBufsize     bytesize.Int64    `toml:"session_send_bufsize" json:"session_send_bufsize"`
	SessionSendTimeout     timesize.Duration `toml:"session_send_timeout" json:"session_send_timeout"`
	SessionMaxPipeline     int               `toml:"session_max_pipeline" json:"session_max_pipeline"`
	SessionKeepAlivePeriod timesize.Duration `toml:"session_keepalive_period" json:"session_keepalive_period"`
	SessionBreakOnFailure  bool              `toml:"session_break_on_failure" json:"session_break_on_failure"`
	BackupBufsize          int               `toml:"backup_bufsize" json:"backup_bufsize"`
}

func NewDefaultConfig() *Config {
	c := &Config{}
	if _, err := toml.Decode(DefaultConfig, c); err != nil {
		log.PanicErrorf(err, "decode toml failed")
	}
	if err := c.Validate(); err != nil {
		log.PanicErrorf(err, "validate config failed")
	}
	return c
}

func (c *Config) LoadFromFile(path string) error {
	_, err := toml.DecodeFile(path, c)
	if err != nil {
		return errors.Trace(err)
	}
	return c.Validate()
}

func (c *Config) String() string {
	var b bytes.Buffer
	e := toml.NewEncoder(&b)
	e.Indent = "    "
	e.Encode(c)
	return b.String()
}

func (c *Config) Validate() error {
	if c.ProtoType == "" {
		return errors.New("invalid proto_type")
	}
	if c.BackupAddr == "" {
		return errors.New("invalid proxy_addr")
	}
	if c.AdminAddr == "" {
		return errors.New("invalid admin_addr")
	}

	if c.BackupMaxClients < 0 {
		return errors.New("invalid proxy_max_clients")
	}

	const MaxInt = bytesize.Int64(^uint(0) >> 1)

	if d := c.BackupMaxOffheapBytes; d < 0 || d > MaxInt {
		return errors.New("invalid proxy_max_offheap_size")
	}
	if d := c.BackupHeapPlaceholder; d < 0 || d > MaxInt {
		return errors.New("invalid proxy_heap_placeholder")
	}

	if d := c.BackendRecvBufsize; d < 0 || d > MaxInt {
		return errors.New("invalid backend_recv_bufsize")
	}
	if c.BackendRecvTimeout < 0 {
		return errors.New("invalid backend_recv_timeout")
	}
	if d := c.BackendSendBufsize; d < 0 || d > MaxInt {
		return errors.New("invalid backend_send_bufsize")
	}
	if c.BackendSendTimeout < 0 {
		return errors.New("invalid backend_send_timeout")
	}
	if c.BackendMaxPipeline < 0 {
		return errors.New("invalid backend_max_pipeline")
	}
	if c.BackendPrimaryParallel < 0 {
		return errors.New("invalid backend_primary_parallel")
	}
	if c.BackendReplicaParallel < 0 {
		return errors.New("invalid backend_replica_parallel")
	}
	if c.BackendKeepAlivePeriod < 0 {
		return errors.New("invalid backend_keepalive_period")
	}

	if d := c.SessionRecvBufsize; d < 0 || d > MaxInt {
		return errors.New("invalid session_recv_bufsize")
	}
	if c.SessionRecvTimeout < 0 {
		return errors.New("invalid session_recv_timeout")
	}
	if d := c.SessionSendBufsize; d < 0 || d > MaxInt {
		return errors.New("invalid session_send_bufsize")
	}
	if c.SessionSendTimeout < 0 {
		return errors.New("invalid session_send_timeout")
	}
	if c.SessionMaxPipeline < 0 {
		return errors.New("invalid session_max_pipeline")
	}
	if c.SessionKeepAlivePeriod < 0 {
		return errors.New("invalid session_keepalive_period")
	}

	return nil
}
