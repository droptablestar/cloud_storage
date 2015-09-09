// memfs implements a simple in-memory file system.  v0.2A
package main

// NOTES:
//
//  redis compiled on OS X successfully 9/8/15 - 22:06
//
//  Thoughts on why flush() is called multiple times:
//  The documentation (if that's what we want to call it) for HandleFlusher()
//  says, "Flush is called each time the file or directory is closed". My
//  thought is that we open the directory when we write (or copy) the file into
//  it, then to copy the file out we need to open the directory again to modify
//  the contents of the directory to reflect that the file is no longer there.
//  Not sure if that is what's actually happening...

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

// Debugging outputs
func p_out(s string, args ...interface{}) {
	if !debug {
		return
	}
	fmt.Printf(s, args...)
}

func p_err(s string, args ...interface{}) {
	fmt.Printf(s, args...)
}

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

var ID uint64 = 0
var uid uint32 = uint32(os.Getuid())
var gid uint32 = uint32(os.Getegid())

func (d *DFSNode) init(name string, mode os.FileMode) {
	p_out("init: %q with name: %q and mode: %#X\n", d, name, mode)
	// had some isssues with dir's that were initially 0B size
	startTime := time.Now()
	var size uint64 = 0
	if os.ModeDir&mode == os.ModeDir {
		size = 64
	}
	ID += 1
	d.name = name
	d.nid = ID
	d.attr = fuse.Attr{
		Valid:  1 * time.Minute,
		Inode:  ID,
		Size:   size,
		Atime:  startTime,
		Mtime:  startTime,
		Ctime:  startTime,
		Crtime: startTime,
		Mode:   mode,
		Nlink:  1,
		Uid:    uid,
		Gid:    gid,
	}
	d.kids = make(map[string]*DFSNode)
}

func (d *DFSNode) String() string {
	// return fmt.Sprintf("nid: %d, name: %s, attr: {%q}, dirty: %t, kids: %#v, data: %s\n",
	// 	d.nid, d.name, d.attr, d.dirty, d.kids, d.data)
	return fmt.Sprintf("nid: %d, name: %s, attr: {%q}, dirty: %t, kids: %#v\n",
		d.nid, d.name, d.attr, d.dirty, d.kids)
}

type FS struct{}

var root *DFSNode

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
	// Setattr() should only be allowed to modify particular parts of a
	// nodes attributes. TODO: Is switch the best option here? Could
	// a request change multiple values (i.e. could more than one of these
	// be true?
	if req.Valid.Size() {
		n.attr.Size = req.Size
	}
	if req.Valid.Atime() {
		n.attr.Atime = req.Atime
	}
	if req.Valid.Mtime() {
		n.attr.Mtime = req.Mtime
	}
	if req.Valid.Mode() {
		n.attr.Mode = req.Mode
	}
	if req.Valid.Gid() {
		n.attr.Size = req.Size
	}
	if req.Valid.Uid() {
		n.attr.Uid = req.Uid
	}
	if req.Valid.Gid() {
		n.attr.Gid = req.Gid
	}
	return nil
}

func (n *DFSNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	// p_out("lookup for %q in \n%q\n\n", name, n)
	if child, ok := n.kids[name]; ok {
		return child, nil
	}
	return nil, fuse.ENOENT
}

// Cut and paste :-P
func (n *DFSNode) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	p_out("mkdir %q in \n%q\n\n", req, n.name)
	d := new(DFSNode)
	d.init(req.Name, req.Mode)
	n.kids[req.Name] = d
	// n.attr.Uid = req.Header.Uid
	// n.attr.Gid = req.Header.Uid
	return d, nil
}

// TODO: This seems verbose. Can I find a better way to copy the data out?
func (n *DFSNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	p_out("readdirall for %q\n", n.name)
	var dirDirs = []fuse.Dirent{}
	for _, val := range n.kids {
		typ := fuse.DT_File
		if os.ModeDir&val.attr.Mode == os.ModeDir {
			typ = fuse.DT_Dir
		}
		dirDirs = append(dirDirs,
			fuse.Dirent{Inode: val.attr.Inode, Type: typ, Name: val.name})
	}
	p_out("dirs: %q\n\n", dirDirs)
	return dirDirs, nil
}

func (n *DFSNode) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	p_out("create req: %q \nin %q\n", req, n)
	f := new(DFSNode)
	f.init(req.Name, req.Mode)
	n.kids[req.Name] = f
	return f, f, nil
}

func (n *DFSNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	// req.Offset = 200
	p_out("write req: %q\nin %q\n\n", req, n)
	// make sure there is room for whatever data is already there, whatever
	// crazy offset the write might be using, and the amount of new data
	// being written
	t := make([]uint8, int64(len(n.data))+int64(req.Offset)+int64(len(req.Data)))
	copy(t, n.data)
	resp.Size = copy(t[req.Offset:], req.Data)
	p_out("resp.Size: %d, len(t): %d\n", resp.Size, len(t))
	n.data = t
	n.attr.Size = uint64(len(t))
	// n.attr.Size = uint64(resp.Size)
	// n.dirty = true // TODO: Does this matter?
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
	p_out("remove %q from \n%q \n\n", req, n)
	// If the DFSNode exists...delete it.
	if _, ok := n.kids[req.Name]; ok {
		delete(n.kids, req.Name)
		return nil
	}
	return fuse.ENOENT
}

func (n *DFSNode) findId(nodeID uint64) (*DFSNode, bool) {
	if n.attr.Inode == nodeID {
		return n, true
	}
	for _, val := range n.kids {
		if val.attr.Inode == nodeID {
			return val, true
		}
		if rn, ok := val.findId(nodeID); ok {
			return rn, true
		}
	}
	return nil, false
}

func (n *DFSNode) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	p_out("Rename: \nreq: %q \nn: %q \nnew: %q\n\n", req, n, newDir)
	if rn, ok := root.findId(uint64(req.NewDir)); ok {
		rn.kids[req.NewName] = n.kids[req.OldName]
		p_out("rn: %q\n", rn)
		delete(n.kids, req.OldName)
		return nil
	}
	return fuse.ENOENT
}

func (n *DFSNode) Symlink(ctx context.Context, req *fuse.SymlinkRequest) (fs.Node, error) {
	p_out("symlink: \nreq: %q \nn: %q\n\n", req, n)
	return nil, nil
}

func (n *DFSNode) Readlink(ctx context.Context, req *fuse.ReadlinkRequest) (string, error) {
	p_out("readlink: \nreq: %q \nn: %q\n\n", req, n)
	return "", nil
}
func (n *DFSNode) Link(ctx context.Context, req *fuse.LinkRequest, old fs.Node) (fs.Node, error) {
	p_out("link: \nreq: %q \nn: %q \nold: %q\n", req, n, old)
	return nil, nil
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
