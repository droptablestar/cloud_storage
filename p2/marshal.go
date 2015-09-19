// 
package main

import (
	"fmt"
	"encoding/json"
)


type Vid struct {
	Sid, Index int
}

type Node struct {
	Name 		string
	Nid  		uint64
	Vid		Vid
	Dirty		bool
	Kids 		map[string]Vid
	Data		[]uint8
}


type Message struct {
    Name string
    Body string
    Time int64
}


func main() {
	nodes := make([]*Node, 2)
	nodes[0] = &Node{Nid: 13, Vid: Vid{3,4}, Name: "foo"}
	nodes[1] = &Node{Nid: 15, Vid: Vid{5, 6}, Name: "bar"}
	j, err := json.MarshalIndent(nodes, "", "  ")
	fmt.Println(nodes,"\nmarshalled: ",string(j),err)

	var nds  []*Node
	y := json.Unmarshal(j, &nds)
	fmt.Println("got back: ", len(nds), cap(nds), nds[0].Name, nds, nds[0], y)

	x := make(map[string]int)
	x["one"] = 1
	x["two"] = 2
	delete(x, "two")
	delete(x, "three")
	fmt.Println(x)
}


