package dfs

import (
	"encoding/json"
	"fmt"
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
	Name       string
	Attrs      fuse.Attr
	Version    uint64
	PrevSig    string
	ChildSigs  map[string]string
	DataBlocks []string

	sig       string
	dirty     bool
	metaDirty bool
	expanded  bool
	parent    *DNode
	kids      map[string]*DNode
	data      []byte
}

func (d *DNode) String() string {
	return fmt.Sprintf("Version: %d, Name: %s", d.Version, d.Name)
}

func (d *DNode) init(name string, mode os.FileMode) {
	startTime := time.Now()
	d.Name = name
	d.Version = version
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
	nextInd++
	d.kids = make(map[string]*DNode)
}

type Head struct {
	Root    string
	NextInd uint64
	Replica uint64
}

var debug = false
var tmStr = ""
var compress = false
var uid = uint32(os.Geteuid())
var gid = uint32(os.Getegid())
var root *DNode
var head *Head
var nextInd uint64 = 1
var version uint64 = 1
var replicaID uint64
var inPast = false
var sem chan int

type FS struct{}

//=============================================================================
// Let one at a time in

func getHead() (*DNode, uint64) {
	if val, err := db.Get([]byte("head"), nil); err == nil {
		p_out("FOUND HEAD!")
		json.Unmarshal(val, &head)
		if tmStr == "" {
			return getDNode(head.Root), head.NextInd
		}
		tmTime, err := time.Parse("2006-01-02T15:04:05", tmStr)
		if err != nil {
			panic(err)
		}
		p_out("%q\n", tmTime)

		root = getDNode(head.Root)
		if tmTime.After(root.Attrs.Atime) {
			return root, head.NextInd
		}
		for root.PrevSig != "" {
			inPast = true
			preRoot := getDNode(root.PrevSig)
			if tmTime.After(preRoot.Attrs.Atime) &&
				tmTime.Before(root.Attrs.Atime) {
				return root, 0
			}
			root = preRoot
		}
		return root, 0
	}
	return nil, 0
}

func Init(dbg bool, cmp bool, mountPoint string, newfs bool, dbPath string, tm string) {
	// Initialize a new diskv store, rooted at "my-data-dir", with a 1MB cache.

	debug = dbg
	tmStr = tm
	compress = cmp
	initStore(newfs, dbPath)

	replicaID = uint64(rand.Int63())

	if n, ni := getHead(); n != nil {
		root = n
		root.sig = head.Root
		nextInd = ni
		version = n.Version + 1
	} else {
		p_out("GETHEAD fail\n")
		root = new(DNode)
		root.init("", os.ModeDir|0755)

		head = new(Head)
		head.Root = root.sig
		head.NextInd = nextInd
	}
	p_out("root %q", root)
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
		if root.metaDirty {
			p_out("FLUSHING\n")
			root.Attrs.Atime = time.Now()
			root.PrevSig = root.sig
			flush(root)
			version++

			head.Root = root.sig
			head.NextInd = nextInd
			putBlockSig("head", marshal(head))
		}

		out()
	}
}

//=============================================================================

func (FS) Root() (fs.Node, error) {
	p_out("root returns as %d\n", int(root.Attrs.Inode))
	return root, nil
}
