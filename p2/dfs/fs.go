//
package dfs

/*
 Two main files are ../fuse.go and ../fs/serve.go
*/

import (
	"encoding/json"
	"log"
	"math/rand"
	"os"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

type DNode struct {
	Name      string
	Attrs     fuse.Attr
	ParentSig string
	Version   int
	PrevSig   string

	ChildSigs map[string]string

	DataBlocks []string

	sig       string
	dirty     bool
	metaDirty bool
	expanded  bool
	parent    *DNode
	kids      map[string]*DNode
	data      []byte
}

type Head struct {
	Root    string
	NextInd uint64
	Replica uint64
}

var debug = false
var compress = false
var uid = os.Geteuid()
var gid = os.Getegid()
var root *DNode
var nextInd uint64 = 1
var replicaID uint64

var sem chan int

type FS struct{}

//=============================================================================
// Let one at a time in

func Init(dbg bool, cmp bool, mountPoint string, newfs bool, dbPath string, tm string) {
	// Initialize a new diskv store, rooted at "my-data-dir", with a 1MB cache.

	debug = dbg
	compress = cmp

	initStore(newfs, dbPath)

	replicaID = uint64(rand.Int63())

	if n, ni := getHead(); n != nil {
		root = n
		nextInd = ni
	} else {
		p_out("GETHEAD fail\n")
		root = new(DNode)
		root.init("", os.ModeDir|0755)
	}
	p_out("root inode %v", root.Attrs.Inode)

	p_out("compress: %t\n", compress)

	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		p_err("mount pt creation fail\n")
	}

	fuse.Unmount(mountPoint)
	c, err := fuse.Mount(mountPoint)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	sem = make(chan int, 1)
	go Flusher(sem)

	err = fs.Serve(c, FS{})
	if err != nil {
		log.Fatal(err)
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		log.Fatal(err)
	}
}

func in() {
	sem <- 1
}

func out() {
	<-sem
}

// ...

func Flusher(sem chan int) {
	for {
		time.Sleep(5 * time.Second)
		in()

		p_out("\n\tFLUSHER\n\n")

		// ...

		out()
	}
}

//=============================================================================

func (FS) Root() (fs.Node, error) {
	p_out("root returns as %d\n", int(root.Attrs.Inode))
	return root, nil
}
