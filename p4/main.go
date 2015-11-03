package main

import (
	"bitbucket.org/jreeseue/818/p3/dfs"
	"fmt"
)

var newfs string
var debug = false
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
		case 'c':
			newfs = "NEWFS "
		case 'd':
			debug = !debug
		case 'f':
			flusherPeriod, _ = strconv.Atoi(OptArg)
		case 'm':
			modeConsistency = OptArg
		case 'r':
			replicaString = OptArg
		default:
			println("usage: main.go [-c | -d | -f <flush dur> | -m <mode> | -r <rep string>]", c)
			os.Exit(1)
		}
	}
	fmt.Printf("\nStartup up with debug %v, flush period %v, %smode: %q, replicaStr  %q\n\n",
		debug, flusherPeriod, newfs, modeConsistency, replicaString)

	loadConfig(replicaString, "config.txt")

}
