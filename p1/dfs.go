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

type FS struct{}

var root *DFSNode

// Implement:
func (FS) Root() (fs.Node, error) {
	fmt.Println("HERE")
	root = &DFSNode{
		name: "rt",
		kids: make(map[string]*DFSNode),
		attr: fuse.Attr{Mode: os.ModeDir | 0755},
	}
	return root, nil
	// return &DFSNode{1, "/", attr: fuse.Attr{Mode: os.ModeDir | 0755}, dirty: false}
	// n.kids = make(map[string]*DFSNode{mountpoint: root})
	// n.data = new([]uint8, 64)
	// root = new(DFSNode)
	// root.name = "/"
	// root.kids = make(map[string]*DFSNode)
	// root.data = make([]uint8, 64)
	// fmt.Printf("Root: %#v\n\n", root)
}
func (n *DFSNode) Attr(ctx context.Context, attr *fuse.Attr) error {
	fmt.Printf("Attr: %#v\n\n", n)
	attr.Inode = 0
	attr.Mode = os.ModeDir | 0755
	return nil
}

func (n *DFSNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	fmt.Printf("Lookup: name: %s - %#v\n\n", name, n)
	if val, ok := n.kids[name]; ok {
		return val, nil
	}
	return nil, fuse.ENOENT
}

func (n *DFSNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	fmt.Printf("ReadDirAll: %#v\n\n", n)
	var dirDirs = []fuse.Dirent{}
	for _, val := range n.kids {
		dirDirs = append(dirDirs, fuse.Dirent{Inode: val.nid, Type: fuse.DT_Dir, Name: val.name})
	}
	return dirDirs, nil
}

// func (n *DFSNode) Getattr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error {
// 	fmt.Printf("Getattr: req: %#v\n\n", req)
// 	return nil
// }

// func (n *DFSNode) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
// 	fmt.Printf("Fsync\n\n")
// 	return nil
// }
// func (n *DFSNode) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
// 	fmt.Printf("Setattr\n\n")
// 	return nil
// }
func (n *DFSNode) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	fmt.Printf("Mkdir: %#v\n\n", n)
	if _, ok := n.kids[req.Name]; ok {
		return nil, fuse.EIO
	}
	node := &DFSNode{name: req.Name, kids: make(map[string]*DFSNode), attr: fuse.Attr{Mode: os.ModeDir | 0755}}
	n.kids[req.Name] = node
	return node, nil
}

// func (p *DFSNode) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
// 	fmt.Printf("Create\n\n")
// 	return nil, nil, nil
// }

// func (n *DFSNode) ReadAll(ctx context.Context) ([]byte, error) {
// 	fmt.Printf("ReadAll\n\n")
// 	return []byte("HELLO"), nil
// }
// func (n *DFSNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
// 	fmt.Printf("Write\n\n")
// 	return nil
// }
// func (n *DFSNode) Flush(ctx context.Context, req *fuse.FlushRequest) error {
// 	fmt.Printf("Flush\n\n")
// 	return nil
// }
// func (n *DFSNode) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
// 	fmt.Printf("Remove\n\n")
// 	return nil
// }
// func (n *DFSNode) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
// 	fmt.Printf("Rename\n\n")
// 	return nil
// }

//=============================================================================

var Usage = func() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s MOUNTPOINT\n", os.Args[0])
	flag.PrintDefaults()
}

var debug bool
var mountpoint string

func main() {
	flag.Usage = Usage
	flag.BoolVar(&debug, "debug", true, "debugging")
	flag.StringVar(&mountpoint, "mount", "dss", "defaults to local 'dss'")
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
	if err != nil {
		log.Fatal(err)
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		log.Fatal(err)
	}
}
