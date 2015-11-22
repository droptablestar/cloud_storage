package dfs

import (
	"time"
)

func flush(n *DNode) string {
	for _, val := range n.kids {
		if val.metaDirty {
			p_out("flush(): %q\n", val)
			n.metaDirty = true // sanity check
			n.ChildSigs[val.Name] = flush(val)
		}
	}
	if n.metaDirty {
		// p_out("flushing: %q\n", n)
		n.Version = version
		tmp := putBlock(marshal(n))
		for _, c := range Clients {
			var reply Response
			p_out("sending %s to %s:%d\n", n, c.Addr, c.port)
			c.Call("Node.Receive", *n, &reply)
			p_out("Response from %s:%d -- %q\n", c.Addr, c.port, &reply)
		}
		n.PrevSig = tmp
		n.sig = n.PrevSig
		n.metaDirty = false
	}
	return n.sig
}

func flushRoot() {
	p_out("flushRoot: %q\n", root)
	if root.metaDirty {
		flush(root)
		version++

		head.Root = root.PrevSig
		head.NextInd = nextInd
		putBlockSig("head", marshal(head))
	} else {
		for _, c := range Clients {
			var reply Response
			p_out("sending %s to %s:%d\n", root, c.Addr, c.port)
			c.Call("Node.Receive", *root, &reply)
			p_out("Response from %s:%d -- %q\n",
				c.Addr, c.port, &reply)
		}
	}
}

func Flusher(sem chan int) {
	for {
		time.Sleep(time.Duration(FlusherPeriod) * time.Second)
		in()
		// p_out("\n\tFLUSHER\n\n")
		flushRoot()
		out()
	}
}
