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

var b uint64
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

	// ...  for rk-chunking

}

// returns len of next chunk
func rkChunk(buf []byte) int {
	return 0
	// ...
}

// return base64 (stringified) version of sha1 hash of array of bytes
func shaString(buf []byte) string {
	h := sha1.Sum(buf)
	return base64.StdEncoding.EncodeToString(h[:])
}

// Use rk fingerprints to chunkify array of data. Take the
// stringified sha1 hash of each such chunk and use as key
// to store in key-value store. Return array of such strings.
func putBlocks(data []byte) []string {
	return []string{""}
	// ...

}

// puts a block of data at key defined by hash of data. Return ASCII hash.
func putBlock(data []byte) string {
	return ""
	// ...

}

// store data at a specific key, used for "head"
func putBlockSig(s string, data []byte) error {
	return nil
	// ...

}

// []byte or nil
func getBlock(key string) []byte {
	return nil
	// ...

}

func marshal(toMarshal interface{}) []byte {
	if buf, err := json.MarshalIndent(toMarshal, "", " "); err == nil {
		return buf
	} else {
		panic(fmt.Sprintf("Couldn't marshall %q\n", err))
	}
}
