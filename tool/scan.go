package main

import (
	"flag"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"math"
	"os"
	"reflect"
)

var addr, passwd string

func init() {
	flag.StringVar(&addr, "addr", "127.0.0.1:6379", "address of redis-server")
	flag.StringVar(&passwd, "auth", "vip", "auth passwd")
}

func keytype(c redis.Conn, key string) string {

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
		return t
	}
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

	for i := 0; i < 1024; i += 1 {
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
				for l, s := range values {
					v := reflect.ValueOf(s)
					if v.Kind() == reflect.Slice {
						if l == 0 {
							cursor = 0
							for j := 0; j != v.Len(); j += 1 {
								e := v.Index(j)
								if e.Kind() == reflect.Uint8 {
									cursor += int(e.Interface().(uint8)-'0') * int(math.Pow10(v.Len()-j-1))
								}
							}
						} else {
							if v.Kind() == reflect.Slice {
								for j := 0; j != v.Len(); j += 1 {
									fmt.Printf("slot|%d|cursor|%d|key|%s\n", i, cursor, v.Index(j))
								}

							}
						}
					}
				}
			}
			if cursor == 0 {
				break
			}
		}
	}
}
