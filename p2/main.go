//
package main

/*

 */

import (
	//	Should be "github.com/mattn/go-getopt", but seems go
	//	doesn't like to mix absolute and relative imports.
	. "../../../github.com/mattn/go-getopt"
	"fmt"
	"os"

	"./dfs"
)

//=============================================================================

func main() {
	var c int
	tm := ""
	debug := false
	compress := false
	mount := "dss"
	storePath := "db"
	newfs := ""

	for {
		if c = Getopt("cdnm:s:"); c == EOF {
			break
		}

		switch c {
		case 'c':
			compress = !compress // ignore
		case 'd':
			debug = !debug
		case 'm':
			mount = OptArg
		case 'n':
			newfs = "NEWFS "
		case 's':
			storePath = OptArg
		case 't':
			tm = OptArg
		default:
			println("usage: main.go [-d | -c | -m <mountpt> | -t <timespec>]", c)
			os.Exit(1)
		}
	}
	fmt.Printf("\nStartup up with debug %v, compress %v, mountpt: %q, %sstorePath %q, time %q\n\n", debug, compress, mount, newfs, storePath, tm)

	dfs.Init(debug, compress, mount, newfs != "", storePath, tm)

}
