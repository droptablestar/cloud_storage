package main

import (
	"bitbucket.org/jreeseue/818/p4/dfs"
	"fmt"
	. "github.com/mattn/go-getopt"
	"os"
	"strconv"
	"time"
)

var newfs string
var flusherPeriod = 5
var modeConsistency = "none"
var replicaString = "auto"

func main() {
	var c int

	for {
		if c = Getopt("ndf:m:r:"); c == EOF {
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
		if r != dfs.Merep {
			fmt.Printf("client r: %q\n", r)
			dfs.Clients = append(dfs.Clients, dfs.NewServerConn(r.Addr, r.Port))
		}
	}
	go func() {
		dfs.ServeInterface(dfs.Merep.Addr, dfs.Merep.Port, new(dfs.Node))
		fmt.Printf("\nDebug %v, mountpt: %q, %sstorePath %q, at%s:%d\n\n",
			dfs.Debug, dfs.Merep.Mount, newfs, dfs.Merep.Db,
			dfs.Merep.Addr, dfs.Merep.Port)
		dfs.Init(dfs.Merep.Mount, newfs != "", dfs.Merep.Db)
	}()
	for {
		time.Sleep(time.Second)
	}
}
