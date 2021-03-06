package dfs

import (
	"fmt"
	"log"
	"os"
)

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func findDNode(n *DNode) {
	if ModeConsistency == "strong" && !Token {
		p_out("\nNEED TOKEN\n")
		for _, c := range Clients {
			p_out("Requesting token from %d\n", n.Owner)
			var reply Response
			c.Call("Node.ReqToken", &Request{"", Merep.Pid}, &reply)
			if reply.Ack {
				Token = true
				p_out("Got token from %d\n\n", n.Owner)
				break
			}
		}
		if !Token {
			p_out("SOMETHING BAD HAPPENED AND I DIDN'T GET THE TOKEN!\n")
		}
	}
	if nd, ok := nodeMap[n.Attrs.Inode]; ok {
		n = nd
	} else {
		nodeMap[n.Attrs.Inode] = n
	}
}

func p_out(s string, args ...interface{}) {
	if !Debug {
		return
	}
	log.Printf(s, args...)
}

func p_err(s string, args ...interface{}) {
	log.Printf(s, args...)
}

func p_dieif(b bool, s string, args ...interface{}) {
	if b {
		fmt.Printf(s, args...)
		os.Exit(1)
	}
}

func p_die(s string, args ...interface{}) {
	fmt.Printf(s, args...)
	os.Exit(1)
}
