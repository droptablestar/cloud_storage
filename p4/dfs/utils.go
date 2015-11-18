package dfs

import (
	"fmt"
	"log"
	"os"
	"time"
)

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
