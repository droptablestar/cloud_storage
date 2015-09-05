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
	"time"

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

var startTime = time.Now()

type DFSNode struct {
	nid   uint64
	name  string
	attr  fuse.Attr
	dirty bool
	kids  map[string]*DFSNode
	data  []uint8
}

func (d *DFSNode) init(name string, mode os.FileMode) {
	d.name = name
	d.attr = fuse.Attr{
		Valid:  1 * time.Minute,
		Atime:  startTime,
		Mtime:  startTime,
		Ctime:  startTime,
		Crtime: startTime,
		Mode:   mode,
		Nlink:  1,
		Uid:    501,
		Gid:    20,
	}
	d.kids = make(map[string]*DFSNode)
	d.data = make([]uint8, 64)
}

type FS struct{}

var root *DFSNode

// Implement:
func (FS) Root() (fs.Node, error) {
	root.attr.Size = 64
	return root, nil
}
func (n *DFSNode) Attr(ctx context.Context, attr *fuse.Attr) error {
	p_out("Attr: \n%#v\nattr: %#v\n\n", n, attr)
	*attr = n.attr
	return nil
}

func (n *DFSNode) Getattr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error {
	p_out("Getattr:\nn: %#v \nreq: %#v\nresp:%#v\n\n", n, req, resp)
	resp.Attr = n.attr
	return nil
}

func (n *DFSNode) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	p_out("Setattr\nn: %#v \nreq: %#v\n\n", n, req)
	n.attr.Size = req.Size
	return nil
}

func (n *DFSNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	// p_out("Lookup: \nname: %s \n%#v\n\n", name, n)
	if child, ok := n.kids[name]; ok {
		return child, nil
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

func (n *DFSNode) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	p_out("mkdir %q in %q\n", req, n.name)
	d := new(DFSNode)
	d.init(req.Name, req.Mode)
	n.kids[req.Name] = d
	return d, nil
}

func (p *DFSNode) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	p_out("Create: \nreq: %#v\n\n", req)
	f := new(DFSNode)
	f.init(req.Name, 0666)
	p.kids[req.Name] = f
	p_out("f: %#v\n\n", f)
	return f, f, nil
}

func (n *DFSNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	p_out("Write: \nreq: %#v\n: %#v\n\n", req, n)
	if int64(len(n.data)) < req.Offset {
		n.data = make([]uint8, int8(req.Offset)+int8(len(req.Data)))
	}
	resp.Size = copy(n.data[req.Offset:], req.Data)
	n.attr.Size = uint64(resp.Size)
	// n.dirty = true
	p_out("n: %#v\n\n", n)
	return nil
}

func (n *DFSNode) ReadAll(ctx context.Context) ([]byte, error) {
	p_out("ReadAll: \nn:%#v\n\n", n)
	return n.data, nil
}

func (n *DFSNode) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	p_out("Fsync\n\n")
	return nil
}

func (n *DFSNode) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	p_out("Flush: \n:%#v \nn: %#v\n\n", req, n)
	return nil
}

func (n *DFSNode) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	p_out("Remove\n\n")
	return nil
}

func (n *DFSNode) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	p_out("Rename\n\n")
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
	flag.Usage = Usage
	flag.BoolVar(&debug, "debug", false, "debugging")
	flag.StringVar(&mountpoint, "mount", "dss", "defaults to local 'dss'")
	flag.Parse()

	p_out("main\n")

	root = new(DFSNode)
	root.init("", os.ModeDir|0755)
	// root.attr.Size = 64

	// nodeMap[uint64(root.attr.Inode)] = root
	p_out("root inode %d\n", int(root.attr.Inode))
	// nodeMap[uint64(root.attr.Inode)] = root

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
