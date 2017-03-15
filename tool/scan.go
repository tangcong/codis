package main

import (
	"flag"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"os"
	"strconv"
)

var addr, passwd string
var sid, eid int

func init() {
	flag.StringVar(&addr, "addr", "127.0.0.1:6379", "address of redis-server")
	flag.StringVar(&passwd, "auth", "vip", "auth passwd")
	flag.IntVar(&sid, "sid", 0, "slot id start")
	flag.IntVar(&eid, "eid", 1024, "slot id end")
}

func main() {
	flag.Parse()

	c, err := redis.Dial("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer c.Close()

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

	for i := sid; i < eid; i += 1 {
		cursor := 0
		for {
			if err := c.Send("slotsscan", i, cursor); err != nil {
				panic(err)
			}
			if err := c.Flush(); err != nil {
				panic(err)
			}
			switch values, err := redis.Values(c.Receive()); {
			case err != nil:
				panic(err)
			default:
				for _, s := range values {
					switch s.(type) {
					case []byte:
						cursor, _ = strconv.Atoi(string(s.([]byte)))
					case []interface{}:
						for _, v := range s.([]interface{}) {
							switch v.(type) {
							case []byte:
								key := string(v.([]byte))
								fmt.Printf("slotid|%d|cursor|%d|key|%s\n", i, cursor, key)
							default:
								fmt.Printf("inner unknown type, %t, %v\n", s, s)
							}
						}
					default:
						fmt.Printf("outer unknown type, %t, %v\n", s, s)
					}
				}
			}
			if cursor == 0 {
				break
			}
		}
	}
}
