// memfs implements a simple in-memory file system.
package main

/*
 Two main files are ../fuse.go and ../fs/serve.go
*/

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"

	"golang.org/x/net/context"
)

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

type Node struct {
	nid   uint64
	name  string
	attr  fuse.Attr
	dirty bool
	kids  map[string]*Node
	data  []uint8
}

var root *Node

type FS struct{}

//  Compile error if Node does not implement interface fs.Node, or if FS does not implement fs.FS
var _ fs.Node = (*Node)(nil)
var _ fs.FS = (*FS)(nil)

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

// Implement:
//   func (FS) Root() (n fs.Node, err error)
//   func (n *Node) Attr(ctx context.Context, attr *fuse.Attr) error
//   func (n *Node) Lookup(ctx context.Context, name string) (fs.Node, error)
//   func (n *Node) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error)
//   func (n *Node) Getattr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error
//   func (n *Node) Fsync(ctx context.Context, req *fuse.FsyncRequest) error
//   func (n *Node) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error
//   func (p *Node) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error)
//   func (p *Node) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error)
//   func (n *Node) ReadAll(ctx context.Context) ([]byte, error)
//   func (n *Node) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error
//   func (n *Node) Flush(ctx context.Context, req *fuse.FlushRequest) error
//   func (n *Node) Remove(ctx context.Context, req *fuse.RemoveRequest) error
//   func (n *Node) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error

//=============================================================================

var Usage = func() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s MOUNTPOINT\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	var debug bool
	var mountpoint string

	flag.Usage = Usage
	flag.BoolVar(&debug, "debug", true, "debugging")
	flag.StringVar(&mountpoint, "mount", "dss", "defaults to local 'dss'")
	flag.Parse()

	p_out("main\n")

	root = new(Node)
	root.init("", os.ModeDir|0755)

	nodeMap[uint64(root.attr.Inode)] = root
	p_out("root inode %d", int(root.attr.Inode))

	if _, err := os.Stat(mountpoint); err != nil {
		os.Mkdir(mountpoint, 0755)
	}
	fuse.Unmount(mountpoint)
	c, err := fuse.Mount(mountpoint, fuse.FSName("dssFS"), fuse.Subtype("project P1"),
		fuse.LocalVolume(), fuse.VolumeName("dssFS"))
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

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
