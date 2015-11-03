/*
   What this code shows:
   - how to initialize and connect REQ/REP and PUB/SUB sockets
   - a way to differentiate ports
   - sending/receiving on different types of ZMQ sockets
   - parsing of required command-line arguments
   - parsing of config file.
*/

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	. "github.com/mattn/go-getopt"
	zmq "github.com/pebbe/zmq4"
	"io/ioutil"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const DEF_PORT = 3000

// The REQ port is the default, others are offset
func REQ_PORT(x int) int     { return x }
func CONSOLE_PORT(x int) int { return x + 1 }
func SERVER_PORT(x int) int  { return x + 2 }
func PUB_PORT(x int) int     { return x + 3 }
func SUB_PUB_PORT(x int) int { return x + 4 }
func RAFT_PORT(x int) int    { return x + 5 }

const (
	_ = iota // I want the constants to start at "1"
	MSG_FLUSH
	MSG_CONSOLE
	MSG_REQ_CHUNKS
	MSG_REP_CHUNKS
)

var msgtypes = map[int]string{
	MSG_FLUSH:      "MSG_FLUSH",
	MSG_CONSOLE:    "MSG_CONSOLE",
	MSG_REQ_CHUNKS: "MSG_REQ_CHUNKS",
	MSG_REP_CHUNKS: "MSG_REP_CHUNKS",
}

// All of this is application-specific. ZMQ will ignore it. You will
// have to augment this...
type Message struct {
	Seqnum int // debugging
	Mtype  int
	From   int
	To     int
	Len    int
	S      string
}

type Replica struct {
	name    string
	pid     int
	mount   string
	db      string
	addr    string
	port    int
	reqSock *zmq.Socket
	live    bool // true after we've received a flush from them
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

func send(sock *zmq.Socket, m *Message) error {
	m.From = pid
	m.Seqnum = nextSeqNum
	nextSeqNum++
	p_out("SEND %q: %v\n", msgtypes[m.Mtype], m)
	s, _ := json.Marshal(m)
	bytes, err := sock.Send(string(s), 0)
	if (err != nil) || (bytes != len(s)) {
		p_err("SEND error, %d bytes, err: %v\n", bytes, err)
		return errors.New("SEND error")
	}
	return nil
}

func recv(sock *zmq.Socket) (*Message, error) {
	m := new(Message)
	str, _ := sock.Recv(0)
	if err := json.Unmarshal([]byte(str), m); err != nil {
		p_err("ERROR unmarshaling message %q\n", string(str))
	}
	p_out("\tRECV %q: %v\n", msgtypes[m.Mtype], m)
	return m, nil
}

//=============================================================================

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

	// Connect a local REQ socket w/ a REP socket on each replica. The
	// code works whether or not they exist.
	for _, r := range replicas {
		r.reqSock, _ = zmq.NewSocket(zmq.REQ)
		s := fmt.Sprintf("tcp://%s:%d", r.addr, REQ_PORT(r.port))
		if err := r.reqSock.Connect(s); err != nil {
			p_die("ERROR connecting REP socket %q (%v)\n", s, err)
		}
	}
	p_out("Bound request sockets to each remote replica\n")

	go flushHandler()
	go blockReplyer()
	go flusher()

	for {
		time.Sleep(10 * time.Second)

		// For this test code, only the replica with ID = 1 sends
		// requests, and only to those other replicas it has received
		// flushes from.
		if pid == 1 {
			for _, r := range replicas {
				if (r != merep) && r.live {
					msg := &Message{Mtype: MSG_REQ_CHUNKS, From: pid, S: "bogus hash sig"}
					send(r.reqSock, msg)
					recv(r.reqSock)
				}
			}
		}
	}
}

func blockReplyer() {
	// Create a single reply socket (REP) to handle all incoming block
	// requests.

	sock, err := zmq.NewSocket(zmq.REP)
	s := fmt.Sprintf("tcp://*:%d", REQ_PORT(merep.port))
	if err = sock.Bind(s); err != nil {
		p_die("ERROR binding REP socket %q\n", s)
	}

	for {
		m, _ := recv(sock)
		p_out("\tblock replyer got %q, %v\n", msgtypes[m.Mtype], m)

		m.Mtype = MSG_REP_CHUNKS
		p_out("\tSending backd reply\n")

		// zmq know where to send the reply, despite being connected
		// to multiple requesters
		send(sock, m)
	}
}

func flushHandler() {
	subsock, err := zmq.NewSocket(zmq.SUB)
	p_dieif(err != nil, "Bad SUB sock in flushHandler(), %v\n", err)
	subsock.SetSubscribe("")

	// one-time setup
	for rid, r := range replicas {
		if r.pid == pid {
			p_out("SKIPPING pid %d\n", pid)
			continue
		}

		addr := fmt.Sprintf("tcp://%s:%d", r.addr, PUB_PORT(r.port))
		if err := subsock.Connect(addr); err != nil {
			p_die("ERROR: connect to publisher %d, %q, %v\n", rid, addr, err)
		}
	}

	// main loop
	for {
		m, _ := recv(subsock)
		if m.Mtype == MSG_FLUSH {
			in()
			replicas[m.From].live = true
			out()
			// never reply to a PUB socket
		} else {
			p_die("Bad msg type\n")
		}
	}
}

func in() {
}

func out() {
}

func flusher() {
	pubsock, err := zmq.NewSocket(zmq.PUB)
	if err != nil {
		p_out("fl skt create err %v\n", err)
	}
	p_out("flusher created\n")

	s := fmt.Sprintf("tcp://*:%d", PUB_PORT(merep.port))
	err = pubsock.Bind(s)
	if err != nil {
		p_out("fl sock BIND err %v\n", err)
	}
	p_out("flusher bound\n")

	for {
		time.Sleep(5 * time.Second)
		msg := &Message{Mtype: MSG_FLUSH, From: pid, S: "bogus hash sig"}
		send(pubsock, msg)
	}
}

// sends flush on the PUB socket
func flushChanges(sock *zmq.Socket) {
	p_out("sending flush\n")

	//	for _, p := range pairs {
	//		p_out("\tFlushing %q\n", string(p.Hash))
	// flush to leveldb
	//	}
	// storePutHead(roothash)
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
