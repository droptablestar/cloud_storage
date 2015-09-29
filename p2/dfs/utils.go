//
package dfs

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"log"
	"os"
)

//var d *diskv.Diskv
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

func p_out(s string, args ...interface{}) {
	if !debug {
		return
	}
	log.Printf(s, args...)
}

func p_err(s string, args ...interface{}) {
	log.Printf(s, args...)
	os.Exit(1)
}

//=============================================================================
func initStore(newfs bool, dbPath string) {
	/*
		d = diskv.New(diskv.Options{
			BasePath:     "key-store",
			CacheSizeMax: 1024 * 1024,
		})*/
	var err error

	if newfs {
		os.RemoveAll(dbPath)
	}
	db, err = leveldb.OpenFile(dbPath, nil)

	if err != nil {
		panic("no open db\n")
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
		p_err("ERROR: getDNode [%s]\n", err)
		return nil
	}
}

func markDirty(n *DNode) {
	for ; n.parent != nil; n = n.parent {
		// p_out("MARKING %q\n", n)
		n.metaDirty = true
	}
	// p_out("OUT MARKING %q\n", n)
	n.metaDirty = true
}

func flush(n *DNode) string {
	for _, val := range n.kids {
		if val.metaDirty {
			p_out("flush(): %q\n", val)
			n.metaDirty = true // sanity check
			n.ChildSigs[val.Name] = flush(val)
		}
	}
	if n.metaDirty {
		p_out("flushing: %q\n", n)
		n.Version = version
		n.sig = putBlock(marshal(n))
		n.metaDirty = false
	}
	return n.sig
}
