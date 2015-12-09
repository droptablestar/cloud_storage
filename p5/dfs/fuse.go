package dfs

import (
	"os"
	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"

	"golang.org/x/net/context"
)

//=============================================================================

func (n *DNode) Lookup(ctx context.Context, name string) (fs.Node, error) {
	in()
	findDNode(n)
	// p_out("Lookup for %q in \n%q\n", name, n)
	if child, ok := n.kids[name]; ok { // in memory
		// p_out("IN MEMORY\n\n")
		out()
		return child, nil
	}
	if child, ok := n.ChildSigs[name]; ok { // not in memory
		// p_out("ON DISK\n\n")
		node := getDNode(child)
		if node == nil {
			if node = getRemoteDNode(n.Owner, child); node == nil {
				out()
				return nil, fuse.ENOENT // doesn't exist
			}
		}
		node.parent = n
		node.Parent = n.Attrs.Inode
		node.sig = child
		n.kids[name] = node
		nodeMap[node.Attrs.Inode] = node
		out()
		return node, nil
	}
	out()
	return nil, fuse.ENOENT // doesn't exist
}

func (n *DNode) Attr(ctx context.Context, attr *fuse.Attr) error {
	in()
	findDNode(n)
	// p_out("Attr %q <- \n%q\n\n", attr, n)
	*attr = n.Attrs
	out()
	return nil
}

func (n *DNode) Getattr(ctx context.Context, req *fuse.GetattrRequest, resp *fuse.GetattrResponse) error {
	in()
	findDNode(n)
	// p_out("Getattr for %q in \n%q\n\n", req, n)
	resp.Attr = n.Attrs
	out()
	return nil
}

func (n *DNode) Setattr(ctx context.Context, req *fuse.SetattrRequest, resp *fuse.SetattrResponse) error {
	in()
	findDNode(n)
	// p_out("Setattr for %q in \n%q\n\n", req, n)
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
	resp.Attr = n.Attrs

	out()
	return nil
}

func (n *DNode) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	in()
	findDNode(n)
	p_out("Mkdir %q in \n%q\n\n", req, n)
	d := new(DNode)
	d.init(req.Name, req.Mode)
	d.Attrs.Uid = req.Header.Uid
	d.Attrs.Gid = req.Header.Gid
	d.sig = shaString(Marshal(d))
	d.parent = n
	d.Parent = n.Attrs.Inode

	n.kids[req.Name] = d

	nodeMap[d.Attrs.Inode] = d
	markDirty(d)

	out()
	return d, nil
}

func (n *DNode) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	in()
	findDNode(n)
	p_out("Readdirall for %q\n\n", n)
	var dirDirs = []fuse.Dirent{}
	for _, val := range n.kids {
		dirDirs = append(dirDirs, addDirEnt(val))
	}
	for key, val := range n.ChildSigs {
		if _, ok := n.kids[key]; !ok {
			cn := getDNode(val)
			if cn == nil {
				if cn = getRemoteDNode(n.Owner, val); cn == nil {
					continue
				}
			}
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
	findDNode(n)
	// p_out("Create req: %q \nin %q\n\n", req, n)
	f := new(DNode)
	f.init(req.Name, req.Mode)
	f.sig = shaString(Marshal(f))
	f.parent = n
	f.Parent = n.Attrs.Inode

	n.kids[req.Name] = f

	nodeMap[f.Attrs.Inode] = f
	markDirty(f)

	out()
	return f, f, nil
}

func (n *DNode) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	in()
	findDNode(n)
	// p_out("Write req: %q\nin %q\n\n", req, n)
	olen := uint64(len(n.data))
	wlen := uint64(len(req.Data))
	offset := uint64(req.Offset)
	limit := offset + wlen

	if limit > olen {
		if n.Attrs.Size > uint64(len(n.data)) {
			n.data = n.readall()
		}
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
	findDNode(n)
	p_out("Readall: %q\n\n", n)
	b = n.readall()
	n.data = b
	out()
	return b, nil
}

func (n *DNode) readall() (b []byte) {
	for _, dblk := range n.DataBlocks {
		if n.Owner != Merep.Pid {
			p_out("Requesting block %s from %d\n", dblk, n.Owner)
			var enc_reply []byte
			req := prepare_request(dblk, Merep.Pid)
			Clients[n.Owner].Call("Node.ReqData", req, &enc_reply)
			p_out("enc_reply: [%s]\n", sha256bytesToString(enc_reply))

			reply := accept_response(enc_reply)
			if reply.Ack {
				b = append(b, reply.Block...)
			} else {
				p_err("Block request %s to %d failed\n", dblk, n.Owner)
			}
		} else {
			b = append(b, getBlock(dblk)...)
		}
	}
	return
}

func (n *DNode) Fsync(ctx context.Context, req *fuse.FsyncRequest) error {
	in()
	// p_out("fsync for %q\n\n", n)
	out()
	return nil
}

func (n *DNode) Flush(ctx context.Context, req *fuse.FlushRequest) error {
	in()
	findDNode(n)
	p_out("Flush %q \nin %q\n\n", req, n)
	if n.dirty {
		n.Attrs.Atime = time.Now()
		n.Attrs.Mtime = time.Now()
		n.DataBlocks = putBlocks(n.data)
		n.Owner = Merep.Pid
		n.sig = shaString(Marshal(n))

		n.dirty = false
	}
	out()
	return nil
}

func (n *DNode) Remove(ctx context.Context, req *fuse.RemoveRequest) (err error) {
	in()
	findDNode(n)
	err = fuse.ENOENT
	// p_out("Remove %q from \n%q \n\n", req, n)
	// If the DNode exists...delete it.
	nid := -1
	if val, ok := n.kids[req.Name]; ok {
		nid = int(val.Attrs.Inode)
		delete(n.kids, req.Name)
		err = nil
	}
	if _, ok := n.ChildSigs[req.Name]; ok {
		delete(n.ChildSigs, req.Name)
		err = nil
	}
	if err == nil {
		markDirty(n)
		if nid >= 0 {
			if _, ok := nodeMap[uint64(nid)]; ok {
				nodeMap[uint64(nid)] = nil
			}
		}
	}
	out()
	return
}

func (n *DNode) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	in()
	findDNode(n)
	if outDir, ok := newDir.(*DNode); ok {
		// p_out("Rename: \nreq: %q \nn: %q \nnew: %q\n\n", req, n, outDir)

		if child, ok := n.kids[req.OldName]; ok {
			child.Name = req.NewName
			outDir.kids[req.NewName] = child
			delete(n.kids, req.OldName)
		} else {
			outDir.ChildSigs[req.NewName] = n.ChildSigs[req.OldName]
			outDir.kids[req.NewName] = getDNode(n.ChildSigs[req.OldName])
		}

		delete(n.ChildSigs, req.OldName)
		markDirty(n)
		markDirty(outDir.kids[req.NewName])

		out()
		return nil
	}
	out()
	return fuse.ENOENT
}
