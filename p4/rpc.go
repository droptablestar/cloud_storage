package main

//
// Invoke "go build rpc.go", then:
//        "./rpc" for client, and
//        "./rpc server" for server
//
// Client can deal with server crashing and restarting.
// Server can handle any number of simultaneous connections.

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"time"
)

//=====================================================================
// This is for the client.
//=====================================================================
type ServerConn struct {
	conn *rpc.Client
	port int
	addr string
}

func newServerConn(ip string, port int) *ServerConn {
	return &ServerConn{port: port, addr: ip + fmt.Sprintf(":%d", port)}
}

func (s ServerConn) Call(str string, args interface{}, reply interface{}) {
	for {
		for s.conn == nil {
			s.conn, _ = rpc.Dial("tcp", s.addr)
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

func serveInterface(ip string, port int, arg interface{}) {
	rpc.Register(arg)
	l, e := net.Listen("tcp", ip+fmt.Sprintf(":1234"))
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

type Quotient struct {
	Quo, Rem int
}

type Arith int

func (t *Arith) Multiply(args *Args, reply *int) error {
	*reply = args.A * args.B
	return nil
}

func (t *Arith) Divide(args *Args, quo *Quotient) error {
	if args.B == 0 {
		return errors.New("divide by zero")
	}
	quo.Quo = args.A / args.B
	quo.Rem = args.A % args.B
	return nil
}

//=====================================================================

const (
	IP   = "127.0.0.1"
	PORT = 1234
)

var newfs string
var debug = false
var flusherPeriod = 5
var modeConsistency = "none"
var replicaString = "auto"

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

	if len(os.Args) > 1 && os.Args[1] == "server" {
		serveInterface(IP, PORT, new(Arith))

		for {
			fmt.Printf(".")
			time.Sleep(time.Second)
		}
	} else {
		serv := newServerConn(IP, PORT)
		for i := 0; i < 100; i++ {
			var reply int
			args := &Args{7, i}

			serv.Call("Arith.Multiply", args, &reply)

			time.Sleep(time.Second)
			fmt.Printf("Arith: %d*%d=%d\n", args.A, args.B, reply)
		}
	}
}
