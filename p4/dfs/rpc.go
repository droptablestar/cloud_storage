package dfs

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/rpc"
	"os"
	"regexp"
	"strconv"
	"strings"
)

//=====================================================================
// RPC code
//=====================================================================

var hostname string
var hostip string
var pid int

var Replicas map[int]*Replica
var Merep *Replica
var Clients []*serverConn

var Debug = false

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

type Node DNode

func (n *Node) string() string {
	return fmt.Sprintf("%q\n", DNode(*n))
}

type Response struct {
	Ack bool
	Pid int
}

func (r *Response) String() string {
	return fmt.Sprintf("Ack: [%t] Pid: %d\n", r.Ack, r.Pid)
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

func (nd *Node) Receive(n *DNode, reply *Response) error {
	p_out("received %q from %d\n", n, n.Owner)
	n.PrevSig = putBlock(marshal(n))
	n.sig = n.PrevSig

	if n.Attrs.Inode > nextInd {
		nextInd = n.Attrs.Inode
	} else if n.Attrs.Inode == nextInd {
		nextInd++
	}

	if child, ok := nodeMap[n.Attrs.Inode]; ok { // in map
		p_out("overwriting childSigs %q with n\n", child)
		child.Name = n.Name
		child.Attrs = n.Attrs
		child.Version = n.Version
		child.PrevSig = n.PrevSig
		child.ChildSigs = n.ChildSigs
		child.DataBlocks = n.DataBlocks
		child.Owner = n.Owner
		child.kids = make(map[string]*DNode)
		p_out("new n = %q\n", child)
	} else {
		p_out("overwriting %q with n\n", nodeMap[n.Attrs.Inode])
		nodeMap[n.Attrs.Inode] = n
		p_out("new n = %q\n", nodeMap[n.Attrs.Inode])
	}
	if n.Attrs.Inode == root.Attrs.Inode {
		head.Root = n.PrevSig
		head.NextInd = nextInd
		putBlockSig("head", marshal(head))
	}

	reply.Ack = true
	reply.Pid = Merep.Pid
	return nil
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

// From a NAT, host 'hub' has both a local address (192.168.1.13), and
// an internet-visible address. Use the one appropriate for the local
// machine.
func sameNet(s1, s2 string) bool {
	re := regexp.MustCompile(`\d+\.\d+`)
	p1 := re.FindString(s1)
	p2 := re.FindString(s2)
	return p1 == p2
}

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

		p_out("myname [%s] Merep [%q] hostname [%s]\n", myname, Merep, hostname)
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
	p_err("\n")
}
