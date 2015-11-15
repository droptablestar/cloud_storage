package dfs

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"io/ioutil"
	"log"
	"net"
	"net/rpc"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var db *leveldb.DB

const (
	HASHLEN     = 32
	THE_PRIME   = 31
	MINCHUNK    = 2048
	TARGETCHUNK = 4096
	MAXCHUNK    = 8192
)

var b uint64 = 0
var saved [256]uint64

//=============================================================================
func initStore(newfs bool, dbPath string) {
	var err error

	if newfs {
		os.RemoveAll(dbPath)
	}
	db, err = leveldb.OpenFile(dbPath, nil)

	if err != nil {
		panic("no open db\n")
	}
}

// returns len of next chunk
func rkChunk(buf []byte) int {
	var off uint64
	var hash uint64
	var b_n uint64
	if b == 0 {
		b = THE_PRIME
		b_n = 1
		for i := 0; i < (HASHLEN - 1); i++ {
			b_n *= b
		}
		for i := uint64(0); i < 256; i++ {
			saved[i] = i * b_n
		}
	}

	for off = 0; off < HASHLEN && off < uint64(len(buf)); off++ {
		hash = hash*b + uint64((buf[off]))
	}

	for off < uint64(len(buf)) {
		hash = (hash-saved[buf[off-HASHLEN]])*b + uint64(buf[off])
		off++

		if (off >= MINCHUNK && ((hash % TARGETCHUNK) == 1)) || (off >= MAXCHUNK) {
			return int(off)
		}
	}
	return int(off)
}

// return base64 (stringified) version of sha1 hash of array of bytes
func shaString(buf []byte) string {
	h := sha1.Sum(buf)
	return base64.StdEncoding.EncodeToString(h[:])
}

// Use rk fingerprints to chunkify array of data. Take the
// stringified sha1 hash of each such chunk and use as key
// to store in key-value store. Return array of such strings.
func putBlocks(data []byte) (s []string) {
	off := 0
	for off < len(data) {
		ret := rkChunk(data[off:])
		// p_out("offset: %d, length: %d\n", off, ret)
		s = append(s, putBlock(data[off:(off+ret)]))
		off += ret
	}
	return
}

// puts a block of data at key defined by hash of data. Return ASCII hash.
func putBlock(data []byte) string {
	sig := shaString(data)
	if err := putBlockSig(sig, data); err == nil {
		db.Put([]byte(sig), data, nil)
		return sig
	} else {
		panic(fmt.Sprintf("FAIL: putBlock(%s): [%q]\n", sig, err))
	}
}

// store data at a specific key, used for "head"
func putBlockSig(s string, data []byte) error {
	return db.Put([]byte(s), data, nil)
}

// []byte or nil
func getBlock(key string) []byte {
	if val, err := db.Get([]byte(key), nil); err == nil {
		return val
	}
	return nil
}

func marshal(toMarshal interface{}) []byte {
	if buf, err := json.MarshalIndent(toMarshal, "", " "); err == nil {
		return buf
	} else {
		panic(fmt.Sprintf("Couldn't marshall %q\n", err))
	}
}

func getDNode(sig string) *DNode {
	n := new(DNode)
	if val, err := db.Get([]byte(sig), nil); err == nil {
		json.Unmarshal(val, &n)
		n.kids = make(map[string]*DNode)
		return n
	} else {
		p_out("ERROR: getDNode [%s]\n", err)
		return nil
	}
}

func markDirty(n *DNode) {
	for ; n.parent != nil; n = n.parent {
		n.metaDirty = true
	}
	n.metaDirty = true
}

func inArchive(n *DNode) bool {
	for ; n != nil; n = n.parent {
		if n.archive {
			return true
		}
	}
	return false
}

func flush(n *DNode) string {
	for _, val := range n.kids {
		if val.metaDirty {
			// p_out("flush(): %q\n", val)
			n.metaDirty = true // sanity check
			n.ChildSigs[val.Name] = flush(val)
		}
	}
	if n.metaDirty {
		// p_out("flushing: %q\n", n)
		n.Attrs.Atime = time.Now()
		n.Attrs.Mtime = time.Now()
		n.Version = version
		n.PrevSig = putBlock(marshal(n))
		n.sig = n.PrevSig
		n.metaDirty = false
	}
	return n.sig
}

func peteTime(s string) (time.Time, bool) {
	timeFormats := []string{"2006-1-2 15:04:05", "2006-1-2 15:04", "2006-1-2",
		"1-2-2006 15:04:05", "1-2-2006 15:04", "1-6-2006", "2006/1/2 15:04:05",
		"2006/1/2 15:04", "2006/1/2", "1/2/2006 15:04:05", "1/2/2006 15:04", "1/2/2006"}
	loc, _ := time.LoadLocation("Local")

	for _, v := range timeFormats {
		if tm, terr := time.ParseInLocation(v, s, loc); terr == nil {
			return tm, false
		}
	}
	return time.Time{}, true
}

func (top *DNode) timeTravel(tm time.Time) *DNode {
	if tm.After(top.Attrs.Atime) {
		return top
	}
	var preTop *DNode
	for top.PrevSig != "" {
		if preTop = getDNode(top.PrevSig); preTop == nil {
			break
		}
		// p_out("preTop: %d, top: %d\n", preTop.Version, top.Version)
		p_out("preTop: %s, top: %s\n", preTop.Attrs.Atime, top.Attrs.Atime)
		p_out("preTop: %t, top: %t\n",
			tm.After(preTop.Attrs.Atime), tm.Before(top.Attrs.Atime))
		if tm.After(preTop.Attrs.Atime) &&
			tm.Before(top.Attrs.Atime) {
			return preTop
		}
		top = preTop
	}
	return top
}

type Replica struct {
	Name  string
	Pid   int
	Mount string
	Db    string
	Addr  string
	Port  int
}

func (r *Replica) String() string {
	return fmt.Sprintf("name: [%s] pid: [%d] mount: [%s] db: [%s] addr: [%s] port: [%d]\n",
		r.Name, r.Pid, r.Mount, r.Db, r.Addr, r.Port)
}

type serverConn struct {
	conn *rpc.Client
	port int
	Addr string
}

func (s *serverConn) String() string {
	return fmt.Sprintf("port: [%d] addr: [%s]\n", s.port, s.Addr)
}

//=====================================================================
// This is for the client.
//=====================================================================
func NewServerConn(ip string, port int) *serverConn {
	return &serverConn{port: port, Addr: ip + fmt.Sprintf(":%d", port)}
}

func (s serverConn) Call(str string, args interface{}, reply interface{}) {
	for {
		for s.conn == nil {
			s.conn, _ = rpc.Dial("tcp", s.Addr)
		}

		if err := s.conn.Call(str, args, reply); err == nil {
			return
		}
		s.conn = nil
	}
}

//=====================================================================
// This is for the server.
//=====================================================================
func ServeInterface(ip string, port int, arg interface{}) {
	rpc.Register(arg)
	l, e := net.Listen("tcp", ip+fmt.Sprintf(":%d", port))
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go func() {
		for {
			conn, _ := l.Accept()
			go rpc.ServeConn(conn)
		}
	}()
}

//=====================================================================

type Args struct {
	A, B int
}

type Arith int

func (t *Arith) Multiply(args *Args, reply *int) error {
	*reply = args.A * args.B
	return nil
}

// From a NAT, host 'hub' has both a local address (192.168.1.13), and
// an internet-visible address. Use the one appropriate for the local
// machine.
func sameNet(s1, s2 string) bool {
	re := regexp.MustCompile(`\d+\.\d+`)
	p1 := re.FindString(s1)
	p2 := re.FindString(s2)
	return p1 == p2
}

var hostname string
var hostip string

var Replicas map[int]*Replica
var Merep *Replica
var pid int

var Debug = false

func LoadConfig(rstr, fname string) {
	rnames := strings.Split(rstr, ",")
	myname, rnames := rnames[0], rnames[1:]

	hostname, _ = os.Hostname()
	addrs, _ := net.LookupHost(hostname)

	hostip = addrs[0]
	p_out("loading %q, hostname %q, hostip %q\n", fname, hostname, hostip)

	ldata, lerr := ioutil.ReadFile(fname)
	p_dieif(lerr != nil, "NO read config: %v\n", lerr)

	// for matching below
	hostname = strings.Replace(hostname, ".local", "", 1)

	// init our Replicas map
	Replicas = make(map[int]*Replica)

	p_out("read %d\n", len(ldata))
	for _, ln := range strings.Split(string(ldata), "\n") {
		if (len(ln) < 5) || (ln[0] == '#') {
			continue
		}

		var lname, lpid, lmount, ldb, laddr, lport, laddr2, lport2 string

		flds := strings.Split(ln, ",")
		if !contains(rnames, flds[0]) && flds[0] != myname {
			continue
		}
		switch len(flds) {
		case 6:
			lname, lpid, lmount, ldb, laddr, lport = flds[0], flds[1], flds[2], flds[3], flds[4], flds[5]
		case 8:
			lname, lpid, lmount, ldb, laddr, lport = flds[0], flds[1], flds[2], flds[3], flds[4], flds[5]
			laddr2, lport2 = flds[6], flds[7]
		default:
			p_die("bad line in config file %q\n", ln)
		}
		if (laddr2 != "") && (sameNet(laddr2, hostip)) {
			laddr = laddr2
			lport = lport2
		}
		rep := &Replica{
			Name:  lname,
			Mount: lmount,
			Db:    ldb,
			Addr:  laddr,
		}
		rep.Pid, _ = strconv.Atoi(lpid)
		rep.Port, _ = strconv.Atoi(lport)

		p_out("myname [%s] Merep [%s] hostname [%s]\n", myname, Merep, hostname)
		if (myname == rep.Name) || ((myname == "auto") && (Merep == nil) && (hostname == rep.Name)) {
			Merep = rep
			pid = rep.Pid
			p_err("I'm %q, pid %d\n", rep.Name, pid)
		}

		Replicas[rep.Pid] = rep
	}
	p_dieif(Merep == nil, "Cannot find my name (%q/%q) in config (%q)!\n", myname, hostip, fname)
	p_err("Replicas:\n")
	for _, r := range Replicas {
		p_err("\t%2d) %10s, %15s/%2d\n", r.Pid, r.Name, r.Addr, r.Port)
	}
}

func p_out(s string, args ...interface{}) {
	if !Debug {
		return
	}
	log.Printf(s, args...)
}

func p_err(s string, args ...interface{}) {
	log.Printf(s, args...)
}

func p_dieif(b bool, s string, args ...interface{}) {
	if b {
		fmt.Printf(s, args...)
		os.Exit(1)
	}
}

func p_die(s string, args ...interface{}) {
	fmt.Printf(s, args...)
	os.Exit(1)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
