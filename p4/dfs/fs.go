package dfs

import (
	"encoding/json"
	"fmt"
	"log"
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
	Owner      int

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
	d.Owner = Merep.Pid
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

var uid = uint32(os.Geteuid())
var gid = uint32(os.Getegid())
var root *DNode
var head *Head
var nextInd uint64 = 1
var version uint64 = 1
var sem chan int

var nodeMap map[uint64]*DNode

var server *fs.Server

type FS struct{}

//=============================================================================

func getHead() (*DNode, uint64) {
	if val, err := db.Get([]byte("head"), nil); err == nil {
		p_out("FOUND HEAD!")
		json.Unmarshal(val, &head)
		return getDNode(head.Root), head.NextInd
	}
	return nil, 0
}

func Init(mountPoint string, newfs bool, dbPath string) {
	initStore(newfs, dbPath)

	nodeMap = make(map[uint64]*DNode)

	if n, ni := getHead(); n != nil {
		root = n
		root.sig = head.Root
		nextInd = ni
		version = n.Version + 1
		nodeMap[root.Attrs.Inode] = root
	} else {
		p_out("GETHEAD fail\n")
		root = new(DNode)
		root.init("", os.ModeDir|0755)
		nodeMap[root.Attrs.Inode] = root

		head = new(Head)
		head.Root = root.sig
		head.NextInd = nextInd
	}
	p_out("root %q", root)
	p_out("root inode %v", root.Attrs.Inode)

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

	for _, c := range Clients {
		p_out("client: %s\n", c)
	}

	server = fs.New(c, nil)
	err = server.Serve(FS{})

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

//=============================================================================

func (FS) Root() (fs.Node, error) {
	p_out("root returns as %d\n", int(root.Attrs.Inode))
	return root, nil
}
