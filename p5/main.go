package main

import (
	"bitbucket.org/jreeseue/818/p5/dfs"
	"crypto/aes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	. "github.com/mattn/go-getopt"
	"os"
	"strconv"
	"time"
)

var newfs string
var replicaString = "auto"
var first = true
var auth = ""

func main() {
	var c int

	for {
		if c = Getopt("ndta:f:m:r:"); c == EOF {
			break
		}

		switch c {
		case 'n':
			newfs = "NEWFS "
		case 'd':
			dfs.Debug = !dfs.Debug
		case 'f':
			dfs.FlusherPeriod, _ = strconv.Atoi(OptArg)
		case 't':
			dfs.Token = true
		case 'm':
			dfs.ModeConsistency = OptArg
		case 'r':
			replicaString = OptArg
		case 'a':
			first = false
			auth = OptArg
		default:
			println("usage: main.go [-n | -d | -f <flush dur> | -m <mode> | -r <rep string>]", c)
			os.Exit(1)
		}
	}
	fmt.Printf("\nStartup up with debug %v, flush period %v, %smode: %q, replicaStr  %q token: %t\n\n",
		dfs.Debug, dfs.FlusherPeriod, newfs, dfs.ModeConsistency, replicaString, dfs.Token)

	if first {
		dfs.AESkey = make([]byte, aes.BlockSize)
		_, _ = rand.Read(dfs.AESkey[:])
		fmt.Printf("session key: [%s]\n", dfs.AESkey)
	}
	dfs.LoadConfig(replicaString, "config.txt")
	for _, r := range dfs.Replicas {
		if r != dfs.Merep {
			fmt.Printf("client r: %q\n", r)
			dfs.Clients[r.Pid] = dfs.NewServerConn(r.Addr, r.Port)

			// authenticate with auth server
			if r.Name == auth {
				// request
				pub := dfs.ReadPublicKey(auth)
				nonce, _ := rand.Read(make([]byte, aes.BlockSize))
				var reply dfs.Response
				req := &dfs.Request{dfs.Merep.Name, 0, nonce}
				encrypted := dfs.RSAEncrypt(pub, dfs.Marshal(req))
				dfs.Clients[r.Pid].Call("Node.Authenticate",
					&encrypted, &reply)

				// reply
				private := dfs.ReadPrivateKey(dfs.Merep.Name)
				AESkey := dfs.RSADecrypt(private, reply.AESkey)
				json.Unmarshal(AESkey, &dfs.AESkey)
				test_str := dfs.AESDecrypt(dfs.AESkey, reply.Nonce)
				var test_int int
				json.Unmarshal(test_str, &test_int)
				fmt.Printf("reply [%s] %d %d\n", dfs.AESkey, nonce, test_int)
				if test_int != nonce {
					fmt.Printf("Incorrect nonce!\n")
					os.Exit(0)
				}
			}
		}
	}
	go func() {
		dfs.ServeInterface(dfs.Merep.Addr, dfs.Merep.Port, new(dfs.Node))
		fmt.Printf("\nDebug %v, mountpt: %q, %sstorePath %q, at %s:%d\n\n",
			dfs.Debug, dfs.Merep.Mount, newfs, dfs.Merep.Db,
			dfs.Merep.Addr, dfs.Merep.Port)
		dfs.Init(dfs.Merep.Mount, newfs != "", dfs.Merep.Db)
	}()
	for {
		time.Sleep(time.Second)
	}
}
