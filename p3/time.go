//
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unsafe"
)

type Vid struct {
	Sid, Index int
}

type Node struct {
	Name  string
	Nid   uint64
	Vid   Vid
	Dirty bool
	Kids  map[string]Vid
	Data  []uint8
}

type A struct{}

func (a *A) try() {
	fmt.Printf("tried A\n")
}

type B struct {
	A
}

func (b *B) try() {
	fmt.Printf("tried B\n")
	(&b.A).try()
}

func peteTime(s string) (time.Time, bool) {
	timeFormats := []string{"2006-1-2 15:04:05", "2006-1-2 15:04", "2006-1-2", "1-2-2006 15:04:05",
		"1-2-2006 15:04", "1-6-2006", "2006/1/2 15:04:05", "2006/1/2 15:04", "2006/1/2",
		"1/2/2006 15:04:05", "1/2/2006 15:04", "1/2/2006"}
	loc, _ := time.LoadLocation("Local")

	for _, v := range timeFormats {
		if tm, terr := time.ParseInLocation(v, s, loc); terr == nil {
			return tm, false
		}
	}
	return time.Time{}, true
}

func main() {
	nodes := make([]*Node, 2)
	nodes[0] = &Node{Nid: 13, Vid: Vid{3, 4}, Name: "foo"}
	nodes[1] = &Node{Nid: 15, Vid: Vid{5, 6}, Name: "bar"}
	j, err := json.MarshalIndent(nodes, "", "  ")
	fmt.Println(nodes, "\nmarshalled: ", string(j), err)

	var nds []*Node
	y := json.Unmarshal(j, &nds)
	fmt.Println("got back: ", len(nds), cap(nds), nds[0].Name, nds, nds[0], y)

	x := make(map[string]int)
	x["one"] = 1
	x["two"] = 2
	delete(x, "two")
	delete(x, "three")
	fmt.Println(x)

	q := "nice@1 hour ago"
	if i := strings.Index(q, "@"); i >= 0 {
		fmt.Printf("time is %q, %t\n", q[i+1:], strings.HasSuffix(q, " ago"))
	}

	dur, e := time.ParseDuration("-53m")
	fmt.Printf("pd %v %v %v\n", dur, e, time.Now().Add(dur))

	tms := []string{"2014-03-25 02:01:03", "2014-03-25 02:01", "2014-3-25", "2014/3/25", "3/25/2014"}
	for _, v := range tms {
		t, err := peteTime(v)
		if err {
			fmt.Printf("tm %q: %v\n", v, err)
		} else {
			fmt.Printf("tm %q: %v\n", v, t)
		}
	}

	tm, _ := peteTime("9/18/2014 00:40")
	n := time.Now()
	dur = n.Sub(tm)
	fmt.Printf("diff %d\n", int(dur/time.Second))

	var xsl []int
	xsl = append(xsl, 1)
	xsl = append(xsl, 1)
	fmt.Println(xsl)

	//	var xmp map[int]string
	//	xmp[1] = "nice"
	//	fmt.Println(xmp)

	var m map[string]*int
	m2 := make(map[string]*int)
	sl := []int{4, 5}
	sl2 := sl[:1]
	fmt.Printf("size maps: %d, %d, %d, %d\n",
		unsafe.Sizeof(m), unsafe.Sizeof(m2), unsafe.Sizeof(sl), unsafe.Sizeof(sl2))

	nx := new(Node)
	nx.Data = []uint8{3, 4, 5}
	fmt.Println(nx)

	ny := new(Node)
	*ny = *nx
	fmt.Println(ny)

	var b B
	b.try()
}
