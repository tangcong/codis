package main

import (
	"fmt"
	"github.com/CodisLabs/codis/pkg/backup"
	"github.com/CodisLabs/codis/pkg/utils"
	"github.com/CodisLabs/codis/pkg/utils/log"
	"github.com/docopt/docopt-go"
	"time"
)

func main() {
	const usage = `
Usage:
	codis-backup [--ncpu=N [--max-ncpu=MAX]] [--config=CONF] [--log-level=LEVEL] [--host-admin=ADDR] [--host-proxy=ADDR] [--zookeeper=ADDR|--etcd=ADDR|--db=ADDR] [--pidfile=FILE]
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

	s, err := backup.New(config)
	if err != nil {
		log.PanicErrorf(err, "create proxy with config failed\n%s", config)
	}
	defer s.Close()

	log.Warnf("create backup with config\n%s", config)

	log.Warnf("[%p] backup is working ...", s)

	for !s.IsClosed() {
		time.Sleep(time.Second)
	}
}
