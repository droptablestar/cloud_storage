//
package dfs

/*
 Two main files are ../fuse.go and ../fs/serve.go
*/

import (
	"fmt"
	"os"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"

	"golang.org/x/net/context"
)

//=============================================================================

// ...   modified versions of your fuse call implementations from P1

func (d *DNode) String() string {
	// return fmt.Sprintf("nid: %d, Name: %s, attr: {%q}, dirty: %t, kids: %#v, data: %s\n",
	// 	d.nid, d.Name, d.Attrs, d.dirty, d.kids, d.data)
	return fmt.Sprintf("Version: %d, Name: %s, Attrs: {%q}, ParentSig: %s, PrevSig: %s\n",
		d.Version, d.Name, d.Attrs, d.ParentSig, d.PrevSig)
}

// type FS struct{}

// var root *DNode

// func (FS) Root() (fs.Node, error) {
// 	root.Attrs.Inode = 1
// 	return root, nil
// }

func (n *DNode) Attr(ctx context.Context, attr *fuse.Attr) error {
	// p_out("attr %q <- \n%q\n\n", attr, n)
	*attr = n.Attrs
	return nil
}

func (n *DNode) Getattr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error {
	// p_out("getattr for %q in \n%q\n\n", req, n)
	resp.Attr = n.Attrs
	return nil
}

func (n *DNode) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	// p_out("attr for %q in \n%q\n\n", req, n)
	// Setattr() should only be allowed to modify particular parts of a
	// nodes attributes.
	if req.Valid.Mode() {
		n.Attrs.Mode = req.Mode
	}
	if req.Valid.Uid() {
		n.Attrs.Uid = req.Uid
	}
	if req.Valid.Gid() {
		n.Attrs.Gid = req.Gid
	}
	if req.Valid.Size() {
		n.Attrs.Size = req.Size
		n.data = n.data[:req.Size]
	}
	if req.Valid.Atime() {
		n.Attrs.Atime = req.Atime
	}
	if req.Valid.Mtime() {
		n.Attrs.Mtime = req.Mtime
	}
	if req.Valid.Chgtime() {
		n.Attrs.Crtime = req.Crtime
	}
	if req.Valid.Crtime() {
		n.Attrs.Crtime = req.Crtime
	}
	if req.Valid.Flags() {
		n.Attrs.Flags = req.Flags
	}
	// if req.Valid.Handle() {
	// 	n.Attrs.Handle = req.Handle
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
	resp.Attr = n.Attrs

	return nil
}

func (n *DNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	// p_out("lookup for %q in \n%q\n\n", Name, n)
	if child, ok := n.kids[name]; ok {
		return child, nil
	}
	return nil, fuse.ENOENT
}

// Cut and paste :-P
func (n *DNode) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	p_out("mkdir %q in \n%q\n\n", req, n.Name)
	d := new(DNode)
	d.init(req.Name, req.Mode)
	n.kids[req.Name] = d
	n.Attrs.Uid = req.Header.Uid
	n.Attrs.Gid = req.Header.Gid
	return d, nil
}

// TODO: This seems verbose. Can I find a better way to copy the data out?
func (n *DNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	// p_out("readdirall for %q\n", n.name)
	var dirDirs = []fuse.Dirent{}
	for key, val := range n.kids {
		typ := fuse.DT_Unknown
		if val.Attrs.Mode.IsDir() {
			typ = fuse.DT_Dir
		}
		if val.Attrs.Mode.IsRegular() {
			typ = fuse.DT_File
		}

		if val.Attrs.Mode&os.ModeType == os.ModeSymlink {
			typ = fuse.DT_Link
		}
		dirDirs = append(dirDirs,
			fuse.Dirent{Inode: val.Attrs.Inode, Type: typ, Name: key})
	}
	return dirDirs, nil
}

func (n *DNode) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	p_out("create req: %q \nin %q\n\n", req, n)
	f := new(DNode)
	f.init(req.Name, req.Mode)
	n.kids[req.Name] = f

	return f, f, nil
}

func (n *DNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	p_out("write req: %q\nin %q\n", req, n)
	olen := uint64(len(n.data))
	wlen := uint64(len(req.Data))
	offset := uint64(req.Offset)
	limit := offset + wlen

	if limit > olen {
		t := make([]byte, limit)
		copy(t, n.data)
		n.data = t
		n.Attrs.Size = limit
	}
	resp.Size = copy(n.data[offset:], req.Data)
	n.dirty = true

	return nil
}

func (n *DNode) ReadAll(ctx context.Context) ([]byte, error) {
	p_out("readall: %q\n\n", n)
	return n.data, nil
}

func (n *DNode) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	// p_out("fsync for %q\n", n)
	return nil
}

func (n *DNode) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	// p_out("flush %q \nin %q\n\n", req, n)
	if n.dirty {
		n.Attrs.Atime = time.Now()
		n.Attrs.Mtime = time.Now()
		n.dirty = false
	}
	return nil
}

func (n *DNode) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	p_out("remove %q from \n%q \n\n", req, n)
	// If the DNode exists...delete it.
	if val, ok := n.kids[req.Name]; ok {
		if val.Attrs.Mode&os.ModeType == os.ModeSymlink {
			n.Attrs.Nlink -= 1
		}
		delete(n.kids, req.Name)
		return nil
	}
	return fuse.ENOENT
}

func (n *DNode) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	p_out("Rename: \nreq: %q \nn: %q \nnew: %q\n\n", req, n, newDir)
	if outDir, ok := newDir.(*DNode); ok {
		n.kids[req.OldName].Name = req.NewName
		outDir.kids[req.NewName] = n.kids[req.OldName]
		delete(n.kids, req.OldName)
		return nil
	}
	return fuse.ENOENT
}

func (n *DNode) Symlink(ctx context.Context, req *fuse.SymlinkRequest) (fs.Node, error) {
	p_out("symlink: \nreq: %q \nn: %q\n\n", req, n)
	link := *new(DNode)
	link.Attrs = *new(fuse.Attr)
	// for some reason redis was trying to link to a file that didn't exist
	if _, ok := n.kids[req.Target]; ok {
		link.Attrs.Mode = os.ModeSymlink | 0755
		n.Attrs.Nlink += 1
		n.kids[req.NewName] = &link

		return &link, nil
	}
	return nil, fuse.ENOENT
}

func (n *DNode) Readlink(ctx context.Context, req *fuse.ReadlinkRequest) (string, error) {
	p_out("readlink: \nreq: %q \nn: %q\n\n", req, n)
	return n.Name, nil
}

func (n *DNode) Link(ctx context.Context, req *fuse.LinkRequest, old fs.Node) (fs.Node, error) {
	p_out("link: \nreq: %q \nn: %q \nold: %q\n", req, n, old)
	if oldDir, ok := old.(*DNode); ok {
		n.kids[req.NewName] = oldDir
		return oldDir, nil
	}
	return nil, fuse.ENOENT
}

//=============================================================================

// var Usage = func() {
// 	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
// 	fmt.Fprintf(os.Stderr, "  %s MOUNTPOINT\n", os.Args[0])
// 	flag.PrintDefaults()
// }
