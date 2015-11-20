package dfs

import (
	"time"
)

func flush(n *DNode) string {
	for _, val := range n.kids {
		if val.metaDirty {
			// p_out("flush(): %q\n", val)
			n.metaDirty = true // sanity check
			n.ChildSigs[val.Name] = flush(val)
		}
	}
	if n.metaDirty {
		// p_out("flushing: %q\n", n)
		// n.Attrs.Atime = time.Now()
		// n.Attrs.Mtime = time.Now()
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

func Flusher(sem chan int) {
	for {
		time.Sleep(5 * time.Second)
		in()
		// p_out("\n\tFLUSHER\n\n")
		if root.metaDirty {
			// p_out("FLUSHING root: %q\n", root)
			root.Attrs.Atime = time.Now()
			flush(root)
			version++

			head.Root = root.PrevSig
			head.NextInd = nextInd
			putBlockSig("head", marshal(head))
		}

		out()
	}
}
