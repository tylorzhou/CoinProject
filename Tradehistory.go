package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	bittrex "github.com/go-bittrex"
	"github.com/shopspring/decimal"
)

type Trade struct {
	OrderUuid int64           `json:"Id"`
	Timestamp time.Time       `json:"TimeStamp"`
	Quantity  decimal.Decimal `json:"Quantity"`
	Price     decimal.Decimal `json:"Price"`
	Total     decimal.Decimal `json:"Total"`
	FillType  string          `json:"FillType"`
	OrderType string          `json:"OrderType"`
}

func Tradehistory(market string, lastid int64) []Trade {
	bittrex := bittrex.NewWithCustomTimeout(config.ApiKey, config.ApiSecret, time.Millisecond*600)
	tried := 1
RETRY:
	marketHistory, err := bittrex.GetMarketHistory(market)

	if err != nil {
		if tried < 3 {
			tried++
			goto RETRY
		}
		fmt.Printf("tried %d market %s %v \n", tried, market, err)
		Error.Printf("GetMarketHistory error %s, %s\n", market, err.Error())
		return nil
	}

	if tried == 3 {
		fmt.Printf("market %s tried %d len %d\n", market, tried, len(marketHistory))
	}

	lmarket := len(marketHistory)
	if lmarket < 1 {
		Error.Printf("GetMarketHistory length error %s\n", market)
		return nil
	}

	i := sort.Search(len(marketHistory), func(i int) bool { return marketHistory[i].OrderUuid <= lastid })

	if lastid != 0 && i == 0 {
		return nil //no updated trade
	}

	if i < lmarket && marketHistory[i].OrderUuid == lastid {
		temp := make([]Trade, 0, 100)

		for idx := 0; idx < i; idx++ {
			temp = append(temp, Trade{
				OrderUuid: marketHistory[idx].OrderUuid,
				Timestamp: marketHistory[idx].Timestamp.Time,
				Quantity:  marketHistory[idx].Quantity,
				Price:     marketHistory[idx].Price,
				Total:     marketHistory[idx].Total,
				FillType:  marketHistory[idx].FillType,
				OrderType: marketHistory[idx].OrderType,
			})
		}
		return temp

	}

	temp := make([]Trade, 0, 100)
	for idx := 0; idx < lmarket; idx++ {
		temp = append(temp, Trade{
			OrderUuid: marketHistory[idx].OrderUuid,
			Timestamp: marketHistory[idx].Timestamp.Time,
			Quantity:  marketHistory[idx].Quantity,
			Price:     marketHistory[idx].Price,
			Total:     marketHistory[idx].Total,
			FillType:  marketHistory[idx].FillType,
			OrderType: marketHistory[idx].OrderType,
		})
	}
	return temp

	/* b, _ := json.Marshal(marketHistory)
	err = ioutil.WriteFile("./data/Tradehistory/Tradehistory.json", b, 0644)
	if err != nil {
		Error.Printf("CurtCurrency error %s\n", err.Error())
	} */
}

type secRec struct {
	lastid   int64
	newadd   int
	sumtrade []Trade
}

var secRecMap = make(map[string]secRec)
var secRecmux sync.Mutex

func GetTrades(markets []string) {

	defer func() {
		grouplock.Done()
	}()
	for _, market := range markets {

		secRecmux.Lock()
		id := secRecMap[market].lastid
		secRecmux.Unlock()

		trades := Tradehistory(market, id)

		if trades != nil {
			//fmt.Printf("%s market: %s len %d first %d end %d\n", time.Now().Format(time.RFC3339), market, len(trades), trades[0].OrderUuid, trades[len(trades)-1].OrderUuid)

			secRecmux.Lock()
			sumtrade := secRecMap[market].sumtrade
			sumtrade = append(trades, sumtrade...)

			secRecMap[market] = secRec{
				lastid:   sumtrade[0].OrderUuid,
				sumtrade: sumtrade,
				newadd:   len(trades),
			}
			secRecmux.Unlock()
		} else {
			secRecmux.Lock()
			sumtrade := secRecMap[market]
			sumtrade.newadd = 0

			secRecMap[market] = sumtrade
			secRecmux.Unlock()
		}
	}

	/*


		b, _ := json.MarshalIndent(trade1000, "", " ")
		err := ioutil.WriteFile("./data/Tradehistory/Tradehistory.json", b, 0644)
		if err != nil {
			Error.Printf("Get1000Trades error %s\n", err.Error())
		}

	*/

}

var bakmarkt = make([]string, 0, 300)
var grouplock sync.WaitGroup

func SecGetTrades() {
	markets := GetnewMarkets()
	if len(bakmarkt) == 0 && markets != nil {
		bakmarkt = markets
	}

	if markets == nil && bakmarkt != nil {
		markets = bakmarkt
	}

	l := len(markets)

	div := 80
	groupf := float64(l) / float64(div)

	group := 0
	tempgroup := int64(groupf)
	if groupf-float64(tempgroup) > 0 {
		group = int(tempgroup) + 1
	}

	for i := 0; i < group; i++ {
		if i != group-1 {
			grouplock.Add(1)
			go GetTrades(markets[i*div : (i+1)*div])
		} else {
			grouplock.Add(1)
			go GetTrades(markets[i*div:])
		}

	}

	grouplock.Wait()
	//time.Sleep(time.Second * 15)

}

func GetnewMarkets() (makets []string) {

	bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
	data, err := bittrex.GetMarketSummaries()
	if err != nil {
		Error.Printf("error get marketsummaries %v", err)
		return
	}

	for _, v := range data {
		if strings.Contains(v.MarketName, "BTC-") == false {
			continue
		}

		makets = append(makets, v.MarketName)
	}
	return
}
func WriteTradeHisotry(market, msg string) {
	WriteReport("./data/Tradehistory/allmarket/"+market+".txt", msg)
}

func TaskSec5() {
	defer func() {
		if err := recover(); err != nil {
			Error.Printf("exception: %v \n %s\n", err,
				debug.Stack())
		}
		os.Exit(1)
	}()

	Hyped100old := make(map[string]int)

	ignorfirst := true
	for {
		t := time.Now()
		SecGetTrades()

		d := time.Since(t)

		for k, v := range secRecMap {

			buycount := 0
			buytotal, _ := decimal.NewFromString("0.0")
			sellcount := 0
			selltotal, _ := decimal.NewFromString("0.0")

			for _, vt := range v.sumtrade {
				if vt.OrderType == "BUY" {
					buycount++
					buytotal = buytotal.Add(vt.Total)
				} else {
					sellcount++
					selltotal = selltotal.Add(vt.Total)
				}
			}

			if v.newadd > 0 {

				if "BTC-XLM" == k {
					if v.newadd > 0 {
						fmt.Printf("len %d %v\n", v.newadd, v.sumtrade[0])
						fmt.Printf("test secRecMap[BTC-XLM].lastid %d\n", secRecMap["BTC-XLM"].lastid)
					}
				}

				msg := fmt.Sprintf("duration %s market %s trade %d, price %s \n", d, k, v.newadd, v.sumtrade[0].Price)
				msg += fmt.Sprintf("%s buycount : %d(%f) buytotal %s sellcount : %d(%f) selltotal %s\n", TimeStamp(), buycount, float64(buycount)/float64(buycount+sellcount),
					buytotal.String(), sellcount, float64(sellcount)/float64(sellcount+buycount), selltotal.String())

				if Hyped100old[k] == 100 && v.newadd == 100 {
					msg := TimeStamp() + "market:" + k + " is hot\n"
					WriteReport("./data/Tradehistory/Hyped100market.log", msg)
				}
				Hyped100old[k] = v.newadd
				//fmt.Printf("%s", msg)
				go WriteTradeHisotry(k, msg)
			}

			if v.newadd == 100 && ignorfirst == false {
				msg := fmt.Sprintf("duration %s market %s trade %d, price %s \n", d, k, v.newadd, v.sumtrade[0].Price)
				msg += GetTicker(k)
				msg += fmt.Sprintf("%s buycount : %d(%f) buytotal %s sellcount : %d(%f) selltotal %s\n", TimeStamp(), buycount, float64(buycount)/float64(buycount+sellcount),
					buytotal.String(), sellcount, float64(sellcount)/float64(sellcount+buycount), selltotal.String())
				fmt.Printf("%s", msg)

				WriteReport("./data/Tradehistory/Trade100.log", msg)
			}

			secRecMap[k] = secRec{
				lastid:   v.lastid,
				newadd:   0,
				sumtrade: nil,
			}
		}

		ignorfirst = false
		//fmt.Printf("record history test all %s\n", d)
	}
}

func GetTicker(market string) string {
	bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
	data, err := bittrex.GetTicker(market)
	return fmt.Sprintf("err %v %v \n", err, data)
}
