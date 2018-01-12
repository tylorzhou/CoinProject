package main

import (
	"os"
	"sort"
	"time"

	bittrex "github.com/go-bittrex"
)

// A data structure to hold a key/value pair.
type Pair struct {
	Key   string
	Value int
}

// A slice of Pairs that implements sort.Interface to sort by Value.
type PairList []Pair

func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p PairList) Len() int           { return len(p) }
func (p PairList) Less(i, j int) bool { return p[i].Value < p[j].Value }

// A function to turn a map into a PairList, then sort and return it.
func sortMapByValue(m map[string]int) PairList {
	p := make(PairList, len(m))
	i := 0
	for k, v := range m {
		p[i] = Pair{k, v}
		i++
	}
	sort.Sort(sort.Reverse(p))
	return p
}
func VerifyBalance(crtcy CurtCurrencyMem) bool {

	//to be remove
	//to be done
	return true

	bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
	balances, err := bittrex.GetBalances()

	verified := false
	if err != nil {
		Error.Printf("GetBalances err %s", err.Error())
		return verified
	}

	for _, vblce := range balances {
		if vblce.Currency == crtcy.Name {
			msg := "found in balances: " + vblce.Currency + "  Available: " + vblce.Available.String() + " \n"

			if vblce.Available.Equal(crtcy.Quantity) {
				msg += "the quantity is the same\n"
				verified = true
			} else {
				msg += "error: the quantity is not the same\n"
				Error.Println(msg)
			}
			WriteReport("./data/orderdebug.log", msg)
			break
		}
	}
	return verified
}

func WriteReport(filepath, rpt string) {
	f, err := os.OpenFile(filepath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		Error.Printf("Write report failed %s\n", err.Error())
		return
	}
	defer f.Close()

	if _, err := f.WriteString(rpt); err != nil {
		Error.Printf("WriteReport failed %s", err.Error())
		return
	}

	finfor, err := f.Stat()
	if err != nil {
		Error.Printf("Stat failed %s", err.Error())
		return
	}
	if finfor.Size() > 1024*1024*40 {
		f.Close()
		os.Rename(filepath, filepath+".back")
	}

}

func TimeStamp() string {
	return time.Now().Format(time.RFC3339) + " "
}
