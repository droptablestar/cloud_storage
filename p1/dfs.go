// memfs implements a simple in-memory file system.  v0.2A
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
	// p_out("init: %q with name: %q and mode: %#X\n", d, name, mode, mode)
	// had some isssues with dir's that were initially 0B size
	var size uint64 = 0
	if os.ModeDir&mode == os.ModeDir {
		size = 64
	}
	d.name = name
	d.attr = fuse.Attr{
		Valid:  1 * time.Minute,
		Size:   size,
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
	d.data = make([]uint8, 0)
}

func (d *DFSNode) String() string {
	return fmt.Sprintf("nid: %d, name: %s, attr: {%q}, dirty: %t, kids: %#v, data: %s\n",
		d.nid, d.name, d.attr, d.dirty, d.kids, d.data)
}

type FS struct{}

var root *DFSNode

// Implement:
func (FS) Root() (fs.Node, error) {
	root.attr.Inode = 1
	return root, nil
}
func (n *DFSNode) Attr(ctx context.Context, attr *fuse.Attr) error {
	p_out("attr %q <- \n%q\n\n", attr, n)
	*attr = n.attr
	return nil
}

func (n *DFSNode) Getattr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error {
	p_out("getattr for %q in \n%q\n\n", req, n)
	resp.Attr = n.attr
	return nil
}

func (n *DFSNode) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	p_out("attr for %q in \n%q\n\n", req, n)
	switch {
	case req.Valid == fuse.SetattrMode:
		n.attr.Mode = req.Mode
	case req.Valid == fuse.SetattrUid:
		n.attr.Uid = req.Uid
	case req.Valid == fuse.SetattrGid:
		n.attr.Gid = req.Gid
	case req.Valid == fuse.SetattrSize:
		n.attr.Size = req.Size
	case req.Valid == fuse.SetattrAtime:
		n.attr.Atime = req.Atime
	case req.Valid == fuse.SetattrMtime:
		n.attr.Mtime = req.Mtime
	}
	return nil
}

func (n *DFSNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	// p_out("lookup for %q in \n%q\n", name, n)
	if child, ok := n.kids[name]; ok {
		return child, nil
	}
	return nil, fuse.ENOENT
}

func (n *DFSNode) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	p_out("mkdir %q in \n%q\n\n", req, n.name)
	d := new(DFSNode)
	d.init(req.Name, req.Mode)
	n.kids[req.Name] = d
	return d, nil
}

func (n *DFSNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	p_out("readdirall for %q\n", n.name)
	var dirDirs = []fuse.Dirent{}
	for _, val := range n.kids {
		dirDirs = append(dirDirs,
			fuse.Dirent{Inode: val.attr.Inode, Type: fuse.DT_Dir, Name: val.name})
	}
	p_out("dirs: %q\n\n", dirDirs)
	return dirDirs, nil
}

func (p *DFSNode) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	p_out("create req: %q\n\n", req)
	f := new(DFSNode)
	f.init(req.Name, req.Mode)
	p.kids[req.Name] = f
	return f, f, nil
}

func (n *DFSNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	p_out("write req: %q\nin %q\n\n", req, n)
	t := make([]uint8, int64(len(n.data))+int64(req.Offset)+int64(len(req.Data)))
	copy(t, n.data)
	resp.Size = copy(t[req.Offset:], req.Data)
	n.data = t
	n.attr.Size = uint64(resp.Size)
	// n.dirty = true   TODO: Does this matter?
	return nil
}

func (n *DFSNode) ReadAll(ctx context.Context) ([]byte, error) {
	p_out("readall: %q\n\n", n)
	return n.data, nil
}

func (n *DFSNode) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	p_out("fsync for %q\n", n)
	return nil
}

func (n *DFSNode) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	p_out("flush %q \nin %q\n\n", req, n)
	return nil
}

func (n *DFSNode) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	fmt.Printf("remove %q from \n%q \n\n", req, n)
	if _, ok := n.kids[req.Name]; ok {
		p_out("deleting: %q from n.kids: %#v\n\n", req.Name, n.kids)
		delete(n.kids, req.Name)
		return nil
	}
	return fuse.ENOENT
}

func (n *DFSNode) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	p_out("Rename: \nreq: %q \n\nn: %q \nnew: %q\n\n", req, n, newDir)
	if _, ok := n.kids[req.OldName]; ok {
		n.kids[req.OldName].name = req.NewName
		n.kids[req.NewName] = n.kids[req.OldName]
		delete(n.kids, req.OldName)
		return nil
	}
	return fuse.ENOENT
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
	root.nid = 1

	// p_out("root inode %d\n", int(root.attr.Inode))
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
