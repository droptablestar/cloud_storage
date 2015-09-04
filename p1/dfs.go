// memfs implements a simple in-memory file system.  v0.2
package main

/*
 Two main files are ../fuse.go and ../fs/serve.go
*/

import (
	"flag"
	"fmt"
	"log"
	"os"
	// "time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"

	"golang.org/x/net/context"
)

//=============================================================================

func p_out(s string, args ...interface{}) {
	if !debug {
		return
	}
	fmt.Printf(s, args...)
}

func p_err(s string, args ...interface{}) {
	fmt.Printf(s, args...)
}

//=============================================================================
/*
    Need to implement these types from bazil/fuse/fs!

    type FS interface {
	  // Root is called to obtain the Node for the file system root.
	  Root() (Node, error)
    }

    type Node interface {
	  // Attr fills attr with the standard metadata for the node.
	  Attr(ctx context.Context, attr *fuse.Attr) error
    }
*/

//=============================================================================
//  Compile error if DFSNode does not implement interface fs.Node, or if FS does not implement fs.FS
var _ fs.Node = (*DFSNode)(nil)
var _ fs.FS = (*FS)(nil)

type DFSNode struct {
	nid   uint64
	name  string
	attr  fuse.Attr
	dirty bool
	kids  map[string]*DFSNode
	data  []uint8
}

var root *DFSNode

type FS struct{}

// Implement:
func (FS) Root() (n fs.Node, err error) {
	fmt.Println("HERE")
	// n = DFSNode{name: "/", attr: fuse.Attr{Mode: os.ModeDir | 0755}, dirty: false}
	// n.kids = make(map[string]*DFSNode{mountpoint: root})
	// n.data = new([]uint8, 64)
	root = new(DFSNode)
	root.name = "/"
	root.kids = make(map[string]*DFSNode)
	root.data = make([]uint8, 64)
	return root, nil
}
func (DFSNode) Attr(ctx context.Context, attr *fuse.Attr) error {
	attr.Inode = 1
	attr.Mode = os.ModeDir | 0555
	return nil
}

func (n *DFSNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	return n, nil
}

func (n *DFSNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	return []fuse.Dirent{}, nil
}
func (n *DFSNode) Getattr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error {
	return nil
}
func (n *DFSNode) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	return nil
}
func (n *DFSNode) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	return nil
}
func (p *DFSNode) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	return DFSNode{}, nil
}

func (p *DFSNode) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	return DFSNode{}, DFSNode{}, nil
}

func (n *DFSNode) ReadAll(ctx context.Context) ([]byte, error) {
	return []byte("HELLO"), nil
}
func (n *DFSNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	return nil
}
func (n *DFSNode) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	return nil
}
func (n *DFSNode) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	return nil
}
func (n *DFSNode) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	return nil
}

//=============================================================================

var Usage = func() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s MOUNTPOINT\n", os.Args[0])
	flag.PrintDefaults()
}

var debug bool
var mountpoint string

func main() {
	fmt.Println("HELLO")

	flag.Usage = Usage
	flag.BoolVar(&debug, "debug", true, "debugging")
	flag.StringVar(&mountpoint, "mount", "/tmp/dss", "defaults to local '/tmp/dss'")
	flag.Parse()

	p_out("main\n")

	// nodeMap[uint64(root.attr.Inode)] = root
	// p_out("root inode %d", int(root.attr.Inode))
	// p_out("root mode %d", int(root.attr.Mode))

	if _, err := os.Stat(mountpoint); err != nil {
		os.Mkdir(mountpoint, 0755)
	}
	fuse.Unmount(mountpoint)
	c, err := fuse.Mount(
		mountpoint,
		fuse.FSName("dssFS"),
		fuse.Subtype("project P1"),
		fuse.LocalVolume(),
		fuse.VolumeName("dssFS"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	err = fs.Serve(c, FS{})
	fmt.Printf("%#v\n", root)
	if err != nil {
		log.Fatal(err)
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		log.Fatal(err)
	}
}
