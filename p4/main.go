package main

import (
	"bitbucket.org/jreeseue/818/p4/dfs"
	"fmt"
	. "github.com/mattn/go-getopt"
	"os"
	"strconv"
	"time"
)

var nextSeqNum = 1

var newfs string
var flusherPeriod = 5
var modeConsistency = "none"
var replicaString = "auto"

func main() {
	var c int

	for {
		if c = Getopt("cdf:m:r:"); c == EOF {
			break
		}

		switch c {
		case 'n':
			newfs = "NEWFS "
		case 'd':
			dfs.Debug = !dfs.Debug
		case 'f':
			flusherPeriod, _ = strconv.Atoi(OptArg)
		case 'm':
			modeConsistency = OptArg
		case 'r':
			replicaString = OptArg
		default:
			println("usage: main.go [-n | -d | -f <flush dur> | -m <mode> | -r <rep string>]", c)
			os.Exit(1)
		}
	}
	fmt.Printf("\nStartup up with debug %v, flush period %v, %smode: %q, replicaStr  %q\n\n",
		dfs.Debug, flusherPeriod, newfs, modeConsistency, replicaString)

	dfs.LoadConfig(replicaString, "config.txt")
	for _, r := range dfs.Replicas {
		if r == dfs.Merep {
			fmt.Printf("server r: %q\n", r)
			go func() {
				dfs.ServeInterface(r.Addr, r.Port, new(dfs.Arith))
				for {
					fmt.Printf(".")
					time.Sleep(2 * time.Second)
				}
			}()
			time.Sleep(time.Second)
		} else {
			fmt.Printf("client r: %q\n", r)
			go func() {
				serv := dfs.NewServerConn(r.Addr, r.Port)
				// fmt.Printf("\nStartup up with debug %v, mountpt: %q, %sstorePath %q, at%s:%d\n\n",
				// 	dfs.Debug, r.mount, newfs, r.db, r.addr, r.port)

				// dfs.Init(dfs.Debug, r.Mount, newfs != "", r.Db, serv)
				for i := 0; i < 100; i++ {
					var reply int
					args := &dfs.Args{7, i}

					serv.Call("Arith.Multiply", args, &reply)

					time.Sleep(time.Second)
					fmt.Printf("\nArith: %d*%d=%d from %s\n", args.A, args.B, reply, serv.Addr)
				}
			}()
			time.Sleep(time.Second)
		}
	}
	for {
		time.Sleep(time.Second)
	}
}
