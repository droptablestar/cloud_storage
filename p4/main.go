package main

import (
	// "bitbucket.org/jreeseue/818/p3/dfs"
	"fmt"
	. "github.com/mattn/go-getopt"
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

type Replica struct {
	name  string
	pid   int
	mount string
	db    string
	addr  string
	port  int
	// reqSock *zmq.Socket
	live bool // true after we've received a flush from them
}

func (r *Replica) String() string {
	return fmt.Sprintf("name: [%s] pid: [%d] mount: [%s] db: [%s] addr: [%s] port: [%d]\n",
		r.name, r.pid, r.mount, r.db, r.addr, r.port)
}

type ServerConn struct {
	conn *rpc.Client
	port int
	addr string
}

func (s *ServerConn) String() string {
	return fmt.Sprintf("port: [%d] addr: [%s]\n", s.port, s.addr)
}

var merep *Replica
var replicas map[int]*Replica

var pid int
var hostname string
var hostip string
var nextSeqNum = 1

var newfs string
var debug = false
var flusherPeriod = 5
var modeConsistency = "none"
var replicaString = "auto"

//=====================================================================
// This is for the client.
//=====================================================================
func newServerConn(ip string, port int) *ServerConn {
	return &ServerConn{port: port, addr: ip + fmt.Sprintf(":%d", port)}
}

func (s ServerConn) Call(str string, args interface{}, reply interface{}) {
	for {
		p_out("conn: %p addr: %s\n", s.conn, s.addr)
		for s.conn == nil {
			s.conn, _ = rpc.Dial("tcp", s.addr)
		}

		if err := s.conn.Call(str, args, reply); err == nil {
			return
			n
		}
		s.conn = nil
	}
}

//=====================================================================
// This is for the server.
//=====================================================================
func serveInterface(ip string, port int, arg interface{}) {
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

func main() {
	var c int

	for {
		if c = Getopt("cdf:m:r:"); c == EOF {
			break
		}

		switch c {
		case 'c':
			newfs = "NEWFS "
		case 'd':
			debug = !debug
		case 'f':
			flusherPeriod, _ = strconv.Atoi(OptArg)
		case 'm':
			modeConsistency = OptArg
		case 'r':
			replicaString = OptArg
		default:
			println("usage: main.go [-c | -d | -f <flush dur> | -m <mode> | -r <rep string>]", c)
			os.Exit(1)
		}
	}
	fmt.Printf("\nStartup up with debug %v, flush period %v, %smode: %q, replicaStr  %q\n\n",
		debug, flusherPeriod, newfs, modeConsistency, replicaString)

	loadConfig(replicaString, "config.txt")

	for {
		for _, r := range replicas {
			if r == merep {
				p_out("server r: %q\n", r)
				go func() {
					serveInterface(r.addr, r.port, new(Arith))
					for {
						fmt.Printf(".")
						time.Sleep(time.Second)
					}
				}()
			} else {
				p_out("client r: %q\n", r)
				serv := newServerConn(r.addr, r.port)
				p_out("serv: %q\n", serv)
				for i := 0; i < 100; i++ {
					var reply int
					args := &Args{7, i}

					serv.Call("Arith.Multiply", args, &reply)

					time.Sleep(time.Second)
					fmt.Printf("\nArith: %d*%d=%d\n", args.A, args.B, reply)
				}
			}

		}
	}
}

func in() {
}

func out() {
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

func loadConfig(rstr, fname string) {
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

	// init our replicas map
	replicas = make(map[int]*Replica)

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
			name:  lname,
			mount: lmount,
			db:    ldb,
			addr:  laddr,
		}
		rep.pid, _ = strconv.Atoi(lpid)
		rep.port, _ = strconv.Atoi(lport)

		p_out("myname [%s] merep [%s] hostname [%s]\n", myname, merep, hostname)
		if (myname == rep.name) || ((myname == "auto") && (merep == nil) && (hostname == rep.name)) {
			merep = rep
			pid = rep.pid
			p_err("I'm %q, pid %d\n", rep.name, pid)
		}

		replicas[rep.pid] = rep
	}
	p_dieif(merep == nil, "Cannot find my name (%q/%q) in config (%q)!\n", myname, hostip, fname)
	p_err("Replicas:\n")
	for _, r := range replicas {
		p_err("\t%2d) %10s, %15s/%2d\n", r.pid, r.name, r.addr, r.port)
	}
}

func p_out(s string, args ...interface{}) {
	if !debug {
		return
	}
	fmt.Printf(s, args...)
}

func p_err(s string, args ...interface{}) {
	fmt.Printf(s, args...)
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
