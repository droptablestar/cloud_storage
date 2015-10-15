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
	"strings"
)

//=============================================================================

// ...   modified versions of your fuse call implementations from P1

func (d *DNode) String() string {
	// return fmt.Sprintf("Version: %d, Name: %s, Attrs: {%q}, PrevSig: %s, ChildSigs: %#v, DataBlocks: %#v, sig: [%q], parent: %q, meta: %t, kids: %#v, data: [%s]\n",
	// 	d.Version, d.Name, d.Attrs, d.PrevSig, d.ChildSigs, d.DataBlocks, d.sig, d.parent, d.metaDirty, d.kids, d.data)
	return fmt.Sprintf("Version: %d, Name: %s, Attrs: {%q}, PrevSig: %s, ChildSigs: %#v, DataBlocks: %#v, sig: [%s], Parent: [%v], meta: %t, kids: %#v, archive: %t\n",
		d.Version, d.Name, d.Attrs, d.PrevSig, d.ChildSigs, d.DataBlocks, d.sig, d.parent, d.metaDirty, d.kids, d.archive)
}

func (n *DNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	in()
	// p_out("Lookup for %q in \n%q\n", name, n)
	if child, ok := n.kids[name]; ok { // in memory
		// p_out("IN MEMORY\n\n")
		out()
		return child, nil
	}
	if child, ok := n.ChildSigs[name]; ok { // not in memory
		// p_out("ON DISK\n\n")
		node := getDNode(child)
		node.parent = n
		node.sig = child
		n.kids[name] = node
		out()
		return node, nil
	}
	out()
	return nil, fuse.ENOENT // doesn't exist
}

func (n *DNode) Attr(ctx context.Context, attr *fuse.Attr) error {
	in()
	p_out("Attr %q <- \n%q\n\n", attr, n)
	*attr = n.Attrs
	out()
	return nil
}

func (n *DNode) Getattr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error {
	in()
	p_out("Getattr for %q in \n%q\n\n", req, n)
	resp.Attr = n.Attrs
	out()
	return nil
}

func (n *DNode) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	in()
	p_out("Setattr for %q in \n%q\n\n", req, n)
	if inPast || inArchive(n) {
		out()
		return fuse.EPERM
	}
	// Setattr() should only be allowed to modify particular parts of a
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

	out()
	return nil
}

func (n *DNode) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	in()
	p_out("Mkdir %q in \n%q\n\n", req, n)
	if inPast || inArchive(n) {
		out()
		return nil, fuse.EPERM
	}
	split := strings.Split(req.Name, "@")
	if len(split) > 1 {
		p_out("SPLIT\n")
		p_out("split %q\n", split)
		if _, ok := n.ChildSigs[split[0]]; !ok {
			p_out("split[0] %q\n", split[0])
			p_out("children: %q\n", n.ChildSigs)
			return nil, fuse.ENOENT
		}
		var tm time.Time
		var ok bool
		if tm, ok = peteTime(split[1]); ok {
			var td time.Duration
			var err error
			if td, err = time.ParseDuration(split[1]); err != nil {
				return nil, fuse.ENOENT
			}
			tm = time.Now().Add(td)
			p_out("tm %s\n", tm)
		}
		tN := getDNode(n.ChildSigs[split[0]]).timeTravel(tm)
		tN.Name = req.Name
		tN.metaDirty = false
		tN.archive = true

		n.kids[req.Name] = tN
		out()
		return tN, nil
	}

	d := new(DNode)
	d.init(req.Name, req.Mode)
	d.Attrs.Uid = req.Header.Uid
	d.Attrs.Gid = req.Header.Gid
	d.sig = shaString(marshal(d))
	d.parent = n
	n.kids[req.Name] = d

	markDirty(d)

	out()
	return d, nil
}

// TODO: This seems verbose. Can I find a better way to copy the data out?
func (n *DNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	in()
	p_out("Readdirall for %q\n\n", n)
	var dirDirs = []fuse.Dirent{}
	for _, val := range n.kids {
		p_out("ADDING: %q\n", val)
		dirDirs = append(dirDirs, addDirEnt(val))
	}
	for key, val := range n.ChildSigs {
		p_out("CHILD: %q\n", key)
		if _, ok := n.kids[key]; !ok {
			cn := getDNode(val)
			dirDirs = append(dirDirs, addDirEnt(cn))
		}
	}
	out()
	return dirDirs, nil
}

// helper function for adding directory entries
func addDirEnt(n *DNode) fuse.Dirent {
	typ := fuse.DT_Unknown
	if n.Attrs.Mode.IsDir() {
		typ = fuse.DT_Dir
	}

	if n.Attrs.Mode.IsRegular() {
		typ = fuse.DT_File
	}

	if n.Attrs.Mode&os.ModeType == os.ModeSymlink {
		typ = fuse.DT_Link
	}
	return fuse.Dirent{Inode: n.Attrs.Inode, Type: typ, Name: n.Name}
}

func (n *DNode) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {
	in()
	if inPast || inArchive(n) {
		out()
		return nil, nil, fuse.EPERM
	}
	p_out("Create req: %q \nin %q\n\n", req, n)
	f := new(DNode)
	f.init(req.Name, req.Mode)
	f.sig = shaString(marshal(f))
	f.parent = n

	n.kids[req.Name] = f

	markDirty(f)

	out()
	return f, f, nil
}

func (n *DNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	in()
	p_out("Write req: %q\nin %q\n\n", req, n)
	if inPast || inArchive(n) {
		out()
		return fuse.EPERM
	}
	olen := uint64(len(n.data))
	wlen := uint64(len(req.Data))
	offset := uint64(req.Offset)
	limit := offset + wlen

	if limit > olen {
		if n.Attrs.Size > uint64(len(n.data)) {
			n.data = n.readall()
		}
		// n.data = n.readall()
		t := make([]byte, limit)
		copy(t, n.data)
		n.data = t
		n.Attrs.Size = limit
	}
	resp.Size = copy(n.data[offset:], req.Data)
	n.dirty = true
	markDirty(n)

	out()
	return nil
}

func (n *DNode) ReadAll(ctx context.Context) (b []byte, e error) {
	in()
	p_out("Readall: %q\n\n", n)
	b = n.readall()
	out()
	return b, nil
}

func (n *DNode) readall() (b []byte) {
	for _, dblk := range n.DataBlocks {
		b = append(b, getBlock(dblk)...)
	}
	return
}

func (n *DNode) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	in()
	p_out("fsync for %q\n\n", n)
	out()
	return nil
}

func (n *DNode) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	in()
	p_out("Flush %q \nin %q\n\n", req, n)
	if n.dirty {
		n.Attrs.Atime = time.Now()
		n.Attrs.Mtime = time.Now()
		n.DataBlocks = putBlocks(n.data)
		n.sig = shaString(marshal(n))

		n.dirty = false
	}
	out()
	return nil
}

func (n *DNode) Remove(ctx context.Context, req *fuse.RemoveRequest) (err error) {
	in()
	p_out("Remove %q from \n%q \n\n", req, n)
	if inPast || inArchive(n) {
		out()
		return fuse.EPERM
	}
	err = fuse.ENOENT
	// If the DNode exists...delete it.
	if val, ok := n.kids[req.Name]; ok {
		if val.Attrs.Mode&os.ModeType == os.ModeSymlink {
			n.Attrs.Nlink -= 1
		}
		delete(n.kids, req.Name)
		err = nil
	}
	if _, ok := n.ChildSigs[req.Name]; ok {
		delete(n.ChildSigs, req.Name)
		err = nil
	}
	if err == nil {
		markDirty(n)
	}
	out()
	return
}

func (n *DNode) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	in()
	if inPast || inArchive(n) {
		out()
		return fuse.EPERM
	}
	if outDir, ok := newDir.(*DNode); ok {
		p_out("Rename: \nreq: %q \nn: %q \nnew: %q\n\n", req, n, outDir)

		n.kids[req.OldName].Name = req.NewName
		outDir.kids[req.NewName] = n.kids[req.OldName]

		delete(n.kids, req.OldName)
		delete(n.ChildSigs, req.OldName)
		markDirty(n)
		markDirty(outDir.kids[req.NewName])

		out()
		return nil
	}
	out()
	return fuse.ENOENT
}

func (n *DNode) Symlink(ctx context.Context, req *fuse.SymlinkRequest) (fs.Node, error) {
	in()
	p_out("Symlink: \nreq: %q \nn: %q\n\n", req, n)
	link := *new(DNode)
	link.Attrs = *new(fuse.Attr)
	// for some reason redis was trying to link to a file that didn't exist
	if _, ok := n.kids[req.Target]; ok {
		link.Attrs.Mode = os.ModeSymlink | 0755
		n.Attrs.Nlink += 1
		n.kids[req.NewName] = &link

		out()
		return &link, nil
	}
	out()
	return nil, fuse.ENOENT
}

func (n *DNode) Readlink(ctx context.Context, req *fuse.ReadlinkRequest) (string, error) {
	in()
	p_out("Readlink: \nreq: %q \nn: %q\n\n", req, n)
	out()
	return n.Name, nil
}

func (n *DNode) Link(ctx context.Context, req *fuse.LinkRequest, old fs.Node) (fs.Node, error) {
	in()
	p_out("Link: \nreq: %q \nn: %q \nold: %q\n", req, n, old)
	if oldDir, ok := old.(*DNode); ok {
		n.kids[req.NewName] = oldDir
		return oldDir, nil
	}
	out()
	return nil, fuse.ENOENT
}

//=============================================================================

// var Usage = func() {
// 	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
// 	fmt.Fprintf(os.Stderr, "  %s MOUNTPOINT\n", os.Args[0])
// 	flag.PrintDefaults()
// }
