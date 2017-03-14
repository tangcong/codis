package main

import (
	"bufio"
	"flag"
	"os"
	"sync/atomic"

	"fmt"
	"github.com/garyburd/redigo/redis"
)

var addr, key, passwd, method string
var step int

func init() {
	flag.StringVar(&addr, "addr", "127.0.0.1:6379", "address of redis-server")
	flag.IntVar(&step, "step", 10000, "step of zrange")
	flag.StringVar(&key, "key", "a", "the key will be dumped")
	flag.StringVar(&passwd, "auth", "vip", "auth passwd")
	flag.StringVar(&method, "method", "zcard", "zcard or zrange")
}

func auth(c redis.Conn) {
	if err := c.Send("auth", passwd); err != nil {
		panic(err)
	}
	if err := c.Flush(); err != nil {
		panic(err)
	}

	if status, err := redis.String(c.Receive()); err == nil {
		if status != "OK" {
			fmt.Println("auth passwd is invalid!")
			os.Exit(0)
		}
	} else {
		panic(err)
	}
}

func zrange(c redis.Conn) {

	var kick = make(chan struct{}, 10)
	var done int64

	go func() {

		defer close(kick)

		auth(c)

		for i := 0; atomic.LoadInt64(&done) == 0; i += step {
			if err := c.Send("zrange", key, i, i+step-1, "withscores"); err != nil {
				panic(err)
			}
			if err := c.Flush(); err != nil {
				panic(err)
			}
			kick <- struct{}{}
		}
	}()

	var w = bufio.NewWriterSize(os.Stdout, 1024*1024*32)

	for _ = range kick {
		switch values, err := redis.Strings(c.Receive()); {
		case err != nil:
			panic(err)
		case values == nil || len(values) == 0:
			atomic.StoreInt64(&done, 1)
		default:
			for _, s := range values {
				w.WriteString(s)
				w.WriteString("\r\n")
			}
			w.Flush()
		}
	}
}

func keytype(c redis.Conn) {

	auth(c)
	if err := c.Send("type", key); err != nil {
		panic(err)
	}
	if err := c.Flush(); err != nil {
		panic(err)
	}
	switch t, err := redis.String(c.Receive()); {
	case err != nil:
		panic(err)
	default:
		fmt.Printf("key|%s|type|%s\n", key, t)
	}
}

func zcard(c redis.Conn) {

	auth(c)

	if err := c.Send("zcard", key); err != nil {
		panic(err)
	}
	if err := c.Flush(); err != nil {
		panic(err)
	}
	switch num, err := redis.Int(c.Receive()); {
	case err != nil:
		panic(err)
	default:
		fmt.Printf("key|%s|num|%d\n", key, num)
	}
}

func main() {
	flag.Parse()

	c, err := redis.Dial("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	if method == "zrange" {
		zrange(c)
	} else if method == "zcard" {
		zcard(c)
	} else if method == "type" {
		keytype(c)
	}
}
