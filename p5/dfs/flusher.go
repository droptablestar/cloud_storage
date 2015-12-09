package dfs

import (
	"time"
)

func flush(n *DNode) string {
	for _, val := range n.kids {
		if val.metaDirty {
			n.metaDirty = true // sanity check
			n.ChildSigs[val.Name] = flush(val)
		}
	}
	if n.metaDirty {
		n.Version = version
		tmp := putBlock(Marshal(n))
		for _, c := range Clients {
			var enc_reply []byte
			req := aesEncrypt(AESkey, Marshal(n))
			c.Call("Node.Receive", &req, &enc_reply)
		}
		n.PrevSig = tmp
		n.sig = n.PrevSig
		n.metaDirty = false
	}
	return n.sig
}

func flushRoot() {
	if root.metaDirty {
		flush(root)
		version++

		head.Root = root.PrevSig
		head.NextInd = nextInd
		putBlockSig("head", Marshal(head))
	} else {
		for _, c := range Clients {
			var enc_reply []byte
			req := aesEncrypt(AESkey, Marshal(root))
			c.Call("Node.Receive", &req, &enc_reply)
		}
	}
}

func Flusher(sem chan int) {
	for {
		time.Sleep(time.Duration(FlusherPeriod) * time.Second)
		in()
		flushRoot()
		out()
	}
}
