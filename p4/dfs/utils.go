package dfs

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"log"
	"os"
	"time"
)

var db *leveldb.DB

const (
	HASHLEN     = 32
	THE_PRIME   = 31
	MINCHUNK    = 2048
	TARGETCHUNK = 4096
	MAXCHUNK    = 8192
)

var b uint64 = 0
var saved [256]uint64

//=============================================================================
func initStore(newfs bool, dbPath string) {
	var err error

	if newfs {
		os.RemoveAll(dbPath)
	}
	db, err = leveldb.OpenFile(dbPath, nil)

	if err != nil {
		p_die("no open db: %s\n", err)
	}
}

// returns len of next chunk
func rkChunk(buf []byte) int {
	var off uint64
	var hash uint64
	var b_n uint64
	if b == 0 {
		b = THE_PRIME
		b_n = 1
		for i := 0; i < (HASHLEN - 1); i++ {
			b_n *= b
		}
		for i := uint64(0); i < 256; i++ {
			saved[i] = i * b_n
		}
	}

	for off = 0; off < HASHLEN && off < uint64(len(buf)); off++ {
		hash = hash*b + uint64((buf[off]))
	}

	for off < uint64(len(buf)) {
		hash = (hash-saved[buf[off-HASHLEN]])*b + uint64(buf[off])
		off++

		if (off >= MINCHUNK && ((hash % TARGETCHUNK) == 1)) || (off >= MAXCHUNK) {
			return int(off)
		}
	}
	return int(off)
}

// return base64 (stringified) version of sha1 hash of array of bytes
func shaString(buf []byte) string {
	h := sha1.Sum(buf)
	return base64.StdEncoding.EncodeToString(h[:])
}

// Use rk fingerprints to chunkify array of data. Take the
// stringified sha1 hash of each such chunk and use as key
// to store in key-value store. Return array of such strings.
func putBlocks(data []byte) (s []string) {
	off := 0
	for off < len(data) {
		ret := rkChunk(data[off:])
		// p_out("offset: %d, length: %d\n", off, ret)
		s = append(s, putBlock(data[off:(off+ret)]))
		off += ret
	}
	return
}

// puts a block of data at key defined by hash of data. Return ASCII hash.
func putBlock(data []byte) string {
	sig := shaString(data)
	if err := putBlockSig(sig, data); err == nil {
		db.Put([]byte(sig), data, nil)
		return sig
	} else {
		panic(fmt.Sprintf("FAIL: putBlock(%s): [%q]\n", sig, err))
	}
}

// store data at a specific key, used for "head"
func putBlockSig(s string, data []byte) error {
	return db.Put([]byte(s), data, nil)
}

// []byte or nil
func getBlock(key string) []byte {
	if val, err := db.Get([]byte(key), nil); err == nil {
		return val
	}
	return nil
}

func marshal(toMarshal interface{}) []byte {
	if buf, err := json.MarshalIndent(toMarshal, "", " "); err == nil {
		return buf
	} else {
		panic(fmt.Sprintf("Couldn't marshall %q\n", err))
	}
}

func getDNode(sig string) *DNode {
	n := new(DNode)
	if val, err := db.Get([]byte(sig), nil); err == nil {
		json.Unmarshal(val, &n)
		n.kids = make(map[string]*DNode)
		return n
	} else {
		p_out("ERROR: getDNode [%s]\n", err)
		return nil
	}
}

func markDirty(n *DNode) {
	for ; n.parent != nil; n = n.parent {
		n.metaDirty = true
	}
	n.metaDirty = true
}

func inArchive(n *DNode) bool {
	for ; n != nil; n = n.parent {
		if n.archive {
			return true
		}
	}
	return false
}

func flush(n *DNode) string {
	for _, val := range n.kids {
		if val.metaDirty {
			// p_out("flush(): %q\n", val)
			n.metaDirty = true // sanity check
			n.ChildSigs[val.Name] = flush(val)
		}
	}
	if n.metaDirty {
		p_out("flushing: %q\n", n)
		n.Attrs.Atime = time.Now()
		n.Attrs.Mtime = time.Now()
		n.Version = version
		tmp := putBlock(marshal(n))
		if n.Owner == Merep.Pid {
			for _, c := range Clients {
				var reply Response
				p_out("sending %s to %s:%d\n", n, c.Addr, c.port)
				c.Call("Node.Receive", n, &reply)
				p_out("Response from %s:%d -- %q\n", c.Addr, c.port, &reply)
			}
		}
		n.PrevSig = tmp
		n.sig = n.PrevSig
		n.metaDirty = false
	}
	return n.sig
}

func (nd *Node) Receive(n *DNode, reply *Response) error {
	p_out("received %q from %d\n", n, n.Owner)
	n.PrevSig = putBlock(marshal(n))
	n.sig = n.PrevSig

	if n.Attrs.Inode > nextInd {
		nextInd = n.Attrs.Inode
	} else if n.Attrs.Inode == nextInd {
		nextInd++
	}

	if child, ok := nodeMap[n.Attrs.Inode]; ok { // in map
		p_out("overwriting childSigs %q with n\n", child)
		child.Name = n.Name
		child.Attrs = n.Attrs
		child.Version = n.Version
		child.PrevSig = n.PrevSig
		child.ChildSigs = n.ChildSigs
		child.DataBlocks = n.DataBlocks
		child.Owner = n.Owner
		child.kids = make(map[string]*DNode)
		p_out("new n = %q\n", child)
	} else {
		p_out("overwriting %q with n\n", nodeMap[n.Attrs.Inode])
		nodeMap[n.Attrs.Inode] = n
		p_out("new n = %q\n", nodeMap[n.Attrs.Inode])
	}
	if n.Attrs.Inode == root.Attrs.Inode {
		head.Root = n.PrevSig
		head.NextInd = nextInd
		putBlockSig("head", marshal(head))
	}

	reply.Ack = true
	reply.Pid = Merep.Pid
	return nil
}

func peteTime(s string) (time.Time, bool) {
	timeFormats := []string{"2006-1-2 15:04:05", "2006-1-2 15:04", "2006-1-2",
		"1-2-2006 15:04:05", "1-2-2006 15:04", "1-6-2006", "2006/1/2 15:04:05",
		"2006/1/2 15:04", "2006/1/2", "1/2/2006 15:04:05", "1/2/2006 15:04", "1/2/2006"}
	loc, _ := time.LoadLocation("Local")

	for _, v := range timeFormats {
		if tm, terr := time.ParseInLocation(v, s, loc); terr == nil {
			return tm, false
		}
	}
	return time.Time{}, true
}

func (top *DNode) timeTravel(tm time.Time) *DNode {
	if tm.After(top.Attrs.Atime) {
		return top
	}
	var preTop *DNode
	for top.PrevSig != "" {
		if preTop = getDNode(top.PrevSig); preTop == nil {
			break
		}
		// p_out("preTop: %d, top: %d\n", preTop.Version, top.Version)
		p_out("preTop: %s, top: %s\n", preTop.Attrs.Atime, top.Attrs.Atime)
		p_out("preTop: %t, top: %t\n",
			tm.After(preTop.Attrs.Atime), tm.Before(top.Attrs.Atime))
		if tm.After(preTop.Attrs.Atime) &&
			tm.Before(top.Attrs.Atime) {
			return preTop
		}
		top = preTop
	}
	return top
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

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
