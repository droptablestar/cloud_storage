// memfs implements a simple in-memory file system.  v0.2A
package main

// NOTES:
//
//  OS X redis compile check: 9/11/15 - 13:55
//  Ubuntu redis compile check: 9/11/15 - 14:03
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
	// p_out("init: %q with name: %q and mode: %#X\n", d, name, mode)
	// had some isssues with dir's that were initially 0B size
	startTime := time.Now()
	var size uint64 = 0
	if mode.IsDir() {
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
	return fmt.Sprintf("nid: %d, name: %s, attr: {%q}, dirty: %t, kids: %#v, data: %s\n",
		d.nid, d.name, d.attr, d.dirty, d.kids, d.data)
	// return fmt.Sprintf("nid: %d, name: %s, attr: {%q}, dirty: %t, kids: %#v\n",
	// 	d.nid, d.name, d.attr, d.dirty, d.kids)
}

type FS struct{}

var root *DFSNode

func (FS) Root() (fs.Node, error) {
	root.attr.Inode = 1
	return root, nil
}

func (n *DFSNode) Attr(ctx context.Context, attr *fuse.Attr) error {
	// p_out("attr %q <- \n%q\n\n", attr, n)
	*attr = n.attr
	return nil
}

func (n *DFSNode) Getattr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error {
	// p_out("getattr for %q in \n%q\n\n", req, n)
	resp.Attr = n.attr
	return nil
}

func (n *DFSNode) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	// p_out("attr for %q in \n%q\n\n", req, n)
	// Setattr() should only be allowed to modify particular parts of a
	// nodes attributes.
	if req.Valid.Mode() {
		n.attr.Mode = req.Mode
	}
	if req.Valid.Uid() {
		n.attr.Uid = req.Uid
	}
	if req.Valid.Gid() {
		n.attr.Gid = req.Gid
	}
	if req.Valid.Size() {
		n.attr.Size = req.Size
	}
	if req.Valid.Atime() {
		n.attr.Atime = req.Atime
	}
	if req.Valid.Mtime() {
		n.attr.Mtime = req.Mtime
	}
	if req.Valid.Chgtime() {
		n.attr.Crtime = req.Crtime
	}
	if req.Valid.Crtime() {
		n.attr.Crtime = req.Crtime
	}
	if req.Valid.Flags() {
		n.attr.Flags = req.Flags
	}
	// if req.Valid.Handle() {
	// 	n.attr.Handle = req.Handle
	// }
	// if req.Valid.AtimeNow() {
	// 	return fl&SetattrAtimeNow != 0
	// }
	// if req.Valid.MtimeNow() {
	// 	return fl&SetattrMtimeNow != 0
	// }
	// if req.Valid.LockOwner() {
	// 	return fl&SetattrLockOwner != 0
	// }
	// if req.Valid.Bkuptime() {
	// 	return fl&SetattrBkuptime != 0
	// }
	resp.Attr = n.attr

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
	// p_out("mkdir %q in \n%q\n\n", req, n.name)
	d := new(DFSNode)
	d.init(req.Name, req.Mode)
	n.kids[req.Name] = d
	n.attr.Uid = req.Header.Uid
	n.attr.Gid = req.Header.Gid
	return d, nil
}

// TODO: This seems verbose. Can I find a better way to copy the data out?
func (n *DFSNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	// p_out("readdirall for %q\n", n.name)
	var dirDirs = []fuse.Dirent{}
	for key, val := range n.kids {
		typ := fuse.DT_Unknown
		if val.attr.Mode.IsDir() {
			typ = fuse.DT_Dir
		}
		if val.attr.Mode.IsRegular() {
			typ = fuse.DT_File
		}

		if val.attr.Mode&os.ModeType == os.ModeSymlink {
			typ = fuse.DT_Link
		}
		dirDirs = append(dirDirs,
			fuse.Dirent{Inode: val.attr.Inode, Type: typ, Name: key})
	}
	return dirDirs, nil
}

func (n *DFSNode) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	p_out("create req: %q \nin %q\n\n", req, n)
	f := new(DFSNode)
	f.init(req.Name, req.Mode)
	n.kids[req.Name] = f

	return f, f, nil
}

func (n *DFSNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	p_out("write req: %q\nin %q\n", req, n)
	olen := uint64(len(n.data))
	wlen := uint64(len(req.Data))
	offset := uint64(req.Offset)
	limit := offset + wlen

	if limit > olen {
		t := make([]byte, limit)
		copy(t, n.data)
		n.data = t
		n.attr.Size = limit
	}
	resp.Size = copy(n.data[offset:], req.Data)
	n.dirty = true

	return nil
}

func (n *DFSNode) ReadAll(ctx context.Context) ([]byte, error) {
	// p_out("readall: %q\n\n", n)
	return n.data, nil
}

func (n *DFSNode) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	// p_out("fsync for %q\n", n)
	return nil
}

func (n *DFSNode) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	// p_out("flush %q \nin %q\n\n", req, n)
	if n.dirty {
		n.attr.Atime = time.Now()
		n.attr.Mtime = time.Now()
		n.dirty = false
	}
	return nil
}

func (n *DFSNode) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	// p_out("remove %q from \n%q \n\n", req, n)
	// If the DFSNode exists...delete it.
	if val, ok := n.kids[req.Name]; ok {
		if val.attr.Mode&os.ModeType == os.ModeSymlink {
			n.attr.Nlink -= 1
		}
		delete(n.kids, req.Name)
		return nil
	}
	return fuse.ENOENT
}

func (n *DFSNode) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	// p_out("Rename: \nreq: %q \nn: %q \nnew: %q\n\n", req, n, newDir)
	if outDir, ok := newDir.(*DFSNode); ok {
		n.kids[req.OldName].name = req.NewName
		outDir.kids[req.NewName] = n.kids[req.OldName]
		delete(n.kids, req.OldName)
		return nil
	}
	return fuse.ENOENT
}

// func (n *DFSNode) Symlink(ctx context.Context, req *fuse.SymlinkRequest) (fs.Node, error) {
// 	p_out("symlink: \nreq: %q \nn: %q\n\n", req, n)
// 	link := *new(DFSNode)
// 	link.attr = *new(fuse.Attr)
// 	link = *n.kids[req.Target]
// 	link.attr.Mode = os.ModeSymlink | 0755
// 	n.attr.Nlink += 1
// 	p_out("link: %q\nn: %q\nkid: %q\n\n", link, n, n.kids[req.Target])
// 	n.kids[req.NewName] = &link
// 	return &link, nil
// }

// func (n *DFSNode) Readlink(ctx context.Context, req *fuse.ReadlinkRequest) (string, error) {
// 	p_out("readlink: \nreq: %q \nn: %q\n\n", req, n)
// 	return n.name, nil
// }

// func (n *DFSNode) Link(ctx context.Context, req *fuse.LinkRequest, old fs.Node) (fs.Node, error) {
// 	p_out("link: \nreq: %q \nn: %q \nold: %q\n", req, n, old)
// 	hlink := new(DFSNode)
// 	hlink.init(req.NewName, os.ModeDir)
// 	if oldDir, ok := old.(*DFSNode); ok {
// 		if ok := oldDir.Attr(ctx, &hlink.attr); ok == nil {
// 			hlink.data = make([]uint8, hlink.attr.Size)
// 			copy(hlink.data, oldDir.data)
// 			n.kids[req.NewName] = hlink
// 			return oldDir, nil
// 		}
// 	}
// 	return nil, fuse.ENOENT
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
