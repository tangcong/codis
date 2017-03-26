package main

import (
	"fmt"
	"github.com/CodisLabs/codis/pkg/backup"
	"github.com/CodisLabs/codis/pkg/models"
	"github.com/CodisLabs/codis/pkg/utils"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/docopt/docopt-go"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

func main() {
	const usage = `
Usage:
	codis-backup [--ncpu=N [--max-ncpu=MAX]] [--config=CONF] [--log=FILE] [--log-level=LEVEL] [--host-admin=ADDR] [--host-proxy=ADDR] [--zookeeper=ADDR|--etcd=ADDR|--db=ADDR] [--pidfile=FILE]
	codis-backup --default-config
	codis-backup --version

Options:
	--ncpu=N set runtime.GOMAXPROCES to N,default is runtime.NumCPU().
	-c CONF,--config=CONF run with the specific configuration
	-l FILE, --log=FILE set path/name of daily rotated log file.
	--log-level=LEVEL  set the log-level,should be INFO,WARN,DEBUG or ERROR,default is INFO
`
	d, err := docopt.Parse(usage, nil, true, "", false)
	if err != nil {
		log.PanicError(err, "parse arguments failed")
	}

	switch {
	case d["--default-config"]:
		fmt.Println(backup.DefaultConfig)
		return

	case d["--version"].(bool):
		fmt.Println("verison:", utils.Version)
		fmt.Println("compile:", utils.Compile)
		return

	}
	if s, ok := utils.Argument(d, "--log"); ok {
		w, err := log.NewRollingFile(s, log.DailyRolling)
		if err != nil {
			log.PanicErrorf(err, "open log file %s failed", s)
		} else {
			log.StdLog = log.New(w, "")
		}
	}

	log.SetLevel(log.LevelInfo)

	if s, ok := utils.Argument(d, "--log-level"); ok {
		if !log.SetLevelString(s) {
			log.Panicf("option --log-level = %s", s)
		}
	}

	config := backup.NewDefaultConfig()
	if s, ok := utils.Argument(d, "--config"); ok {
		if err := config.LoadFromFile(s); err != nil {
			log.PanicErrorf(err, "load config %s failed", s)
		}

	}
	localIp, err := utils.GetLocalIp(config.BindItf)
	if err != nil {
		log.PanicErrorf(err, "interface is %s,get local ip failed", config.BindItf)
	}
	config.BackupAddr = localIp + ":" + strconv.Itoa(config.BackupPort)
	config.AdminAddr = localIp + ":" + strconv.Itoa(config.AdminPort)

	var coordinator struct {
		name string
		addr string
	}

	switch {

	case d["--zookeeper"] != nil:
		coordinator.name = "zookeeper"
		coordinator.addr = utils.ArgumentMust(d, "--zookeeper")

	case d["--etcd"] != nil:
		coordinator.name = "etcd"
		coordinator.addr = utils.ArgumentMust(d, "--etcd")

	case d["--filesystem"] != nil:
		coordinator.name = "filesystem"
		coordinator.addr = utils.ArgumentMust(d, "--filesystem")

	case d["--db"] != nil:
		coordinator.name = "db"
		coordinator.addr = utils.ArgumentMust(d, "--db")
	}
	if coordinator.name != "" {
		log.Warnf("option --%s = %s", coordinator.name, coordinator.addr)
	}

	log.Warnf("name is %s,addr is %s\n", coordinator.name, coordinator.addr)

	if config.ProductFrom == "coordinator" {

		client, err := models.NewClient(coordinator.name, coordinator.addr, time.Minute)
		if err != nil {
			log.PanicErrorf(err, "create '%s' client to '%s' failed", coordinator.name, coordinator.addr)
		}
		defer client.Close()
		store := models.NewStore(client, "__config__")
		cluster, err := store.LoadBackupProduct(config.BackupAddr, true)
		if err != nil {
			log.PanicErrorf(err, "load proxy product failed")
		}
		config.ProductName = cluster.ProductName
		log.Warnf("product name is %s\n", config.ProductName)

	}

	go backup.AsyncFlushFile(config)

	s, err := backup.New(config)
	if err != nil {
		log.PanicErrorf(err, "create proxy with config failed\n%s", config)
	}
	defer s.Close()

	if s, ok := utils.Argument(d, "--pidfile"); ok {
		if pidfile, err := filepath.Abs(s); err != nil {
			log.WarnErrorf(err, "parse pidfile = '%s' failed", s)
		} else if err := ioutil.WriteFile(pidfile, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
			log.WarnErrorf(err, "write pidfile = '%s' failed", pidfile)
		} else {
			defer func() {
				if err := os.Remove(pidfile); err != nil {
					log.WarnErrorf(err, "remove pidfile = '%s' failed", pidfile)
				}
			}()
			log.Warnf("option --pidfile = %s", pidfile)
		}
	}

	log.Warnf("create backup with config\n%s", config)

	log.Warnf("[%p] backup is working ...", s)

	go func() {
		defer s.Close()
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)

		sig := <-c
		log.Warnf("[%p] proxy receive signal = '%v'", s, sig)
	}()

	for !s.IsClosed() {
		time.Sleep(time.Second)
	}
}
