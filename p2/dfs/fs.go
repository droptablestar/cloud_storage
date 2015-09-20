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
	"os/signal"
	"syscall"
)

type DNode struct {
	Name      string
	Attrs     fuse.Attr
	ParentSig string
	Version   uint64
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

func (d *DNode) init(name string, mode os.FileMode) {
	startTime := time.Now()
	d.Name = name
	d.Version = nextInd
	d.ChildSigs = make(map[string]string)
	d.Attrs = fuse.Attr{
		Valid:  1 * time.Minute,
		Inode:  nextInd,
		Atime:  startTime,
		Mtime:  startTime,
		Ctime:  startTime,
		Crtime: startTime,
		Mode:   mode,
		Nlink:  1,
		Uid:    uid,
		Gid:    gid,
	}
	d.kids = make(map[string]*DNode)
}

type Head struct {
	Root    string
	NextInd uint64
	Replica uint64
}

var debug = false
var compress = false
var uid = uint32(os.Geteuid())
var gid = uint32(os.Getegid())
var root *DNode
var head *Head
var nextInd uint64 = 1
var replicaID uint64

var sem chan int

type FS struct{}

//=============================================================================
// Let one at a time in

func getHead() (*DNode, uint64) {
	if val, ok := db.Get([]byte("head"), nil); ok == nil {
		p_out("FOUND HEAD!")
		json.Unmarshal(val, &head)
		p_out("head: %q\n", val)
		if val, ok := db.Get([]byte(head.Root), nil); ok == nil {
			p_out("root: %q\n", val)
			var rt *DNode
			json.Unmarshal(val, &rt)
			return rt, head.NextInd
		}
	}
	return nil, 0
}

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

		mRoot := marshal(root)
		rootSig := shaString(mRoot)

		head = new(Head)
		head.Root = rootSig
		head.NextInd = nextInd
		mHead := marshal(head)

		db.Put([]byte("head"), mHead, nil)
		db.Put([]byte(rootSig), mRoot, nil)
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

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		defer c.Close()
		defer db.Close()
		fuse.Unmount(mountPoint)
		os.Exit(1)
	}()

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

		// p_out("\n\tFLUSHER\n\n")

		// ...

		out()
	}
}

//=============================================================================

func (FS) Root() (fs.Node, error) {
	p_out("root returns as %d\n", int(root.Attrs.Inode))
	return root, nil
}