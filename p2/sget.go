//
package main

/*
 Two main files are ../fuse.go and ../fs/serve.go
*/

import (
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("USAGE: sget <db path> [<key>]")
	} else {
		db, err := leveldb.OpenFile(os.Args[1], nil)
		if err != nil {
			fmt.Printf("Problem opening %q\n", os.Args[1])
		} else {
			if len(os.Args) == 2 {
				iter := db.NewIterator(nil, nil)
				for iter.Next() {
					key := iter.Key()
					value := iter.Value()
					fmt.Printf("%30q: '%s'\n", key, string(value))
				}
				iter.Release()
			} else {
				if value, err := db.Get([]byte(os.Args[2]), nil); err == nil {
					fmt.Println(string(value))
				} else {
					fmt.Printf("ERROR: %v\n", err)
				}
			}
		}
	}
}
