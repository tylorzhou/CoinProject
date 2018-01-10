package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	bittrex "github.com/go-bittrex"
	"github.com/shopspring/decimal"
)

type Ticker struct {
	Bid        decimal.Decimal `json:"Bid"`
	Ask        decimal.Decimal `json:"Ask"`
	Last       decimal.Decimal `json:"Last"`
	Volume     decimal.Decimal `json:"Volume"`
	BaseVolume decimal.Decimal `json:"BaseVolume"`
}

var coinB []string

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

//GetMarketB get all market price
func GetMarketB() *map[string]Ticker {

	bittrex := bittrex.New(config.ApiKey, config.ApiSecret)

	NewMktB := make(map[string]Ticker)
	// Get markets
	for _, v := range coinB {
		//t1 := time.Now()
		data, err := bittrex.GetMarketSummary(v)
		if err != nil {
			Error.Printf("error GetTicker: %s, %s\n\n", v, err.Error())
			continue
		}
		if len(data) > 0 {
			var t Ticker
			ask := data[0].Ask.String()
			bid := data[0].Bid.String()
			last := data[0].Last.String()

			t.Ask, _ = decimal.NewFromString(ask)
			t.Bid, _ = decimal.NewFromString(bid)
			t.Last, _ = decimal.NewFromString(last)
			t.BaseVolume = data[0].BaseVolume
			t.Volume = data[0].Volume
			NewMktB[v] = t
		}
		//fmt.Printf("get %s, take %s\n", v, time.Since(t1))

	}

	return &NewMktB
}

type ResOrder struct {
	Err      error
	Name     string
	V        decimal.Decimal
	Price    decimal.Decimal
	Quantity decimal.Decimal
}

var limt = make(chan struct{}, 10)

//GetOrderBook somehow concurrent get order cannot work for this lib
func GetOrderBook(cpair string, ch chan<- *ResOrder) {

	// Bittrex client
	limt <- struct{}{}
	defer func() {
		<-limt
	}()

	bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
	data, err := bittrex.GetOrderBookBuySell(cpair, "buy")

	/* 	//b, _ := json.MarshalIndent(data, "", " ")
	   	ioutil.WriteFile("./data/buyoder1.json", b, 0644) */
	rel, _ := decimal.NewFromString("0.0")
	for i := range data {

		q := data[i].Quantity
		r := data[i].Rate
		rel = rel.Add(q.Mul(r))
	}

	var rt *ResOrder

	if err != nil {
		rt = &ResOrder{
			Err:  err,
			Name: cpair,
		}
	} else {
		rt = &ResOrder{
			Err:   err,
			Name:  cpair,
			V:     rel,
			Price: data[0].Rate,
		}
	}

	ch <- rt
	wg.Done()

	return
	//fmt.Printf("try first size = %d\n, toal V = %s", len(data), rel.String())
}

var wg sync.WaitGroup

//GetAllOrderTask concurrent order cannot work here.
func GetAllOrderTask() {

	var ch chan *ResOrder
	orders := make(map[string][]string)

	for {

		for _, v := range coinB {
			wg.Add(1)
			go GetOrderBook(v, ch)
		}

		go func() {
			wg.Wait()
			close(ch)
		}()

		for rt := range ch {
			if rt.Err != nil {
				Error.Printf("GetAllOrderTask, %s %s\n", rt.Name, rt.Err.Error())
				continue
			}

			str := fmt.Sprintf("%s price %s, V %s\n", time.Now().Format(time.RFC3339), rt.Price.String(), rt.V.String())
			s := orders[rt.Name]
			orders[rt.Name] = append(s, str)
		}

		if len(orders["BTC-LTC"]) > 1000 {
			for _, v := range coinB {
				wg.Add(1)
				go OutputOrders(v, orders[v])
			}
			wg.Wait()
		}
	}

}

func OutputOrders(cpair string, out []string) {
	/*
		f, err := os.OpenFile("./data/"+cpair+".log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			Error.Printf("OutputOrders %s", err.Error())
			return
		}

		defer f.Close()*/
	defer wg.Done()

	/*
		for _, v := range out {
			if _, err = f.WriteString(v); err != nil {
				Error.Printf("OutputOrders err %s\n", err.Error())
			}

		}*/

}

//GetBCPrice get bitcoin price by usdt
func GetBCPrice() string {
	bittrex := bittrex.New(config.ApiKey, config.ApiSecret)

	data, err := bittrex.GetTicker("USDT-BTC")
	if err != nil {
		Error.Printf("error GetTicker: %s\n\n", err.Error())
		return ""
	}

	return data.Last.String()

}

func initMarketB() {
	file, err := ioutil.ReadFile("./config/marketB.json")
	if err != nil {
		Error.Printf("error read json marketB: %s\n", err.Error())
		return
	}

	json.Unmarshal(file, &coinB)
}

type B2PDis struct{ name, price, pricediff, percentage string }

/* func TaskB2P() {
	p := GetMarketP()
	b := GetMarketB()
	bprice := GetBCPrice()
	bP, _ := decimal.NewFromString(bprice)

	top10 := make([]B2PDis, 10, 10)
	for k, v := range mapB2P {

		blast := (*b)[k].Last
		plast := (*p)[v].Last
		sub := blast.Sub(plast)
		vmul := sub.Mul(bP)
		vdiv := sub.Div(plast)
		History.Printf("(bitcoin %s) B -> P (%s) %s- (%s) %s = %s (%s$) \n", bP.String(), k, blast.String(), v, plast.String(), sub.String(), vmul.String())

		vperc := vdiv.String()
		if vperc[0] == '-' {
			vperc = vperc[1:]
		}

		var i int

		dis := B2PDis{
			name:       k,
			price:      blast.Mul(bP).String(),
			pricediff:  vmul.String(),
			percentage: vperc,
		}

		for i = 0; i < len(top10); i++ {
			if vperc >= top10[i].percentage {
				copy(top10[i+1:], top10[i:])
				top10[i] = dis
				break
			}
		}

		//a := b[k].Last.sub(p[v].Last)
	}

	History.Printf("*******************************************\n")
	for i := range top10 {
		History.Printf("name %s, price %s, pricediff %s, perctg %s\n", top10[i].name, top10[i].price, top10[i].pricediff, top10[i].percentage)
	}

} */

/*
func mainTask() {
	var pS []map[string]Ticker
	MinCount := 0
	for {

		pS = append(pS, *GetMarketB())
		if len(pS) > 60 {
			pS = pS[1:]
		}
		MinCount++
		l := len(pS)

		// hour report
		MaxChangeHour := 0.0
		MaxKHour := ""
		VolumeChange := 0.0
		VolumeOrigin := 0.0

		var MaxVolumeChange [topVolume]float64
		var MaxVHour [topVolume]string
		var PriceChange [topVolume]float64
		var MaxVolumeOrigin [topVolume]float64
		if l == 60 && MinCount >= 60 {
			MinCount = 0

			for k := range pS[0] {
				v1, _ := strconv.ParseFloat(pS[l-1][k].PercentChange, 64)
				v2, _ := strconv.ParseFloat(pS[0][k].PercentChange, 64)

				volume1, _ := strconv.ParseFloat(pS[l-1][k].BaseVolume, 64)
				volume2, _ := strconv.ParseFloat(pS[0][k].BaseVolume, 64)

				if (v1 - v2) > MaxChangeHour {

					VolumeChange = volume1 - volume2
					VolumeOrigin = volume2
					MaxKHour = k
					MaxChangeHour = v1 - v2
				}

				for i := 0; i < topVolume; i++ {
					if (volume1 - volume2) >= MaxVolumeChange[i] {
						if i < topVolume-1 {
							copy(MaxVolumeChange[i+1:], MaxVolumeChange[i:])
							copy(MaxVHour[i+1:], MaxVHour[i:])
							copy(PriceChange[i+1:], PriceChange[i:])
							copy(MaxVolumeOrigin[i+1:], MaxVolumeOrigin[i:])
						}
						MaxVolumeChange[i] = volume1 - volume2
						MaxVolumeOrigin[i] = volume2
						MaxVHour[i] = k
						PriceChange[i] = v1 - v2
						break
					}

				}

			}
			bitcoin1, _ := strconv.ParseFloat(bS[l-1].PercentChange, 64)
			bitcoin2, _ := strconv.ParseFloat(bS[0].PercentChange, 64)
			bitcoinVolume1, _ := strconv.ParseFloat(bS[l-1].BaseVolume, 64)
			bitcoinVolume2, _ := strconv.ParseFloat(bS[0].BaseVolume, 64)

			bitCoinPrice, _ := strconv.ParseFloat(bS[0].HighestBid, 64)
			emailText := "**Hour report**\n"
			emailText += fmt.Sprintf("------------BitCoin chart--------------\n")
			emailText += fmt.Sprintf("Price     : %s => %s (%%%f) \n", bS[0].PercentChange, bS[l-1].PercentChange, (bitcoin1-bitcoin2)*100)
			emailText += fmt.Sprintf("BaseVolume: %s => %s (VolumeChange %s$ %d Btc, %%%f) \n", DollorDisplay(strconv.Itoa(int(bitcoinVolume2))), DollorDisplay(strconv.Itoa(int(bitcoinVolume1))),
				DollorDisplay(strconv.Itoa(int(bitcoinVolume1-bitcoinVolume2))), int(bitcoinVolume1-bitcoinVolume2)/int(bitCoinPrice), (bitcoinVolume1-bitcoinVolume2)/bitcoinVolume2*100)
			emailText += fmt.Sprintf("											 \n")

			Topic := ""
			emailText += fmt.Sprintf("------------Max volume change-------------\n")
			for i := 0; i < topVolume; i++ {
				emailText += fmt.Sprintf("                  %s                   \n", MaxVHour[i])
				emailText += fmt.Sprintf("Price     : %s => %s (%%%f) \n", pS[0][MaxVHour[i]].PercentChange, pS[l-1][MaxVHour[i]].PercentChange, PriceChange[i]*100)
				emailText += fmt.Sprintf("BaseVolume: %s => %s (VolumeChange %f, %%%f) \n", pS[0][MaxVHour[i]].BaseVolume, pS[l-1][MaxVHour[i]].BaseVolume, MaxVolumeChange[i],
					MaxVolumeChange[i]/MaxVolumeOrigin[i]*100)
				emailText += fmt.Sprintf("\n")
				if i == topVolume-1 {
					Topic += (MaxVHour[i])

				} else {
					Topic += (MaxVHour[i] + ",")
				}
			}

			Topic = strings.Replace(Topic, "BTC_", "", -1)
			sendemail(emailText, "Hour report:"+Topic)
			History.Print(emailText)
		}

		//check volume change
		if l == 60 && MinCount != 0 && MinCount%20 == 0 {

			for k := range pS[0] {
				v1, _ := strconv.ParseFloat(pS[l-1][k].PercentChange, 64)
				v2, _ := strconv.ParseFloat(pS[l-20][k].PercentChange, 64)

				volume1, _ := strconv.ParseFloat(pS[l-1][k].BaseVolume, 64)
				volume2, _ := strconv.ParseFloat(pS[l-20][k].BaseVolume, 64)

				if (volume1 - volume2) > 800 {

					VolumeChange = volume1 - volume2
					VolumeOrigin = volume2
					MaxKHour = k
					MaxChangeHour = v1 - v2

					emailText := "**Volume increase 800**\n"
					emailText += fmt.Sprintf("-----Volume increase 800----(%d Minutes report)---\n", 20)
					emailText += fmt.Sprintf(" 					%s  				\n", MaxKHour)
					emailText += fmt.Sprintf("Price     : %s => %s (%%%f) \n", pS[l-20][MaxKHour].PercentChange, pS[l-1][MaxKHour].PercentChange, MaxChangeHour*100)
					emailText += fmt.Sprintf("BaseVolume: %s => %s (VolumeChange %f, %%%f) \n", pS[l-20][MaxKHour].BaseVolume, pS[l-1][MaxKHour].BaseVolume, VolumeChange, VolumeChange/VolumeOrigin*100)
					emailText += fmt.Sprintf("											 \n")
					sendemail(emailText, "increase 800:"+MaxKHour)
					History.Print(emailText)
				}

			}
		}

		//check volume change
		if l == 60 && MinCount != 0 && MinCount%15 == 0 {
			for k := range pS[0] {
				v1, _ := strconv.ParseFloat(pS[l-1][k].PercentChange, 64)
				v2, _ := strconv.ParseFloat(pS[l-15][k].PercentChange, 64)

				volume1, _ := strconv.ParseFloat(pS[l-1][k].BaseVolume, 64)
				volume2, _ := strconv.ParseFloat(pS[l-15][k].BaseVolume, 64)

				if (volume1 - volume2) > 300 {

					VolumeChange = volume1 - volume2
					VolumeOrigin = volume2
					MaxKHour = k
					MaxChangeHour = v1 - v2

					emailText := "**Volume increase 300**\n"
					emailText += fmt.Sprintf("-----Volume increase 300----(%d Minutes report)---\n", 15)
					emailText += fmt.Sprintf(" 					%s  				\n", MaxKHour)
					emailText += fmt.Sprintf("Price     : %s => %s (%%%f) \n", pS[l-15][MaxKHour].PercentChange, pS[l-1][MaxKHour].PercentChange, MaxChangeHour*100)
					emailText += fmt.Sprintf("BaseVolume: %s => %s (VolumeChange %f, %%%f) \n", pS[l-15][MaxKHour].BaseVolume, pS[l-1][MaxKHour].BaseVolume, VolumeChange, VolumeChange/VolumeOrigin*100)
					emailText += fmt.Sprintf("											 \n")
					sendemail(emailText, "increase 300:"+MaxKHour)
					History.Print(emailText)
				}

			}

		}

		time.Sleep(1 * time.Minute)
	}
}
*/

//GetOneOrderBook order
func GetOneOrderBook(cpair string) *ResOrder {

	// Bittrex client

	bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
	data, err := bittrex.GetOrderBookBuySell(cpair, "buy")

	/* 	//b, _ := json.MarshalIndent(data, "", " ")
	ioutil.WriteFile("./data/buyoder1.json", b, 0644) */

	/* 	rel, _ := decimal.NewFromString("0.0")
	   	for i := range data {

	   		q := data[i].Quantity
	   		r := data[i].Rate
	   		rel = rel.Add(q.Mul(r))
	   	} */

	var rt *ResOrder

	if err != nil {
		rt = &ResOrder{
			Err:  err,
			Name: cpair,
		}
	} else {
		if len(data) == 0 {
			rt = &ResOrder{
				Err:  errors.New("len is 0"),
				Name: cpair,
			}
		} else {
			rt = &ResOrder{
				Err:      err,
				Name:     cpair,
				Price:    data[0].Rate,
				Quantity: data[0].Quantity,
			}
		}

	}

	return rt
	//fmt.Printf("try first size = %d\n, toal V = %s", len(data), rel.String())
}

type MValue struct {
	output []string
	mRes   []*ResOrder
}

//GetOneOrderTask test.
func GetOneOrderTask() {

	var rt *ResOrder
	orders := make(map[string]*MValue)

	for {

		for _, v := range coinB {

			rt = GetOneOrderBook(v)

			if rt.Err != nil {
				Error.Printf("GetAllOrderTask, %s %s\n", rt.Name, rt.Err.Error())
				continue
			}

			str := fmt.Sprintf("%s price %s, V %s\n", time.Now().Format(time.RFC3339), rt.Price.String(), rt.V.String())
			m, b := orders[rt.Name]
			if !b {
				orders[rt.Name] = &MValue{
					output: make([]string, 0),
					mRes:   make([]*ResOrder, 0),
				}
				m = orders[rt.Name]
			}
			m.output = append(m.output, str)
			m.mRes = append(m.mRes, rt)
			orders[rt.Name] = m

		}

		if len(orders["BTC-LTC"].output) > 55 {
			for _, v := range coinB {
				wg.Add(1)
				go OutputOrders(v, orders[v].output)
			}
			wg.Wait()

			Top10VP(orders)

			for _, v := range coinB {
				orders[v].mRes = orders[v].mRes[:0]
				orders[v].output = orders[v].output[:0]
			}
		}

	}

}

type Top10Val struct {
	name string
	pchg decimal.Decimal
	vchg decimal.Decimal
}

func Top10VP(orders map[string]*MValue) {

	zero, _ := decimal.NewFromString("0.0")
	top10 := make([]Top10Val, 10, 10)

	for _, v := range orders {

		l := len(v.mRes)
		if l == 0 {
			continue
		}

		vtmp := Top10Val{
			pchg: v.mRes[0].Price.Sub(v.mRes[l-1].Price),
			vchg: v.mRes[0].V.Sub(v.mRes[l-1].V),
			name: v.mRes[0].Name,
		}

		for i, v := range top10 {
			if vtmp.vchg.Sub(v.vchg).GreaterThan(zero) {
				copy(top10[i+1:], top10[i:])
				top10[i] = vtmp
				break
			}

		}

	}

	f, err := os.OpenFile("./data/Top10Report.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		Error.Printf("Top10Report file open error: %v", err)
		return
	}
	defer f.Close()

	f.WriteString("**********************************************************\n")
	f.WriteString(time.Now().Format(time.RFC3339) + "\n")
	eminfo := ""
	for _, v := range top10 {
		eminfo += v.name + "  vchg:" + v.vchg.String() + "   pchg:" + v.pchg.String() + "\n"
		f.WriteString(v.name + "  vchg:" + v.vchg.String() + "   pchg:" + v.pchg.String() + "\n")
	}

	sendemail(eminfo, "bittrex")
}

type MarketSummary struct {
	MarketName     string          `json:"MarketName"`
	High           decimal.Decimal `json:"High"`
	Low            decimal.Decimal `json:"Low"`
	Ask            decimal.Decimal `json:"Ask"`
	Bid            decimal.Decimal `json:"Bid"`
	OpenBuyOrders  int             `json:"OpenBuyOrders"`
	OpenSellOrders int             `json:"OpenSellOrders"`
	Volume         decimal.Decimal `json:"Volume"`
	Last           decimal.Decimal `json:"Last"`
	BaseVolume     decimal.Decimal `json:"BaseVolume"`
	PrevDay        decimal.Decimal `json:"PrevDay"`
	TimeStamp      string          `json:"TimeStamp"`
}

func GetSummary(sum map[string][]*MarketSummary) {

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

		vnew := &MarketSummary{
			MarketName:     v.MarketName,
			High:           v.High,
			Low:            v.Low,
			Ask:            v.Ask,
			Bid:            v.Bid,
			OpenBuyOrders:  v.OpenBuyOrders,
			OpenSellOrders: v.OpenSellOrders,
			Volume:         v.Volume,
			Last:           v.Last,
			BaseVolume:     v.BaseVolume,
			PrevDay:        v.PrevDay,
			TimeStamp:      v.TimeStamp,
		}

		sum[v.MarketName] = append(sum[v.MarketName], vnew)
		if len(sum[v.MarketName]) > 60 {
			sum[v.MarketName] = sum[v.MarketName][1:]
		}
	}
}

type SummaryReport struct {
	MarketName       string
	Vchg, Vfrom, Vto decimal.Decimal
	Pchg, Pfrom, Pto decimal.Decimal
}

func GetSummaryTask() {

	defer func() { //catch or finally
		if err := recover(); err != nil { //catch
			Error.Printf("exception: %v\n %s\n", err, debug.Stack())
			os.Exit(1)
		}
	}()

	b, err := ioutil.ReadFile("./config/CurtCurrency.json")
	if err != nil {
		Error.Printf(err.Error())
		return
	}

	var crtcyfile CurtCurrency
	err = json.Unmarshal(b, &crtcyfile)
	if err != nil {
		Error.Printf(err.Error())
		return
	}

	tempquatity, _ := decimal.NewFromString(crtcyfile.Quantity)
	crtcy := CurtCurrencyMem{
		Name:     crtcyfile.Name,
		Quantity: tempquatity,
	}

	b, err = ioutil.ReadFile("./config/MinCurtCurrency.json")
	err = json.Unmarshal(b, &crtcyfile)
	if err != nil {
		Error.Printf(err.Error())
		return
	}

	tempquatity, _ = decimal.NewFromString(crtcyfile.Quantity)
	mincrtcy := CurtCurrencyMem{
		Name:     crtcyfile.Name,
		Quantity: tempquatity,
	}

	b, err = ioutil.ReadFile("./config/10MinCurtCurrency.json")
	err = json.Unmarshal(b, &crtcyfile)
	if err != nil {
		Error.Printf(err.Error())
		return
	}

	tempquatity, _ = decimal.NewFromString(crtcyfile.Quantity)
	Tenmincrtcy := CurtCurrencyMem{
		Name:     crtcyfile.Name,
		Quantity: tempquatity,
	}

	b, err = ioutil.ReadFile("./config/HourCurtCurrency.json")
	err = json.Unmarshal(b, &crtcyfile)
	if err != nil {
		Error.Printf(err.Error())
		return
	}

	tempquatity, _ = decimal.NewFromString(crtcyfile.Quantity)
	hourcrtcy := CurtCurrencyMem{
		Name:     crtcyfile.Name,
		Quantity: tempquatity,
	}

	b, err = ioutil.ReadFile("./config/15MinCurtCurrency.json")
	err = json.Unmarshal(b, &crtcyfile)
	if err != nil {
		Error.Printf(err.Error())
		return
	}

	tempquatity, _ = decimal.NewFromString(crtcyfile.Quantity)
	Fifteenmincrtcy := CurtCurrencyMem{
		Name:     crtcyfile.Name,
		Quantity: tempquatity,
	}

	if VerifyBalance(crtcy) == false {
		Error.Printf("verify balance failed\n")
		return
	}

	sliceTop := make([][]SummaryReport, 0, 12)
	smap := make(map[string][]*MarketSummary)
	sdifmap := make(map[string][]MINDIF)
	hourSlicetop := make([][]SummaryReport, 0, 2)
	imincnt := 0
	for {
		GetSummary(smap)
		time.Sleep(time.Second * 60)
		imincnt++
		if imincnt == 60 {

			hourSlicetop = append(hourSlicetop, SummaryHourReport(smap))
			if len(hourSlicetop) > 2 {
				hourSlicetop = hourSlicetop[1:]
			}

			if len(hourSlicetop) == 2 {
				HourTask(&hourcrtcy, hourSlicetop)
			}

			imincnt = 0
		}

		if imincnt != 0 && imincnt%10 == 0 {
			TenMinsTask(&Tenmincrtcy, smap)
		}

		if imincnt != 0 && imincnt%15 == 0 {
			FifteenMinsTask(&Fifteenmincrtcy, smap)

		}

		if imincnt != 0 && imincnt%20 == 0 {
			Summary20MinReport(smap)
		}

		if imincnt != 0 && imincnt%5 == 0 {
			sliceTop = append(sliceTop, Summary5MinTop10Report(smap))
			if len(sliceTop) > 12 { //only keep 1 hour top record now
				sliceTop = sliceTop[1:]
			}

			if len(sliceTop) == 12 {
				scoremap := GetScores(sliceTop)
				buy, score := Readytobuy(*scoremap, smap)
				if buy != "" && crtcy.Name == "BTC" {
					var err error
					crtcy, err = PlaceBuyOrder(buy, crtcy)
					if err != nil {
						continue
					}
				}

				if crtcy.Name != "BTC" {
					crtscore := (*scoremap)["BTC-"+crtcy.Name]
					bsell := ReadyToSell(buy, score, crtcy, crtscore)
					if bsell {
						var err error
						crtcy, err = PlaceSellOrder(crtcy) // next turn to buy high coin should be fine
						if err != nil {
							continue
						}
					}
				}
			}
		}

		//minute task
		MinReport(smap, sdifmap)

		//MinTasktobuy(sdifmap, &mincrtcy)
		MinTasktobuy1try(sdifmap, &mincrtcy)

		if Triggled.coin != "" && mincrtcy.Name != "BTC" {
			MinTasktosell(smap, &mincrtcy)
		}

	}

}

type CurtCurrency struct {
	Name     string
	Quantity string
}

type CurtCurrencyMem struct {
	Name     string
	Quantity decimal.Decimal
}

func Readytobuy(scoremap map[string]int, smap map[string][]*MarketSummary) (string, int) {
	rpt := ""
	debug := ""
	buy := ""
	score := 0
	zero, _ := decimal.NewFromString("0.0")
	debug += "*******" + time.Now().Format(time.RFC3339) + "*******\n"
	scrsort := sortMapByValue(scoremap)
	for i := range scrsort {
		if scrsort[i].Value >= 60 && buy == "" {

			rpt += "********************" + time.Now().Format(time.RFC3339) + "*********************\n"

			Summary := smap[scrsort[i].Key]
			l := len(Summary)
			if l < 2 { // that is not right
				rpt += scrsort[i].Key + " slice lenth cannot be less than 2\n"
				WriteReport("./data/Readytobuy.log", rpt)
				continue
			}

			if Summary[0].Last.Sub(Summary[l-1].Last).GreaterThan(zero) == false {
				rpt += "ready to buy but price in down trend :" + scrsort[i].Key + "  score:" + strconv.Itoa(scrsort[i].Value) + " in 60\n"
				WriteReport("./data/Readytobuy.log", rpt)
				continue
			}

			rpt += "ready to buy, HOT:" + scrsort[i].Key + "  score:" + strconv.Itoa(scrsort[i].Value) + " in 60\n"
			WriteReport("./data/Readytobuy.log", rpt)
			buy = scrsort[i].Key
			score = scrsort[i].Value

		}

		debug += scrsort[i].Key + "  score:" + strconv.Itoa(scrsort[i].Value) + "\n"

	}
	WriteReport("./data/5minsscoremap.log", debug)

	return buy, score
}

func ReadyToSell(hot string, score int, crtcy CurtCurrencyMem, crtscore int) bool {

	rpt := "********************" + time.Now().Format(time.RFC3339) + "*********************\n"
	rpt += "hot coin :" + hot + " score: " + strconv.Itoa(score) + "current: " + crtcy.Name + " crtscore:" + strconv.Itoa(crtscore) + "\n"

	if hot == "BTC-"+crtcy.Name {
		rpt += "coin is safe on high valume\n"
		WriteReport("./data/orderdebug.log", rpt)
		return false
	}

	if score == 60 || crtscore < 20 {
		//force to sell
		WriteReport("./data/orderdebug.log", "force to sell, score:"+strconv.Itoa(score)+" crtscore:"+strconv.Itoa(crtscore)+"\n")
		return true
	}

	WriteReport("./data/orderdebug.log", rpt)
	return false
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

func PlaceBuyOrder(buy string, crtcy CurtCurrencyMem) (CurtCurrencyMem, error) {
	zero, _ := decimal.NewFromString("0.0")
	costRate, _ := decimal.NewFromString("0.0025")
	rpt := "********************" + time.Now().Format(time.RFC3339) + "*********************\n"
	rpt += "begin to place buy order for :" + buy + " \n"
	rpt += "current  currency:" + crtcy.Name + "  Quantity:" + crtcy.Quantity.String() + " \n"
	WriteReport("./data/orderdebug.log", rpt)

	totBought := 0.0

	verifiedb := VerifyBalance(crtcy)
	if verifiedb == false {
		return crtcy, errors.New("verify balance failed")
	}

	bOrderPlaced := false
	for {

		if crtcy.Quantity.LessThan(zero) || crtcy.Quantity.Equal(zero) {
			msg := fmt.Sprintf("buy %s successfully, multiple transition \n", "BTC-"+buy)
			WriteReport("./data/orderdebug.log", msg)
			bOrderPlaced = true
			break
		}
		bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
		data, err := bittrex.GetOrderBookBuySell(buy, "sell")

		if err != nil {
			Error.Printf("GetAllOrderTask, %s %s\n", buy, err.Error())
			continue
		}
		//process order here, current currency have to be BTC

		LowestQ := data[0].Quantity
		LowestP := data[0].Rate

		total := crtcy.Quantity
		cost := crtcy.Quantity.Mul(costRate)

		left := total.Sub(cost)
		Qty := left.Div(LowestP)

		if LowestQ.GreaterThan(Qty) {
			q, _ := Qty.Float64()
			p, _ := LowestP.Float64()
			uuid := "1234"
			var err error
			//to be done
			//uuid, err := bittrex.BuyLimit(buy, q, p)

			if err != nil {
				Error.Printf("buy %s failed, LowestQ %s LowestP %s q:%e p:%e\n", buy, LowestQ.String(),
					LowestP.String(), q, p)
				continue
			}

			msg := fmt.Sprintf(time.Now().Format(time.RFC3339)+" buy %s successfully, LowestQ %s LowestP %s q:%e p:%e uuid: %s\n", buy, LowestQ.String(),
				LowestP.String(), q, p, uuid)
			WriteReport("./data/orderdebug.log", msg)
			totBought += q
			bOrderPlaced = true
			break
		} else {

			q, _ := LowestQ.Float64()
			p, _ := LowestP.Float64()

			uuid := "1234"
			var err error
			//to be done
			//uuid, err := bittrex.BuyLimit(buy, q, p)

			if err != nil {
				Error.Printf("buy %s failed, LowestQ %s LowestP %s q:%e p:%e\n", buy, LowestQ.String(),
					LowestP.String(), q, p)
				continue
			}

			crtcy.Quantity = crtcy.Quantity.Sub(LowestQ.Mul(LowestP))
			msg := fmt.Sprintf(time.Now().Format(time.RFC3339)+" buy %s successfully, LowestQ %s LowestP %s q:%e p:%e uuid: %s, left %s\n", buy, LowestQ.String(),
				LowestP.String(), q, p, uuid, crtcy.Quantity.String())
			WriteReport("./data/orderdebug.log", msg)
			totBought += q

		}
	}

	if bOrderPlaced == true {
		//LABEL:
		//to be done
		dtotBought := decimal.NewFromFloat(totBought)

		curent := CurtCurrencyMem{
			Name:     buy[4:],
			Quantity: dtotBought,
		}

		data := CurtCurrency{
			Name:     buy[4:],
			Quantity: dtotBought.String(),
		}

		/*bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
		balance, err := bittrex.GetBalance(buy)
		if err != nil {
			Error.Printf("GetBalance %s %s\n", buy, err.Error())
			goto LABEL
		}

		curent := CurtCurrencyMem{
			Name:     buy[4:],
			Quantity: balance.Available,
		}

		data := CurtCurrency{
			Name:     buy[4:],
			Quantity: balance.Available.String(),
		}*/

		msg := fmt.Sprintf("dtotBought %s \n", dtotBought.String())
		WriteReport("./data/orderdebug.log", msg)

		b, _ := json.Marshal(data)
		err := ioutil.WriteFile("./config/CurtCurrency.json", b, 0644)
		if err != nil {
			Error.Printf("CurtCurrency error %s\n", err.Error())
		}
		return curent, nil

	}

	return crtcy, errors.New("buy oder failed")

}

func PlaceSellOrder(crtcy CurtCurrencyMem) (CurtCurrencyMem, error) {

	rpt := "********************" + time.Now().Format(time.RFC3339) + "*********************\n"
	rpt += "begin to place sell order for :" + crtcy.Name + " \n"
	rpt += "current  currency:" + crtcy.Name + "  Quantity:" + crtcy.Quantity.String() + " \n"
	WriteReport("./data/orderdebug.log", rpt)

	zero, _ := decimal.NewFromString("0.0")
	ftotSell := 0.0
	verifiedb := VerifyBalance(crtcy)
	if verifiedb == false {
		return crtcy, errors.New("verify balance failed")
	}

	OrderPlaced := false
	errcnt := 0
	for {

		bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
		data, err := bittrex.GetOrderBookBuySell("BTC-"+crtcy.Name, "buy")

		if err != nil {
			Error.Printf("PlaceSellOrder GetOrderBookBuySell %s\n", err.Error())
			errcnt++
			continue
		}

		if crtcy.Quantity.LessThan(zero) || crtcy.Quantity.Equal(zero) {
			msg := fmt.Sprintf("sell %s successfully, multiple transition \n ", "BTC-"+crtcy.Name)
			WriteReport("./data/orderdebug.log", msg)
			OrderPlaced = true
			break
		}

		highestQ := data[0].Quantity
		highestP := data[0].Rate

		if highestQ.GreaterThan(crtcy.Quantity) {
			q, _ := crtcy.Quantity.Float64()
			p, _ := highestP.Float64()
			uuid := "1234"
			var err error
			//to be done
			//uuid, err :=bittrex.SellLimit("BTC-"+crtcy.Name, q, p)

			if err != nil {
				errcnt++
				Error.Printf("sell %s failed, highestQ %s highestP %s q:%e p:%e\n", "BTC-"+crtcy.Name, highestQ.String(),
					highestP.String(), q, p)
				continue
			}
			msg := fmt.Sprintf("sell %s successfully, highestQ %s highestP %s q:%e p:%e uuid %s\n", "BTC-"+crtcy.Name, highestQ.String(),
				highestP.String(), q, p, uuid)
			WriteReport("./data/orderdebug.log", msg)
			OrderPlaced = true

			ftotSell += q * p * (1 - 0.0025)

			break
		} else {

			q, _ := highestQ.Float64()
			p, _ := highestP.Float64()
			uuid := "1234"
			var err error
			//to be done
			//uuid, err := bittrex.SellLimit("BTC-"+crtcy.Name, q, p)

			if err != nil {
				errcnt++
				Error.Printf("sell %s failed, highestQ %s highestP %s q:%e p:%e \n", "BTC-"+crtcy.Name, highestQ.String(),
					highestP.String(), q, p)
				continue
			}
			crtcy.Quantity = crtcy.Quantity.Sub(highestQ)
			msg := fmt.Sprintf("sell %s successfully, highestQ %s highestP %s q:%e p:%e uuid %s left %s\n", "BTC-"+crtcy.Name, highestQ.String(),
				highestP.String(), q, p, uuid, crtcy.Quantity.String())
			WriteReport("./data/orderdebug.log", msg)

			ftotSell += q * p * (1 - 0.0025)
		}
	}

	if OrderPlaced == true {

		//to be done
		dftotSell := decimal.NewFromFloat(ftotSell)
		BTC := CurtCurrencyMem{
			Name:     "BTC",
			Quantity: dftotSell,
		}

		data := CurtCurrency{
			Name:     "BTC",
			Quantity: dftotSell.String(),
		}

		/*
			LABEL:
			bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
			balance, err := bittrex.GetBalance("BTC")
			if err != nil {
				Error.Printf("GetBalance BTC %s", err.Error())
				goto LABEL
			}

			BTC := CurtCurrencyMem{
				Name:     "BTC",
				Quantity: balance.Available,
			}

			data := CurtCurrency{
				Name:     "BTC",
				Quantity: balance.Available.String(),
			}

		*/

		msg := fmt.Sprintf("ftotSell %f \n", ftotSell)
		WriteReport("./data/orderdebug.log", msg)

		b, _ := json.Marshal(data)
		err := ioutil.WriteFile("./config/CurtCurrency.json", b, 0644)
		if err != nil {
			Error.Printf("CurtCurrency error %s", err.Error())
		}
		return BTC, nil

	}

	return crtcy, errors.New("sell not finished")

}

func GetScores(sliceTop [][]SummaryReport) *map[string]int {

	scoremap := make(map[string]int)

	for _, v := range sliceTop {
		for i, val := range v {
			if i == 0 {
				scoremap[val.MarketName] += 5
			}

			if i == 1 {
				scoremap[val.MarketName] += 4
			}

			if i == 2 {
				scoremap[val.MarketName] += 3
			}

			if i == 3 {
				scoremap[val.MarketName] += 2
			}

			if i == 4 {
				scoremap[val.MarketName]++
			}

			if i >= 5 {
				scoremap[val.MarketName] += 0
			}

		}
	}
	return &scoremap
}

func SummaryHourReport(smap map[string][]*MarketSummary) []SummaryReport {

	Top10 := make([]SummaryReport, 10, 10)
	zero, _ := decimal.NewFromString("0.0")
	percent, _ := decimal.NewFromString("100.0")
	Topic := ""
	for k, v := range smap {
		l := len(v)
		if l < 1 {
			continue
		}
		vlatest := v[l-1]
		vfirst := v[0]

		rpt := SummaryReport{
			MarketName: k,
			Vchg:       vlatest.BaseVolume.Sub(vfirst.BaseVolume),
			Vfrom:      vfirst.BaseVolume,
			Vto:        vlatest.BaseVolume,
			Pchg:       vlatest.Last.Sub(vfirst.Last),
			Pfrom:      vfirst.Last,
			Pto:        vlatest.Last,
		}

		for i := range Top10 {
			if rpt.Vchg.Sub(Top10[i].Vchg).GreaterThan(zero) {
				copy(Top10[i+1:], Top10[i:])
				Top10[i] = rpt
				break
			}
		}
	}

	emailText := ""

	for i := range Top10 {
		emailText += fmt.Sprintf("                  %s                   \n", Top10[i].MarketName)
		emailText += fmt.Sprintf("Price     : %s => %s (%%%s) \n", Top10[i].Pfrom.String(), Top10[i].Pto.String(), (Top10[i].Pchg.Div(Top10[i].Pfrom)).Mul(percent).String())
		emailText += fmt.Sprintf("BaseVolume: %s => %s (VolumeChange %s, %%%s) \n", Top10[i].Vfrom.String(), Top10[i].Vto.String(), Top10[i].Vchg.String(),
			Top10[i].Vchg.Div(Top10[i].Vfrom).Mul(percent).String())
		emailText += fmt.Sprintf("\n")

		if i == 9 {
			Topic += Top10[i].MarketName
		} else {
			Topic += (Top10[i].MarketName + ",")
		}
	}

	Topic = strings.Replace(Topic, "BTC-", "", -1)

	sendemail(emailText, "Hour report(bittrex):"+Topic)
	History.Print("*************************************************\n")
	History.Print(emailText)
	// only returen top 5
	return Top10[0:5]
}

func Summary15MinReport(smap map[string][]*MarketSummary) map[string]decimal.Decimal {
	vlimit, _ := decimal.NewFromString("300.0")
	percent, _ := decimal.NewFromString("100.0")
	Zero, _ := decimal.NewFromString("0.0")
	intervel := 15
	hotmap := make(map[string]decimal.Decimal)
	for k, v := range smap {
		l := len(v)
		if l < intervel {
			continue
		}

		vchg := v[l-1].BaseVolume.Sub(v[l-intervel].BaseVolume)
		if vchg.GreaterThan(vlimit) {

			emailText := "**Volume increase 300, bittrex**\n"
			emailText += fmt.Sprintf("-----Volume increase 300----(%d Minutes report)---\n", intervel)
			emailText += fmt.Sprintf(" 					%s  				\n", k)
			emailText += fmt.Sprintf("Price     : %s => %s (%%%s) \n", v[l-intervel].Last.String(), v[l-1].Last.String(), v[l-1].Last.Sub(v[l-intervel].Last).Div(v[l-intervel].Last).Mul(percent).String())
			emailText += fmt.Sprintf("BaseVolume: %s => %s (VolumeChange %s, %%%s) \n", v[l-intervel].BaseVolume.String(), v[l-1].BaseVolume.String(), vchg.String(), vchg.Div(v[l-intervel].BaseVolume).Mul(percent).String())
			emailText += fmt.Sprintf("											 \n")
			sendemail(emailText, "increase 300:"+k)
			History.Print("*************************************************\n")
			History.Print(emailText)

			pricedif := v[l-1].Last.Sub(v[l-intervel].Last)
			if pricedif.GreaterThanOrEqual(Zero) { // make sure it is not down trend
				hotmap[k] = vchg
			}
		}
	}
	return hotmap
}

func TenMinsTask(Tenmincrtcy *CurtCurrencyMem, smap map[string][]*MarketSummary) {

	hotmap := Summary10MinReport(smap)

	if len(hotmap) > 0 && Tenmincrtcy.Name == "BTC" {
		topcoin := ""
		vtop, _ := decimal.NewFromString("0.0")
		for k, v := range hotmap {
			if v.GreaterThan(vtop) {
				vtop = v
				topcoin = k
			}
		}
		crtcy, _ := PlaceBuyOrderforTenmin(topcoin, *Tenmincrtcy)
		*Tenmincrtcy = crtcy
	}

	if Tenmincrtcy.Name != "BTC" {
		if whentoSellforTenMin(smap, Tenmincrtcy) == true {
			*Tenmincrtcy, _ = PlaceSellOrderforTenMin(*Tenmincrtcy)
		}

	}
}

func FifteenMinsTask(Fifteenmincrtcy *CurtCurrencyMem, smap map[string][]*MarketSummary) {

	hotmap := Summary15MinReport(smap)
	if len(hotmap) > 0 && Fifteenmincrtcy.Name == "BTC" {
		topcoin := ""
		vtop, _ := decimal.NewFromString("0.0")
		for k, v := range hotmap {
			if v.GreaterThan(vtop) {
				vtop = v
				topcoin = k
			}
		}
		crtcy, _ := PlaceBuyOrderforFifteenmin(topcoin, *Fifteenmincrtcy)
		*Fifteenmincrtcy = crtcy
	}

	if Fifteenmincrtcy.Name != "BTC" {
		if whentoSellforFifTeenMin(smap, Fifteenmincrtcy) == true {
			*Fifteenmincrtcy, _ = PlaceSellOrderforFifTeenMin(*Fifteenmincrtcy)
		}

	}
}

func Summary10MinReport(smap map[string][]*MarketSummary) map[string]decimal.Decimal {
	vlimit, _ := decimal.NewFromString("100.0")
	percent, _ := decimal.NewFromString("100.0")
	Zero, _ := decimal.NewFromString("0.0")
	intervel := 10

	hotmap := make(map[string]decimal.Decimal)
	for k, v := range smap {
		l := len(v)
		if l < intervel {
			continue
		}

		vchg := v[l-1].BaseVolume.Sub(v[l-intervel].BaseVolume)
		if vchg.GreaterThan(vlimit) {

			emailText := "**Volume increase 100, bittrex**\n"
			emailText += fmt.Sprintf("-----Volume increase 100----(%d Minutes report)---\n", intervel)
			emailText += fmt.Sprintf(" 					%s  				\n", k)
			emailText += fmt.Sprintf("Price     : %s => %s (%%%s) \n", v[l-intervel].Last.String(), v[l-1].Last.String(), v[l-1].Last.Sub(v[l-intervel].Last).Div(v[l-intervel].Last).Mul(percent).String())
			emailText += fmt.Sprintf("BaseVolume: %s => %s (VolumeChange %s, %%%s) \n", v[l-intervel].BaseVolume.String(), v[l-1].BaseVolume.String(), vchg.String(), vchg.Div(v[l-intervel].BaseVolume).Mul(percent).String())
			emailText += fmt.Sprintf("											 \n")
			sendemail(emailText, "increase 100:"+k)
			History.Print("*************************************************\n")
			History.Print(emailText)

			pricedif := v[l-1].Last.Sub(v[l-intervel].Last)
			if pricedif.GreaterThanOrEqual(Zero) { // make sure it is not down trend
				hotmap[k] = vchg
			}

		}
	}

	return hotmap
}

func Summary20MinReport(smap map[string][]*MarketSummary) {
	vlimit, _ := decimal.NewFromString("800.0")
	percent, _ := decimal.NewFromString("100.0")
	intervel := 20
	for k, v := range smap {
		l := len(v)
		if l < intervel {
			continue
		}

		vchg := v[l-1].BaseVolume.Sub(v[l-intervel].BaseVolume)
		if vchg.GreaterThan(vlimit) {

			emailText := "**Volume increase 800, bittrex**\n"
			emailText += fmt.Sprintf("-----Volume increase 800----(%d Minutes report)---\n", intervel)
			emailText += fmt.Sprintf(" 					%s  				\n", k)
			emailText += fmt.Sprintf("Price     : %s => %s (%%%s) \n", v[l-intervel].Last.String(), v[l-1].Last.String(), v[l-1].Last.Sub(v[l-intervel].Last).Div(v[l-intervel].Last).Mul(percent).String())
			emailText += fmt.Sprintf("BaseVolume: %s => %s (VolumeChange %s, %%%s) \n", v[l-intervel].BaseVolume.String(), v[l-1].BaseVolume.String(), vchg.String(), vchg.Div(v[l-intervel].BaseVolume).Mul(percent).String())
			emailText += fmt.Sprintf("											 \n")
			sendemail(emailText, "increase 800:"+k)
			History.Print("*************************************************")
			History.Print(emailText)
		}
	}
}

func Summary5MinTop10Report(smap map[string][]*MarketSummary) []SummaryReport {

	Top10 := make([]SummaryReport, 10, 10)
	zero, _ := decimal.NewFromString("0.0")
	percent, _ := decimal.NewFromString("100.0")
	Topic := ""
	for k, v := range smap {
		l := len(v)
		if l < 5 {
			continue
		}
		vlatest := v[l-1]
		vfirst := v[l-5]

		rpt := SummaryReport{
			MarketName: k,
			Vchg:       vlatest.BaseVolume.Sub(vfirst.BaseVolume),
			Vfrom:      vfirst.BaseVolume,
			Vto:        vlatest.BaseVolume,
			Pchg:       vlatest.Last.Sub(vfirst.Last),
			Pfrom:      vfirst.Last,
			Pto:        vlatest.Last,
		}

		for i := range Top10 {
			if rpt.Vchg.Sub(Top10[i].Vchg).GreaterThan(zero) {
				copy(Top10[i+1:], Top10[i:])
				Top10[i] = rpt
				break
			}
		}
	}

	emailText := ""

	for i := range Top10 {

		if Top10[i].Pfrom.Equal(zero) || Top10[i].Vfrom.Equal(zero) {
			continue
		}
		emailText += fmt.Sprintf("                  %s                   \n", Top10[i].MarketName)
		emailText += fmt.Sprintf("Price     : %s => %s (%%%s) \n", Top10[i].Pfrom.String(), Top10[i].Pto.String(), (Top10[i].Pchg.Div(Top10[i].Pfrom)).Mul(percent).String())
		emailText += fmt.Sprintf("BaseVolume: %s => %s (VolumeChange %s, %%%s) \n", Top10[i].Vfrom.String(), Top10[i].Vto.String(), Top10[i].Vchg.String(),
			Top10[i].Vchg.Div(Top10[i].Vfrom).Mul(percent).String())
		emailText += fmt.Sprintf("\n")

		if i == 9 {
			Topic += Top10[i].MarketName
		} else {
			Topic += (Top10[i].MarketName + ",")
		}
	}

	Topic = strings.Replace(Topic, "BTC-", "", -1)

	rpt := "********************" + time.Now().Format(time.RFC3339) + "*********************\n"
	rpt += emailText

	WriteReport("./data/Summary5MinTop10Report.log", rpt)
	return Top10

	//sendemail(emailText, "Hour report(bittrex):"+Topic)
	//History.Print("*************************************************\n")
	//History.Print(emailText)
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

func SummaryReportbySec(smap map[string][]*MarketSummary) {

	Top10 := make([]SummaryReport, 10, 10)
	zero, _ := decimal.NewFromString("0.0")
	percent, _ := decimal.NewFromString("100.0")
	Topic := ""
	for k, v := range smap {
		l := len(v)
		if l < 1 {
			continue
		}
		vlatest := v[l-1]
		vfirst := v[0]

		rpt := SummaryReport{
			MarketName: k,
			Vchg:       vlatest.BaseVolume.Sub(vfirst.BaseVolume),
			Vfrom:      vfirst.BaseVolume,
			Vto:        vlatest.BaseVolume,
			Pchg:       vlatest.Last.Sub(vfirst.Last),
			Pfrom:      vfirst.Last,
			Pto:        vlatest.Last,
		}

		for i := range Top10 {
			if rpt.Vchg.Sub(Top10[i].Vchg).GreaterThan(zero) {
				copy(Top10[i+1:], Top10[i:])
				Top10[i] = rpt
				break
			}
		}
	}

	emailText := ""

	for i := range Top10 {
		emailText += fmt.Sprintf("                  %s                   \n", Top10[i].MarketName)
		emailText += fmt.Sprintf("Price     : %s => %s (%%%s) \n", Top10[i].Pfrom.String(), Top10[i].Pto.String(), (Top10[i].Pchg.Div(Top10[i].Pfrom)).Mul(percent).String())
		emailText += fmt.Sprintf("BaseVolume: %s => %s (VolumeChange %s, %%%s) \n", Top10[i].Vfrom.String(), Top10[i].Vto.String(), Top10[i].Vchg.String(),
			Top10[i].Vchg.Div(Top10[i].Vfrom).Mul(percent).String())
		emailText += fmt.Sprintf("\n")

		if i == 9 {
			Topic += Top10[i].MarketName
		} else {
			Topic += (Top10[i].MarketName + ",")
		}
	}

	Topic = strings.Replace(Topic, "BTC-", "", -1)

	//sendemail(emailText, "Hour report(bittrex):"+Topic)
	//History.Print("*************************************************\n")
	//History.Print(emailText)
	rpt := "********************" + time.Now().Format(time.RFC3339) + "*********************\n"
	rpt += emailText
	WriteReport("./data/SummaryReportbySec.log", rpt)
}

func GetSummaryTaskBySec() {

	smap := make(map[string][]*MarketSummary)
	imincnt := 0
	for {
		GetSummary(smap)
		time.Sleep(time.Second)
		imincnt++
		if imincnt == 60 {
			SummaryReportbySec(smap)
			imincnt = 0
		}
	}

}

type MINDIF struct {
	Last, High, Lastdif decimal.Decimal
	BaseVolumedif       decimal.Decimal
	TimeStamp           string
}

func MinReport(smap map[string][]*MarketSummary, sdifmap map[string][]MINDIF) {
	Hundred, _ := decimal.NewFromString("100.0")
	now := time.Now()
	for k, v := range smap {
		l := len(v)
		if l < 2 {
			continue
		}

		sumary := v[l-1]

		b, err := json.MarshalIndent(sumary, "", " ")
		if err != nil {
			Error.Printf("marshalident %s\n", err.Error())
			continue
		}

		data := time.Now().Format(time.RFC3339) + "\n"
		data += string(b)
		WriteReport("./data/MinReport/"+k+".log", data)

		Highdif := v[l-1].High.Sub(v[l-2].High)
		Lowdif := v[l-1].Low.Sub(v[l-2].Low)
		Askdif := v[l-1].Ask.Sub(v[l-2].Ask)
		Biddif := v[l-1].Bid.Sub(v[l-2].Bid)
		OpenBuyOrdersdif := v[l-1].OpenBuyOrders - v[l-2].OpenBuyOrders
		OpenSellOrdersdif := v[l-1].OpenSellOrders - v[l-2].OpenSellOrders
		Volumedif := v[l-1].Volume.Sub(v[l-2].Volume)
		Lastdif := v[l-1].Last.Sub(v[l-2].Last)
		BaseVolumedif := v[l-1].BaseVolume.Sub(v[l-2].BaseVolume)

		difdata := MINDIF{
			Last:          v[l-1].Last,
			High:          v[l-1].High,
			Lastdif:       Lastdif,
			BaseVolumedif: BaseVolumedif,
			TimeStamp:     v[l-1].TimeStamp,
		}
		sdif := sdifmap[k]
		sdif = append(sdif, difdata)
		if len(sdif) > 60 {
			sdif = sdif[1:]
		}
		sdifmap[k] = sdif

		datadif := fmt.Sprintf("%s Highdif: %s Lowdif:%s Askdif:%s Biddif:%s OpenBuyOrdersdif:%d OpenSellOrdersdif:%d Volumedif:%s Lastdif:%s BaseVolumedif:%s \n", time.Now().Format(time.RFC3339), Highdif.String(),
			Lowdif.String(), Askdif.String(), Biddif.String(), OpenBuyOrdersdif, OpenSellOrdersdif, Volumedif.String(), Lastdif.String(), BaseVolumedif.String())
		datadif += fmt.Sprintf("price: %s ~ %s rate %%%s\n", v[l-2].Last.String(), v[l-1].Last.String(), v[l-1].Last.Sub(v[l-2].Last).Div(v[l-2].Last).Mul(Hundred))

		WriteReport("./data/MinReport/"+k+"dif.log", datadif)

		hundreddif, _ := decimal.NewFromString("100.0")
		if BaseVolumedif.GreaterThan(hundreddif) {
			WriteReport("./data/temp/minover100dif.log", datadif)
		}

	}

	last := time.Since(now)
	fmt.Printf("minreport eslaped %f seconds\n", last.Seconds())
}

type TOP3Times struct {
	coin  string
	times decimal.Decimal
}

type TriggledMin struct {
	coin                    string
	purchased, localnewhigh decimal.Decimal
	TriggledTime            time.Time
	newhigh                 bool
}

var Triggled TriggledMin

type lockedcoin struct {
	coin, lockreason string
	locktime         time.Time
}

var glockcmap = make(map[string]lockedcoin)

func MinTasktobuy(sdifmap map[string][]MINDIF, mincrtcy *CurtCurrencyMem) {
	//list LTC coin as example
	// need more time to collect sample
	ltc, b := sdifmap["BTC-LTC"]
	if b == false || len(ltc) < 60 {
		return
	}
	zero, _ := decimal.NewFromString("0.0")
	//one, _ := decimal.NewFromString("1.0")
	NEG, _ := decimal.NewFromString("-1")
	Rang, _ := decimal.NewFromString("30.0")
	avgmap := make(map[string]decimal.Decimal)
	avgprice := make(map[string]decimal.Decimal)

	for k, v := range sdifmap {
		sampledata := v[20:50] // got 30 min sample
		sum, _ := decimal.NewFromString("0.0")
		sumprice, _ := decimal.NewFromString("0.0")
		for _, v1 := range sampledata {
			if v1.BaseVolumedif.LessThan(zero) == true {
				v1.BaseVolumedif = v1.BaseVolumedif.Mul(NEG)
			}
			sum = sum.Add(v1.BaseVolumedif)
			sumprice = sumprice.Add(v1.Last)
		}
		avg := sum.Div(Rang)
		avgp := sumprice.Div(Rang)

		avgmap[k] = avg
		avgprice[k] = avgp
	}

	timesmap := make(map[string]decimal.Decimal)
	for k, v := range sdifmap {
		l := len(v)
		avg := avgmap[k]
		if avg.LessThanOrEqual(zero) {
			//WriteReport("./data/minorderdebug.log", "wrong div:"+k+"v:"+avg.String()+" \n")
			continue
		}

		/* 		if v[l-1].BaseVolumedif.LessThanOrEqual(one) {
			continue
		} */

		timesmap[k] = v[l-1].BaseVolumedif.Div(avgmap[k])
	}

	top3 := make([]TOP3Times, 3, 3)
	for k, v := range timesmap {
		for ktop, vtop := range top3 {

			if v.Sub(vtop.times).GreaterThan(zero) {
				copy(top3[ktop+1:], top3[ktop:])
				top3[ktop] = TOP3Times{
					coin:  k,
					times: v,
				}
				break
			}
		}

	}

	msg := ""
	for i := range top3 {
		l1 := len(sdifmap[top3[i].coin])
		if l1 < 1 {
			Error.Printf("top3 l1 less than 1, %v\n", top3)
			continue
		}
		msg += TimeStamp()
		msg += "coin: "
		msg += top3[i].coin
		msg += " times: "
		msg += top3[i].times.String()
		msg += " avgdif: " + avgmap[top3[i].coin].String()

		msg += " Last: " + sdifmap[top3[i].coin][l1-1].Last.String()
		msg += " High: " + sdifmap[top3[i].coin][l1-1].High.String()
		msg += " volumedif: " + sdifmap[top3[i].coin][l1-1].BaseVolumedif.String()
		msg += " \n"
	}

	WriteReport("./data/minorderdebug.log", msg)

	//only process top 1 now, the other 2 can process later
	TRIGGLE, _ := decimal.NewFromString("20.0")
	Times20 := top3[0].times.Sub(TRIGGLE).GreaterThan(zero)
	ONE, _ := decimal.NewFromString("1.0")

	btriggled := false

	l1 := len(sdifmap[top3[0].coin])
	if l1 > 1 {
		pricebar, _ := decimal.NewFromString("0.30")
		btriggled = Times20 && sdifmap[top3[0].coin][l1-1].BaseVolumedif.GreaterThan(ONE)
		dif := sdifmap[top3[0].coin][l1-1].Last.Sub(avgprice[top3[0].coin])

		if btriggled && dif.Div(avgprice[top3[0].coin]).GreaterThan(pricebar) {
			msg := "Price increase over 30%, maybe not to buy: " + top3[0].coin + "\n"
			msg += "try to lock coin: " + top3[0].coin + "\n"
			WriteReport("./data/minorderdebug.log", msg)
			btriggled = false
			lockcoin := lockedcoin{
				coin:       top3[0].coin,
				locktime:   time.Now(),
				lockreason: "Price increase over 30%",
			}
			glockcmap[lockcoin.coin] = lockcoin
		}

	}

	if btriggled == false {
		return
	}

	lockcoin, locked := glockcmap[top3[0].coin]
	if locked {
		if time.Since(lockcoin.locktime).Seconds() < 60 {
			msg := "this coin locked by reason:" + lockcoin.lockreason + "\n"
			WriteReport("./data/minorderdebug.log", msg)
			return
		}
	}

	if Triggled.coin != "" {
		msg = "Already had trigged coin: " + Triggled.coin + " "
		if top3[0].coin == "BTC-"+mincrtcy.Name {
			msg += "new triggled coin is the same. in the safe mode.\n"
		} else {
			msg += "new triggled coin is " + top3[0].coin + "\n"
		}

		WriteReport("./data/minorderdebug.log", msg)
		return
	}

	if mincrtcy.Name != "BTC" {
		msg = "Current coin: " + mincrtcy.Name + " is not BTC\n"
		WriteReport("./data/minorderdebug.log", msg)
		return
	}

	if btriggled {
		Triggled = TriggledMin{
			coin:         top3[0].coin,
			purchased:    zero, // price here is not right
			TriggledTime: time.Now(),
		}
		lockcoin := lockedcoin{
			coin:       top3[0].coin,
			locktime:   time.Now(),
			lockreason: "already bought",
		}
		msg := "locked coin " + lockcoin.coin + " when buying\n"
		WriteReport("./data/minorderdebug.log", msg)
		glockcmap[lockcoin.coin] = lockcoin
		crtcy, _ := PlaceBuyOrderformin(Triggled.coin, *mincrtcy)
		*mincrtcy = crtcy
	}

}

func PlaceBuyOrderformin(buy string, crtcy CurtCurrencyMem) (CurtCurrencyMem, error) {
	zero, _ := decimal.NewFromString("0.0")
	costRate, _ := decimal.NewFromString("0.0025")
	rpt := "********************" + time.Now().Format(time.RFC3339) + "*********************\n"
	rpt += "begin to place buy order for :" + buy + " \n"
	rpt += "current  currency:" + crtcy.Name + "  Quantity:" + crtcy.Quantity.String() + " \n"
	WriteReport("./data/minorderdebug.log", rpt)

	totBought := 0.0

	verifiedb := VerifyBalance(crtcy)
	if verifiedb == false {
		return crtcy, errors.New("verify balance failed")
	}

	bOrderPlaced := false
	for {

		if crtcy.Quantity.LessThan(zero) || crtcy.Quantity.Equal(zero) {
			msg := fmt.Sprintf("buy %s successfully, multiple transition \n", "BTC-"+buy)
			WriteReport("./data/minorderdebug.log", msg)
			bOrderPlaced = true
			break
		}
		bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
		data, err := bittrex.GetOrderBookBuySell(buy, "sell")

		if err != nil {
			Error.Printf("GetAllOrderTask, %s %s\n", buy, err.Error())
			continue
		}

		if len(data) == 0 {
			Error.Printf("GetAllOrderTask, %s\n", buy)
			continue
		}
		//process order here, current currency have to be BTC

		LowestQ := data[0].Quantity
		LowestP := data[0].Rate

		if LowestP.Sub(Triggled.purchased).GreaterThan(zero) {
			Triggled.purchased = LowestP
		}

		total := crtcy.Quantity
		cost := crtcy.Quantity.Mul(costRate)

		left := total.Sub(cost)
		Qty := left.Div(LowestP)

		if LowestQ.GreaterThan(Qty) {
			q, _ := Qty.Float64()
			p, _ := LowestP.Float64()
			uuid := "1234"
			var err error
			//to be done
			//uuid, err := bittrex.BuyLimit(buy, q, p)

			if err != nil {
				Error.Printf("buy %s failed, LowestQ %s LowestP %s q:%e p:%e\n", buy, LowestQ.String(),
					LowestP.String(), q, p)
				continue
			}

			msg := fmt.Sprintf(time.Now().Format(time.RFC3339)+" buy %s successfully, LowestQ %s LowestP %s q:%e p:%e uuid: %s\n", buy, LowestQ.String(),
				LowestP.String(), q, p, uuid)
			WriteReport("./data/minorderdebug.log", msg)
			totBought += q
			bOrderPlaced = true
			break
		} else {

			q, _ := LowestQ.Float64()
			p, _ := LowestP.Float64()

			uuid := "1234"
			var err error
			//to be done
			//uuid, err := bittrex.BuyLimit(buy, q, p)

			if err != nil {
				Error.Printf("buy %s failed, LowestQ %s LowestP %s q:%e p:%e\n", buy, LowestQ.String(),
					LowestP.String(), q, p)
				continue
			}

			crtcy.Quantity = crtcy.Quantity.Sub(LowestQ.Mul(LowestP))
			msg := fmt.Sprintf(time.Now().Format(time.RFC3339)+" buy %s partially, LowestQ %s LowestP %s q:%e p:%e uuid: %s, left %s\n", buy, LowestQ.String(),
				LowestP.String(), q, p, uuid, crtcy.Quantity.String())
			WriteReport("./data/minorderdebug.log", msg)
			totBought += q

		}
	}

	if bOrderPlaced == true {
		//LABEL:
		//to be done
		dtotBought := decimal.NewFromFloat(totBought)

		curent := CurtCurrencyMem{
			Name:     buy[4:],
			Quantity: dtotBought,
		}

		data := CurtCurrency{
			Name:     buy[4:],
			Quantity: dtotBought.String(),
		}

		/*bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
		balance, err := bittrex.GetBalance(buy)
		if err != nil {
			Error.Printf("GetBalance %s %s\n", buy, err.Error())
			goto LABEL
		}

		curent := CurtCurrencyMem{
			Name:     buy[4:],
			Quantity: balance.Available,
		}

		data := CurtCurrency{
			Name:     buy[4:],
			Quantity: balance.Available.String(),
		}*/

		msg := fmt.Sprintf("dtotBought %s \n", dtotBought.String())
		WriteReport("./data/minorderdebug.log", msg)

		b, _ := json.Marshal(data)
		err := ioutil.WriteFile("./config/MinCurtCurrency.json", b, 0644)
		if err != nil {
			Error.Printf("CurtCurrency error %s\n", err.Error())
		}
		return curent, nil

	}

	return crtcy, errors.New("buy oder failed")

}

func PlaceBuyOrderforTenmin(buy string, crtcy CurtCurrencyMem) (CurtCurrencyMem, error) {
	zero, _ := decimal.NewFromString("0.0")
	costRate, _ := decimal.NewFromString("0.0025")
	rpt := "********************" + time.Now().Format(time.RFC3339) + "*********************\n"
	rpt += "begin to place buy order for :" + buy + " \n"
	rpt += "current  currency:" + crtcy.Name + "  Quantity:" + crtcy.Quantity.String() + " \n"
	WriteReport("./data/10minorderdebug.log", rpt)

	totBought := 0.0

	verifiedb := VerifyBalance(crtcy)
	if verifiedb == false {
		return crtcy, errors.New("verify balance failed")
	}

	bOrderPlaced := false
	for {

		if crtcy.Quantity.LessThan(zero) || crtcy.Quantity.Equal(zero) {
			msg := fmt.Sprintf("buy %s successfully, multiple transition \n", "BTC-"+buy)
			WriteReport("./data/10minorderdebug.log", msg)
			bOrderPlaced = true
			break
		}
		bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
		data, err := bittrex.GetOrderBookBuySell(buy, "sell")

		if err != nil {
			Error.Printf("GetAllOrderTask, %s %s\n", buy, err.Error())
			continue
		}

		if len(data) == 0 {
			Error.Printf("GetAllOrderTask, %s\n", buy)
			continue
		}
		//process order here, current currency have to be BTC

		LowestQ := data[0].Quantity
		LowestP := data[0].Rate

		if LowestP.Sub(Triggled.purchased).GreaterThan(zero) {
			Triggled.purchased = LowestP
		}

		total := crtcy.Quantity
		cost := crtcy.Quantity.Mul(costRate)

		left := total.Sub(cost)
		Qty := left.Div(LowestP)

		if LowestQ.GreaterThan(Qty) {
			q, _ := Qty.Float64()
			p, _ := LowestP.Float64()
			uuid := "1234"
			var err error
			//to be done
			//uuid, err := bittrex.BuyLimit(buy, q, p)

			if err != nil {
				Error.Printf("buy %s failed, LowestQ %s LowestP %s q:%e p:%e\n", buy, LowestQ.String(),
					LowestP.String(), q, p)
				continue
			}

			msg := fmt.Sprintf(time.Now().Format(time.RFC3339)+" buy %s successfully, LowestQ %s LowestP %s q:%e p:%e uuid: %s\n", buy, LowestQ.String(),
				LowestP.String(), q, p, uuid)
			WriteReport("./data/10minorderdebug.log", msg)
			totBought += q
			bOrderPlaced = true
			break
		} else {

			q, _ := LowestQ.Float64()
			p, _ := LowestP.Float64()

			uuid := "1234"
			var err error
			//to be done
			//uuid, err := bittrex.BuyLimit(buy, q, p)

			if err != nil {
				Error.Printf("buy %s failed, LowestQ %s LowestP %s q:%e p:%e\n", buy, LowestQ.String(),
					LowestP.String(), q, p)
				continue
			}

			crtcy.Quantity = crtcy.Quantity.Sub(LowestQ.Mul(LowestP))
			msg := fmt.Sprintf(time.Now().Format(time.RFC3339)+" buy %s partially, LowestQ %s LowestP %s q:%e p:%e uuid: %s, left %s\n", buy, LowestQ.String(),
				LowestP.String(), q, p, uuid, crtcy.Quantity.String())
			WriteReport("./data/10minorderdebug.log", msg)
			totBought += q

		}
	}

	if bOrderPlaced == true {
		//LABEL:
		//to be done
		dtotBought := decimal.NewFromFloat(totBought)

		curent := CurtCurrencyMem{
			Name:     buy[4:],
			Quantity: dtotBought,
		}

		data := CurtCurrency{
			Name:     buy[4:],
			Quantity: dtotBought.String(),
		}

		/*bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
		balance, err := bittrex.GetBalance(buy)
		if err != nil {
			Error.Printf("GetBalance %s %s\n", buy, err.Error())
			goto LABEL
		}

		curent := CurtCurrencyMem{
			Name:     buy[4:],
			Quantity: balance.Available,
		}

		data := CurtCurrency{
			Name:     buy[4:],
			Quantity: balance.Available.String(),
		}*/

		msg := fmt.Sprintf("dtotBought %s \n", dtotBought.String())
		WriteReport("./data/10minorderdebug.log", msg)

		b, _ := json.Marshal(data)
		err := ioutil.WriteFile("./config/10MinCurtCurrency.json", b, 0644)
		if err != nil {
			Error.Printf("CurtCurrency error %s\n", err.Error())
		}
		return curent, nil

	}

	return crtcy, errors.New("buy oder failed")

}

func PlaceSellOrderforMin(crtcy CurtCurrencyMem) (CurtCurrencyMem, error) {

	rpt := "********************" + time.Now().Format(time.RFC3339) + "*********************\n"
	rpt += "begin to place sell order for :" + crtcy.Name + " \n"
	rpt += "current  currency:" + crtcy.Name + "  Quantity:" + crtcy.Quantity.String() + " \n"
	WriteReport("./data/minorderdebug.log", rpt)

	zero, _ := decimal.NewFromString("0.0")
	ftotSell := 0.0
	verifiedb := VerifyBalance(crtcy)
	if verifiedb == false {
		return crtcy, errors.New("verify balance failed")
	}

	OrderPlaced := false
	errcnt := 0
	for {

		bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
		data, err := bittrex.GetOrderBookBuySell("BTC-"+crtcy.Name, "buy")

		if err != nil {
			Error.Printf("PlaceSellOrder GetOrderBookBuySell %s\n", err.Error())
			errcnt++
			continue
		}

		if crtcy.Quantity.LessThan(zero) || crtcy.Quantity.Equal(zero) {
			msg := fmt.Sprintf("sell %s successfully, multiple transition \n ", "BTC-"+crtcy.Name)
			WriteReport("./data/minorderdebug.log", msg)
			OrderPlaced = true
			break
		}

		highestQ := data[0].Quantity
		highestP := data[0].Rate

		if highestQ.GreaterThan(crtcy.Quantity) {
			q, _ := crtcy.Quantity.Float64()
			p, _ := highestP.Float64()
			uuid := "1234"
			var err error
			//to be done
			//uuid, err :=bittrex.SellLimit("BTC-"+crtcy.Name, q, p)

			if err != nil {
				errcnt++
				Error.Printf("sell %s failed, highestQ %s highestP %s q:%e p:%e\n", "BTC-"+crtcy.Name, highestQ.String(),
					highestP.String(), q, p)
				continue
			}
			msg := fmt.Sprintf("sell %s successfully, highestQ %s highestP %s q:%e p:%e uuid %s\n", "BTC-"+crtcy.Name, highestQ.String(),
				highestP.String(), q, p, uuid)
			WriteReport("./data/minorderdebug.log", msg)
			OrderPlaced = true

			ftotSell += q * p * (1 - 0.0025)

			break
		} else {

			q, _ := highestQ.Float64()
			p, _ := highestP.Float64()
			uuid := "1234"
			var err error
			//to be done
			//uuid, err := bittrex.SellLimit("BTC-"+crtcy.Name, q, p)

			if err != nil {
				errcnt++
				Error.Printf("sell %s failed, highestQ %s highestP %s q:%e p:%e \n", "BTC-"+crtcy.Name, highestQ.String(),
					highestP.String(), q, p)
				continue
			}
			crtcy.Quantity = crtcy.Quantity.Sub(highestQ)
			msg := fmt.Sprintf("sell %s successfully, highestQ %s highestP %s q:%e p:%e uuid %s left %s\n", "BTC-"+crtcy.Name, highestQ.String(),
				highestP.String(), q, p, uuid, crtcy.Quantity.String())
			WriteReport("./data/minorderdebug.log", msg)

			ftotSell += q * p * (1 - 0.0025)
		}
	}

	if OrderPlaced == true {

		//to be done
		dftotSell := decimal.NewFromFloat(ftotSell)
		BTC := CurtCurrencyMem{
			Name:     "BTC",
			Quantity: dftotSell,
		}

		data := CurtCurrency{
			Name:     "BTC",
			Quantity: dftotSell.String(),
		}

		/*
			LABEL:
			bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
			balance, err := bittrex.GetBalance("BTC")
			if err != nil {
				Error.Printf("GetBalance BTC %s", err.Error())
				goto LABEL
			}

			BTC := CurtCurrencyMem{
				Name:     "BTC",
				Quantity: balance.Available,
			}

			data := CurtCurrency{
				Name:     "BTC",
				Quantity: balance.Available.String(),
			}

		*/

		msg := fmt.Sprintf("ftotSell %f \n", ftotSell)
		WriteReport("./data/minorderdebug.log", msg)

		b, _ := json.Marshal(data)
		err := ioutil.WriteFile("./config/MinCurtCurrency.json", b, 0644)
		if err != nil {
			Error.Printf("CurtCurrency error %s", err.Error())
		}
		return BTC, nil

	}

	return crtcy, errors.New("sell not finished")

}

func PlaceSellOrderforTenMin(crtcy CurtCurrencyMem) (CurtCurrencyMem, error) {

	rpt := "********************" + time.Now().Format(time.RFC3339) + "*********************\n"
	rpt += "begin to place sell order for :" + crtcy.Name + " \n"
	rpt += "current  currency:" + crtcy.Name + "  Quantity:" + crtcy.Quantity.String() + " \n"
	WriteReport("./data/10minorderdebug.log", rpt)

	zero, _ := decimal.NewFromString("0.0")
	ftotSell := 0.0
	verifiedb := VerifyBalance(crtcy)
	if verifiedb == false {
		return crtcy, errors.New("verify balance failed")
	}

	OrderPlaced := false
	errcnt := 0
	for {

		bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
		data, err := bittrex.GetOrderBookBuySell("BTC-"+crtcy.Name, "buy")

		if err != nil {
			Error.Printf("PlaceSellOrder GetOrderBookBuySell %s\n", err.Error())
			errcnt++
			continue
		}

		if crtcy.Quantity.LessThan(zero) || crtcy.Quantity.Equal(zero) {
			msg := fmt.Sprintf("sell %s successfully, multiple transition \n ", "BTC-"+crtcy.Name)
			WriteReport("./data/10minorderdebug.log", msg)
			OrderPlaced = true
			break
		}

		highestQ := data[0].Quantity
		highestP := data[0].Rate

		if highestQ.GreaterThan(crtcy.Quantity) {
			q, _ := crtcy.Quantity.Float64()
			p, _ := highestP.Float64()
			uuid := "1234"
			var err error
			//to be done
			//uuid, err :=bittrex.SellLimit("BTC-"+crtcy.Name, q, p)

			if err != nil {
				errcnt++
				Error.Printf("sell %s failed, highestQ %s highestP %s q:%e p:%e\n", "BTC-"+crtcy.Name, highestQ.String(),
					highestP.String(), q, p)
				continue
			}
			msg := fmt.Sprintf("sell %s successfully, highestQ %s highestP %s q:%e p:%e uuid %s\n", "BTC-"+crtcy.Name, highestQ.String(),
				highestP.String(), q, p, uuid)
			WriteReport("./data/10minorderdebug.log", msg)
			OrderPlaced = true

			ftotSell += q * p * (1 - 0.0025)

			break
		} else {

			q, _ := highestQ.Float64()
			p, _ := highestP.Float64()
			uuid := "1234"
			var err error
			//to be done
			//uuid, err := bittrex.SellLimit("BTC-"+crtcy.Name, q, p)

			if err != nil {
				errcnt++
				Error.Printf("sell %s failed, highestQ %s highestP %s q:%e p:%e \n", "BTC-"+crtcy.Name, highestQ.String(),
					highestP.String(), q, p)
				continue
			}
			crtcy.Quantity = crtcy.Quantity.Sub(highestQ)
			msg := fmt.Sprintf("sell %s successfully, highestQ %s highestP %s q:%e p:%e uuid %s left %s\n", "BTC-"+crtcy.Name, highestQ.String(),
				highestP.String(), q, p, uuid, crtcy.Quantity.String())
			WriteReport("./data/10minorderdebug.log", msg)

			ftotSell += q * p * (1 - 0.0025)
		}
	}

	if OrderPlaced == true {

		//to be done
		dftotSell := decimal.NewFromFloat(ftotSell)
		BTC := CurtCurrencyMem{
			Name:     "BTC",
			Quantity: dftotSell,
		}

		data := CurtCurrency{
			Name:     "BTC",
			Quantity: dftotSell.String(),
		}

		/*
			LABEL:
			bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
			balance, err := bittrex.GetBalance("BTC")
			if err != nil {
				Error.Printf("GetBalance BTC %s", err.Error())
				goto LABEL
			}

			BTC := CurtCurrencyMem{
				Name:     "BTC",
				Quantity: balance.Available,
			}

			data := CurtCurrency{
				Name:     "BTC",
				Quantity: balance.Available.String(),
			}

		*/

		msg := fmt.Sprintf("ftotSell %f \n", ftotSell)
		WriteReport("./data/10minorderdebug.log", msg)

		b, _ := json.Marshal(data)
		err := ioutil.WriteFile("./config/10MinCurtCurrency.json", b, 0644)
		if err != nil {
			Error.Printf("CurtCurrency error %s", err.Error())
		}
		return BTC, nil

	}

	return crtcy, errors.New("sell not finished")

}

func whentoSellforMin(smap map[string][]*MarketSummary, crtcy *CurtCurrencyMem) bool {
	zero, _ := decimal.NewFromString("0.0")
	line5p, _ := decimal.NewFromString("0.05")
	line10p, _ := decimal.NewFromString("0.10")
	line20p, _ := decimal.NewFromString("0.20")
	line80p, _ := decimal.NewFromString("0.80")

	maket := smap[Triggled.coin]
	l := len(maket)
	vcurrt := maket[l-1]
	top := maket[l-1].High

	dif := top.Sub(Triggled.purchased)
	purchased := Triggled.purchased

	if vcurrt.Last.GreaterThan(Triggled.localnewhigh) {
		Triggled.localnewhigh = vcurrt.Last
	}

	if Triggled.newhigh == false {
		if top.GreaterThan(maket[0].High) {
			Triggled.newhigh = true //
		}
	}
	dif1 := top.Sub(vcurrt.Last)
	dif2 := Triggled.localnewhigh.Sub(vcurrt.Last)

	if dif1.LessThan(zero) {
		WriteReport("./data/minorderdebug.log", "lose compared to purchased, sell\n")
		return true
	}

	perc := dif.Div(purchased)

	perclost, _ := decimal.NewFromString("0.0")
	if Triggled.newhigh {
		perclost = dif1.Div(purchased)

		msga := fmt.Sprintf("top (%s) - purchased (%s) = dif (%s) \n", top.String(), purchased.String(), dif.String())
		msga += fmt.Sprintf("top (%s) - vcurrt.Last (%s) = dif1 (%s) \n", top.String(), vcurrt.Last.String(), dif1.String())
		msga += fmt.Sprintf("perc (%s) = dif(%s) / purchased(%s) \n", perc.String(), dif.String(), purchased.String())
		msga += fmt.Sprintf("perclost (%s) = dif1(%s) / purchased(%s) \n", perclost.String(), dif1.String(), purchased.String())

		WriteReport("./data/minorderdebug.log", "triggled new high \n"+msga)

	} else {
		perclost = dif2.Div(purchased)
		msga := fmt.Sprintf("top (%s) - purchased (%s) = dif (%s) \n", top.String(), purchased.String(), dif.String())
		msga += fmt.Sprintf("localnewhigh (%s) - vcurrt.Last (%s) = dif2 (%s) \n", Triggled.localnewhigh.String(), vcurrt.Last.String(), dif2.String())
		msga += fmt.Sprintf("perc (%s) = dif(%s) / purchased(%s) \n", perc.String(), dif.String(), purchased.String())
		msga += fmt.Sprintf("perclost (%s) = dif2(%s) / purchased(%s) \n", perclost.String(), dif2.String(), purchased.String())
		WriteReport("./data/minorderdebug.log", "use local high \n")
	}

	if perc.GreaterThanOrEqual(line5p) && perc.LessThanOrEqual(line10p) {
		temp, _ := decimal.NewFromString("0.04")
		if perclost.GreaterThan(temp) {
			WriteReport("./data/minorderdebug.log", "5% ~ 10%, drop more than 4% \n")
			return true
		}
	}

	if perc.GreaterThanOrEqual(line10p) && perc.LessThanOrEqual(line20p) {
		temp, _ := decimal.NewFromString("0.05")
		if perclost.GreaterThan(temp) {
			WriteReport("./data/minorderdebug.log", "10% ~ 20%, drop more than 5% \n")
			return true
		}
	}

	if perc.GreaterThanOrEqual(line20p) && perc.LessThanOrEqual(line80p) {
		temp, _ := decimal.NewFromString("0.05")
		if perclost.GreaterThan(temp) {
			WriteReport("./data/minorderdebug.log", "20% ~ 80%, drop more than 5% \n")
			return true
		}
	}

	dura := time.Since(Triggled.TriggledTime)
	if dura.Minutes() > 60 {
		if dif1.LessThan(zero) {
			WriteReport("./data/minorderdebug.log", "60 mins more, losing , keep waiting\n")
			return false
		}
		WriteReport("./data/minorderdebug.log", "60 mins more, profit , sell\n")
		return true
	}

	if perc.GreaterThan(line80p) {
		WriteReport("./data/minorderdebug.log", "80 percent more, profit , sell\n")
		return true
	}

	return false
}

func MinTasktosell(smap map[string][]*MarketSummary, crtcy *CurtCurrencyMem) {

	maket := smap[Triggled.coin]
	l := len(maket)
	vcurrt := maket[l-1]
	top := maket[l-1].High

	dif := top.Sub(vcurrt.Last)

	if whentoSellforMin(smap, crtcy) == false {
		return
	}

	*crtcy, _ = PlaceSellOrderforMin(*crtcy)

	//empty the coin
	Triggled = TriggledMin{}

	msg := "coin:" + Triggled.coin
	msg += " current:" + vcurrt.Last.String()
	msg += " top:" + top.String()
	msg += " dif:" + dif.String()
	msg += " percentage:" + dif.Div(top).String()
	msg += " \n"

	WriteReport("./data/minorderdebug.log", msg)

}

func HourTask(crtcy *CurtCurrencyMem, hourSlicetop [][]SummaryReport) {

	hourhotmkt := ""
	if crtcy.Name == "BTC" {
		hourhotmkt = GetHourHotMaket(hourSlicetop)
		if hourhotmkt != "" {
			var err error
			*crtcy, err = PlaceBuyOrderforHour(hourhotmkt, *crtcy)
			if err != nil {
				Error.Printf("HourTask PlaceBuyOrderforHour : %s", err.Error())
			}
		}
	}

	if crtcy.Name != "BTC" && hourhotmkt != "" {
		if "BTC-"+crtcy.Name == hourhotmkt {
			msg := "Already bought:" + hourhotmkt + " and it is still hign vulume\n"
			WriteReport("./data/hourorderdebug.log", msg)
			return
		}
		msg := TimeStamp() + " hotmarket is:" + hourhotmkt + " which is different to current:" + "BTC-" + crtcy.Name + "\n"
		msg += TimeStamp() + " place sell order"
		WriteReport("./data/hourorderdebug.log", msg)
		var err error
		*crtcy, err = PlaceSellOrderforHour(*crtcy)
		if err != nil {
			Error.Printf("%s HourTask PlaceSellOrderforHour %s", TimeStamp(), err.Error())
		}

		*crtcy, err = PlaceBuyOrderforHour(hourhotmkt, *crtcy)
		if err != nil {
			Error.Printf("HourTask PlaceBuyOrderforHour : %s", err.Error())
		}
	}

	if crtcy.Name != "BTC" && hourhotmkt == "" {
		b := WhenToSellForHour(*crtcy, hourSlicetop)
		if b {
			var err error
			*crtcy, err = PlaceSellOrderforHour(*crtcy)
			if err != nil {

				Error.Printf("%s HourTask PlaceSellOrderforHour %s", TimeStamp(), err.Error())
			}
		}
	}

	return

}

func GetHourHotMaket(hourSlicetop [][]SummaryReport) string {
	hourbar, _ := decimal.NewFromString("1000.0")
	zero, _ := decimal.NewFromString("0.0")

	matchmap := make(map[string]int)
	matchmapV := make(map[string]decimal.Decimal)
	for _, v := range hourSlicetop {
		for _, v1 := range v {
			if v1.Vchg.GreaterThan(hourbar) && v1.Pchg.GreaterThan(zero) {
				matchmap[v1.MarketName]++
				matchmapV[v1.MarketName] = matchmapV[v1.MarketName].Add(v1.Vchg)
			}
		}
	}

	topv, _ := decimal.NewFromString("0.0")
	hotmkt := ""
	for k, v := range matchmap {
		if v == 2 {
			if matchmapV[k].GreaterThan(topv) {
				topv = matchmapV[k]
				hotmkt = k
			}

			msg := TimeStamp() + "h otmkt: " + k + " \n"
			WriteReport("./data/hourorderdebug.log", msg)
		}
	}

	return hotmkt
}

func TimeStamp() string {
	return time.Now().Format(time.RFC3339) + " "
}

func PlaceBuyOrderforHour(buy string, crtcy CurtCurrencyMem) (CurtCurrencyMem, error) {
	zero, _ := decimal.NewFromString("0.0")
	costRate, _ := decimal.NewFromString("0.0025")
	rpt := "********************" + time.Now().Format(time.RFC3339) + "*********************\n"
	rpt += "begin to place buy order for :" + buy + " \n"
	rpt += "current  currency:" + crtcy.Name + "  Quantity:" + crtcy.Quantity.String() + " \n"
	WriteReport("./data/hourorderdebug.log", rpt)

	totBought := 0.0

	verifiedb := VerifyBalance(crtcy)
	if verifiedb == false {
		return crtcy, errors.New("verify balance failed")
	}

	bOrderPlaced := false
	for {

		if crtcy.Quantity.LessThan(zero) || crtcy.Quantity.Equal(zero) {
			msg := fmt.Sprintf("buy %s successfully, multiple transition \n", "BTC-"+buy)
			WriteReport("./data/hourorderdebug.log", msg)
			bOrderPlaced = true
			break
		}
		bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
		data, err := bittrex.GetOrderBookBuySell(buy, "sell")

		if err != nil {
			Error.Printf("GetAllOrderTask, %s %s\n", buy, err.Error())
			continue
		}

		if len(data) == 0 {
			Error.Printf("GetAllOrderTask, %s\n", buy)
			continue
		}
		//process order here, current currency have to be BTC

		LowestQ := data[0].Quantity
		LowestP := data[0].Rate

		if LowestP.Sub(Triggled.purchased).GreaterThan(zero) {
			Triggled.purchased = LowestP
		}

		total := crtcy.Quantity
		cost := crtcy.Quantity.Mul(costRate)

		left := total.Sub(cost)
		Qty := left.Div(LowestP)

		if LowestQ.GreaterThan(Qty) {
			q, _ := Qty.Float64()
			p, _ := LowestP.Float64()
			uuid := "1234"
			var err error
			//to be done
			//uuid, err := bittrex.BuyLimit(buy, q, p)

			if err != nil {
				Error.Printf("buy %s failed, LowestQ %s LowestP %s q:%e p:%e\n", buy, LowestQ.String(),
					LowestP.String(), q, p)
				continue
			}

			msg := fmt.Sprintf(time.Now().Format(time.RFC3339)+" buy %s successfully, LowestQ %s LowestP %s q:%e p:%e uuid: %s\n", buy, LowestQ.String(),
				LowestP.String(), q, p, uuid)
			WriteReport("./data/hourorderdebug.log", msg)
			totBought += q
			bOrderPlaced = true
			break
		} else {

			q, _ := LowestQ.Float64()
			p, _ := LowestP.Float64()

			uuid := "1234"
			var err error
			//to be done
			//uuid, err := bittrex.BuyLimit(buy, q, p)

			if err != nil {
				Error.Printf("buy %s failed, LowestQ %s LowestP %s q:%e p:%e\n", buy, LowestQ.String(),
					LowestP.String(), q, p)
				continue
			}

			crtcy.Quantity = crtcy.Quantity.Sub(LowestQ.Mul(LowestP))
			msg := fmt.Sprintf(time.Now().Format(time.RFC3339)+" buy %s partially, LowestQ %s LowestP %s q:%e p:%e uuid: %s, left %s\n", buy, LowestQ.String(),
				LowestP.String(), q, p, uuid, crtcy.Quantity.String())
			WriteReport("./data/hourorderdebug.log", msg)
			totBought += q

		}
	}

	if bOrderPlaced == true {
		//LABEL:
		//to be done
		dtotBought := decimal.NewFromFloat(totBought)

		curent := CurtCurrencyMem{
			Name:     buy[4:],
			Quantity: dtotBought,
		}

		data := CurtCurrency{
			Name:     buy[4:],
			Quantity: dtotBought.String(),
		}

		/*bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
		balance, err := bittrex.GetBalance(buy)
		if err != nil {
			Error.Printf("GetBalance %s %s\n", buy, err.Error())
			goto LABEL
		}

		curent := CurtCurrencyMem{
			Name:     buy[4:],
			Quantity: balance.Available,
		}

		data := CurtCurrency{
			Name:     buy[4:],
			Quantity: balance.Available.String(),
		}*/

		msg := fmt.Sprintf("dtotBought %s \n", dtotBought.String())
		WriteReport("./data/hourorderdebug.log", msg)

		b, _ := json.Marshal(data)
		err := ioutil.WriteFile("./config/HourCurtCurrency.json", b, 0644)
		if err != nil {
			Error.Printf("CurtCurrency error %s\n", err.Error())
		}
		return curent, nil

	}

	return crtcy, errors.New("buy oder failed")

}

func PlaceSellOrderforHour(crtcy CurtCurrencyMem) (CurtCurrencyMem, error) {

	rpt := "********************" + time.Now().Format(time.RFC3339) + "*********************\n"
	rpt += "begin to place sell order for :" + crtcy.Name + " \n"
	rpt += "current  currency:" + crtcy.Name + "  Quantity:" + crtcy.Quantity.String() + " \n"
	WriteReport("./data/hourorderdebug.log", rpt)

	zero, _ := decimal.NewFromString("0.0")
	ftotSell := 0.0
	verifiedb := VerifyBalance(crtcy)
	if verifiedb == false {
		return crtcy, errors.New("verify balance failed")
	}

	OrderPlaced := false
	errcnt := 0
	for {

		bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
		data, err := bittrex.GetOrderBookBuySell("BTC-"+crtcy.Name, "buy")

		if err != nil {
			Error.Printf("PlaceSellOrder GetOrderBookBuySell %s\n", err.Error())
			errcnt++
			continue
		}

		if crtcy.Quantity.LessThan(zero) || crtcy.Quantity.Equal(zero) {
			msg := fmt.Sprintf("sell %s successfully, multiple transition \n ", "BTC-"+crtcy.Name)
			WriteReport("./data/hourorderdebug.log", msg)
			OrderPlaced = true
			break
		}

		highestQ := data[0].Quantity
		highestP := data[0].Rate

		if highestQ.GreaterThan(crtcy.Quantity) {
			q, _ := crtcy.Quantity.Float64()
			p, _ := highestP.Float64()
			uuid := "1234"
			var err error
			//to be done
			//uuid, err :=bittrex.SellLimit("BTC-"+crtcy.Name, q, p)

			if err != nil {
				errcnt++
				Error.Printf("sell %s failed, highestQ %s highestP %s q:%e p:%e\n", "BTC-"+crtcy.Name, highestQ.String(),
					highestP.String(), q, p)
				continue
			}
			msg := fmt.Sprintf("sell %s successfully, highestQ %s highestP %s q:%e p:%e uuid %s\n", "BTC-"+crtcy.Name, highestQ.String(),
				highestP.String(), q, p, uuid)
			WriteReport("./data/hourorderdebug.log", msg)
			OrderPlaced = true

			ftotSell += q * p * (1 - 0.0025)

			break
		} else {

			q, _ := highestQ.Float64()
			p, _ := highestP.Float64()
			uuid := "1234"
			var err error
			//to be done
			//uuid, err := bittrex.SellLimit("BTC-"+crtcy.Name, q, p)

			if err != nil {
				errcnt++
				Error.Printf("sell %s failed, highestQ %s highestP %s q:%e p:%e \n", "BTC-"+crtcy.Name, highestQ.String(),
					highestP.String(), q, p)
				continue
			}
			crtcy.Quantity = crtcy.Quantity.Sub(highestQ)
			msg := fmt.Sprintf("sell %s successfully, highestQ %s highestP %s q:%e p:%e uuid %s left %s\n", "BTC-"+crtcy.Name, highestQ.String(),
				highestP.String(), q, p, uuid, crtcy.Quantity.String())
			WriteReport("./data/hourorderdebug.log", msg)

			ftotSell += q * p * (1 - 0.0025)
		}
	}

	if OrderPlaced == true {

		//to be done
		dftotSell := decimal.NewFromFloat(ftotSell)
		BTC := CurtCurrencyMem{
			Name:     "BTC",
			Quantity: dftotSell,
		}

		data := CurtCurrency{
			Name:     "BTC",
			Quantity: dftotSell.String(),
		}

		/*
			LABEL:
			bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
			balance, err := bittrex.GetBalance("BTC")
			if err != nil {
				Error.Printf("GetBalance BTC %s", err.Error())
				goto LABEL
			}

			BTC := CurtCurrencyMem{
				Name:     "BTC",
				Quantity: balance.Available,
			}

			data := CurtCurrency{
				Name:     "BTC",
				Quantity: balance.Available.String(),
			}

		*/

		msg := fmt.Sprintf("ftotSell %f \n", ftotSell)
		WriteReport("./data/hourorderdebug.log", msg)

		b, _ := json.Marshal(data)
		err := ioutil.WriteFile("./config/HourCurtCurrency.json", b, 0644)
		if err != nil {
			Error.Printf("CurtCurrency error %s", err.Error())
		}
		return BTC, nil

	}

	return crtcy, errors.New("sell not finished")

}

func WhenToSellForHour(crtcy CurtCurrencyMem, hourSlicetop [][]SummaryReport) bool {

	curmkt := "BTC-" + crtcy.Name

	hourVbar, _ := decimal.NewFromString("200.0")

	l := len(hourSlicetop)
	lastestreport := hourSlicetop[l-1]

	find := false
	var latestrpt SummaryReport
	for _, v := range lastestreport {
		if v.MarketName == curmkt {
			latestrpt = v
			find = true
		}
	}

	if find == false {
		//even cannot find the top vulume
		msg := TimeStamp() + "cannot find curmkt:" + curmkt + "\n"
		WriteReport("./data/hourorderdebug.log", msg)
		return true
	}

	if latestrpt.Vchg.LessThan(hourVbar) {
		msg := TimeStamp() + "curmkt:" + curmkt + "vchange less than" + hourVbar.String() + "\n"
		WriteReport("./data/hourorderdebug.log", msg)
		return true
	}
	zero, _ := decimal.NewFromString("0.0")
	NEG, _ := decimal.NewFromString("0.0")
	Perc15bar, _ := decimal.NewFromString("0.20")
	pchange := latestrpt.Pchg
	if pchange.LessThan(zero) {
		pospchange := pchange.Mul(NEG)
		if pospchange.Div(latestrpt.Pfrom).GreaterThan(Perc15bar) {

			msg := TimeStamp() + "curmkt:" + curmkt + "price drop more than 20 percent" + "\n"
			WriteReport("./data/hourorderdebug.log", msg)
			return true
		}
	}

	return false
}

func whentoSellforTenMin(smap map[string][]*MarketSummary, crtcy *CurtCurrencyMem) bool {

	marketcoin := "BTC-" + crtcy.Name

	MarketSum, b := smap[marketcoin]

	if b == false {
		WriteReport("./data/10minorderdebug.log", "cannot find the coin in market, that is not right.\n")
		return false
	}

	l := len(MarketSum)
	if l < 10 {
		WriteReport("./data/10minorderdebug.log", "lenth less than 10 in market, that is not right.\n")
		return false
	}

	vdif := MarketSum[l-1].BaseVolume.Sub(MarketSum[l-10].BaseVolume)

	difbar, _ := decimal.NewFromString("10.0")
	if vdif.LessThan(difbar) {
		return true
	}

	return false
}

func whentoSellforFifTeenMin(smap map[string][]*MarketSummary, crtcy *CurtCurrencyMem) bool {

	marketcoin := "BTC-" + crtcy.Name

	MarketSum, b := smap[marketcoin]

	if b == false {
		WriteReport("./data/15minorderdebug.log", "cannot find the coin in market, that is not right.\n")
		return false
	}

	l := len(MarketSum)
	if l < 15 {
		WriteReport("./data/15minorderdebug.log", "lenth less than 15 in market, that is not right.\n")
		return false
	}

	vdif := MarketSum[l-1].BaseVolume.Sub(MarketSum[l-15].BaseVolume)

	difbar, _ := decimal.NewFromString("50.0")
	if vdif.LessThan(difbar) {
		return true
	}

	return false
}
func PlaceSellOrderforFifTeenMin(crtcy CurtCurrencyMem) (CurtCurrencyMem, error) {

	rpt := "********************" + time.Now().Format(time.RFC3339) + "*********************\n"
	rpt += "begin to place sell order for :" + crtcy.Name + " \n"
	rpt += "current  currency:" + crtcy.Name + "  Quantity:" + crtcy.Quantity.String() + " \n"
	WriteReport("./data/15minorderdebug.log", rpt)

	zero, _ := decimal.NewFromString("0.0")
	ftotSell := 0.0
	verifiedb := VerifyBalance(crtcy)
	if verifiedb == false {
		return crtcy, errors.New("verify balance failed")
	}

	OrderPlaced := false
	errcnt := 0
	for {

		bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
		data, err := bittrex.GetOrderBookBuySell("BTC-"+crtcy.Name, "buy")

		if err != nil {
			Error.Printf("PlaceSellOrder GetOrderBookBuySell %s\n", err.Error())
			errcnt++
			continue
		}

		if crtcy.Quantity.LessThan(zero) || crtcy.Quantity.Equal(zero) {
			msg := fmt.Sprintf("sell %s successfully, multiple transition \n ", "BTC-"+crtcy.Name)
			WriteReport("./data/15minorderdebug.log", msg)
			OrderPlaced = true
			break
		}

		highestQ := data[0].Quantity
		highestP := data[0].Rate

		if highestQ.GreaterThan(crtcy.Quantity) {
			q, _ := crtcy.Quantity.Float64()
			p, _ := highestP.Float64()
			uuid := "1234"
			var err error
			//to be done
			//uuid, err :=bittrex.SellLimit("BTC-"+crtcy.Name, q, p)

			if err != nil {
				errcnt++
				Error.Printf("sell %s failed, highestQ %s highestP %s q:%e p:%e\n", "BTC-"+crtcy.Name, highestQ.String(),
					highestP.String(), q, p)
				continue
			}
			msg := fmt.Sprintf("sell %s successfully, highestQ %s highestP %s q:%e p:%e uuid %s\n", "BTC-"+crtcy.Name, highestQ.String(),
				highestP.String(), q, p, uuid)
			WriteReport("./data/15minorderdebug.log", msg)
			OrderPlaced = true

			ftotSell += q * p * (1 - 0.0025)

			break
		} else {

			q, _ := highestQ.Float64()
			p, _ := highestP.Float64()
			uuid := "1234"
			var err error
			//to be done
			//uuid, err := bittrex.SellLimit("BTC-"+crtcy.Name, q, p)

			if err != nil {
				errcnt++
				Error.Printf("sell %s failed, highestQ %s highestP %s q:%e p:%e \n", "BTC-"+crtcy.Name, highestQ.String(),
					highestP.String(), q, p)
				continue
			}
			crtcy.Quantity = crtcy.Quantity.Sub(highestQ)
			msg := fmt.Sprintf("sell %s successfully, highestQ %s highestP %s q:%e p:%e uuid %s left %s\n", "BTC-"+crtcy.Name, highestQ.String(),
				highestP.String(), q, p, uuid, crtcy.Quantity.String())
			WriteReport("./data/15minorderdebug.log", msg)

			ftotSell += q * p * (1 - 0.0025)
		}
	}

	if OrderPlaced == true {

		//to be done
		dftotSell := decimal.NewFromFloat(ftotSell)
		BTC := CurtCurrencyMem{
			Name:     "BTC",
			Quantity: dftotSell,
		}

		data := CurtCurrency{
			Name:     "BTC",
			Quantity: dftotSell.String(),
		}

		/*
			LABEL:
			bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
			balance, err := bittrex.GetBalance("BTC")
			if err != nil {
				Error.Printf("GetBalance BTC %s", err.Error())
				goto LABEL
			}

			BTC := CurtCurrencyMem{
				Name:     "BTC",
				Quantity: balance.Available,
			}

			data := CurtCurrency{
				Name:     "BTC",
				Quantity: balance.Available.String(),
			}

		*/

		msg := fmt.Sprintf("ftotSell %f \n", ftotSell)
		WriteReport("./data/15minorderdebug.log", msg)

		b, _ := json.Marshal(data)
		err := ioutil.WriteFile("./config/15MinCurtCurrency.json", b, 0644)
		if err != nil {
			Error.Printf("CurtCurrency error %s", err.Error())
		}
		return BTC, nil

	}

	return crtcy, errors.New("sell not finished")

}

func PlaceBuyOrderforFifteenmin(buy string, crtcy CurtCurrencyMem) (CurtCurrencyMem, error) {
	zero, _ := decimal.NewFromString("0.0")
	costRate, _ := decimal.NewFromString("0.0025")
	rpt := "********************" + time.Now().Format(time.RFC3339) + "*********************\n"
	rpt += "begin to place buy order for :" + buy + " \n"
	rpt += "current  currency:" + crtcy.Name + "  Quantity:" + crtcy.Quantity.String() + " \n"
	WriteReport("./data/15minorderdebug.log", rpt)

	totBought := 0.0

	verifiedb := VerifyBalance(crtcy)
	if verifiedb == false {
		return crtcy, errors.New("verify balance failed")
	}

	bOrderPlaced := false
	for {

		if crtcy.Quantity.LessThan(zero) || crtcy.Quantity.Equal(zero) {
			msg := fmt.Sprintf("buy %s successfully, multiple transition \n", "BTC-"+buy)
			WriteReport("./data/15minorderdebug.log", msg)
			bOrderPlaced = true
			break
		}
		bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
		data, err := bittrex.GetOrderBookBuySell(buy, "sell")

		if err != nil {
			Error.Printf("GetAllOrderTask, %s %s\n", buy, err.Error())
			continue
		}

		if len(data) == 0 {
			Error.Printf("GetAllOrderTask, %s\n", buy)
			continue
		}
		//process order here, current currency have to be BTC

		LowestQ := data[0].Quantity
		LowestP := data[0].Rate

		if LowestP.Sub(Triggled.purchased).GreaterThan(zero) {
			Triggled.purchased = LowestP
		}

		total := crtcy.Quantity
		cost := crtcy.Quantity.Mul(costRate)

		left := total.Sub(cost)
		Qty := left.Div(LowestP)

		if LowestQ.GreaterThan(Qty) {
			q, _ := Qty.Float64()
			p, _ := LowestP.Float64()
			uuid := "1234"
			var err error
			//to be done
			//uuid, err := bittrex.BuyLimit(buy, q, p)

			if err != nil {
				Error.Printf("buy %s failed, LowestQ %s LowestP %s q:%e p:%e\n", buy, LowestQ.String(),
					LowestP.String(), q, p)
				continue
			}

			msg := fmt.Sprintf(time.Now().Format(time.RFC3339)+" buy %s successfully, LowestQ %s LowestP %s q:%e p:%e uuid: %s\n", buy, LowestQ.String(),
				LowestP.String(), q, p, uuid)
			WriteReport("./data/15minorderdebug.log", msg)
			totBought += q
			bOrderPlaced = true
			break
		} else {

			q, _ := LowestQ.Float64()
			p, _ := LowestP.Float64()

			uuid := "1234"
			var err error
			//to be done
			//uuid, err := bittrex.BuyLimit(buy, q, p)

			if err != nil {
				Error.Printf("buy %s failed, LowestQ %s LowestP %s q:%e p:%e\n", buy, LowestQ.String(),
					LowestP.String(), q, p)
				continue
			}

			crtcy.Quantity = crtcy.Quantity.Sub(LowestQ.Mul(LowestP))
			msg := fmt.Sprintf(time.Now().Format(time.RFC3339)+" buy %s partially, LowestQ %s LowestP %s q:%e p:%e uuid: %s, left %s\n", buy, LowestQ.String(),
				LowestP.String(), q, p, uuid, crtcy.Quantity.String())
			WriteReport("./data/15minorderdebug.log", msg)
			totBought += q

		}
	}

	if bOrderPlaced == true {
		//LABEL:
		//to be done
		dtotBought := decimal.NewFromFloat(totBought)

		curent := CurtCurrencyMem{
			Name:     buy[4:],
			Quantity: dtotBought,
		}

		data := CurtCurrency{
			Name:     buy[4:],
			Quantity: dtotBought.String(),
		}

		/*bittrex := bittrex.New(config.ApiKey, config.ApiSecret)
		balance, err := bittrex.GetBalance(buy)
		if err != nil {
			Error.Printf("GetBalance %s %s\n", buy, err.Error())
			goto LABEL
		}

		curent := CurtCurrencyMem{
			Name:     buy[4:],
			Quantity: balance.Available,
		}

		data := CurtCurrency{
			Name:     buy[4:],
			Quantity: balance.Available.String(),
		}*/

		msg := fmt.Sprintf("dtotBought %s \n", dtotBought.String())
		WriteReport("./data/15minorderdebug.log", msg)

		b, _ := json.Marshal(data)
		err := ioutil.WriteFile("./config/15MinCurtCurrency.json", b, 0644)
		if err != nil {
			Error.Printf("CurtCurrency error %s\n", err.Error())
		}
		return curent, nil

	}

	return crtcy, errors.New("buy oder failed")

}

type minMonitor struct {
	vdif  decimal.Decimal
	mtime time.Time
}

var minMonitorMap = make(map[string]minMonitor)

func MinTasktobuy1try(sdifmap map[string][]MINDIF, mincrtcy *CurtCurrencyMem) {
	//list LTC coin as example
	// need more time to collect sample
	ltc, b := sdifmap["BTC-LTC"]
	if b == false || len(ltc) < 60 {
		return
	}
	zero, _ := decimal.NewFromString("0.0")
	//one, _ := decimal.NewFromString("1.0")
	NEG, _ := decimal.NewFromString("-1")
	Rang, _ := decimal.NewFromString("30.0")
	avgmap := make(map[string]decimal.Decimal)
	avgprice := make(map[string]decimal.Decimal)

	for k, v := range sdifmap {
		sampledata := v[20:50] // got 30 min sample
		sum, _ := decimal.NewFromString("0.0")
		sumprice, _ := decimal.NewFromString("0.0")
		for _, v1 := range sampledata {
			if v1.BaseVolumedif.LessThan(zero) == true {
				v1.BaseVolumedif = v1.BaseVolumedif.Mul(NEG)
			}
			sum = sum.Add(v1.BaseVolumedif)
			sumprice = sumprice.Add(v1.Last)
		}
		avg := sum.Div(Rang)
		avgp := sumprice.Div(Rang)

		avgmap[k] = avg
		avgprice[k] = avgp
	}

	timesmap := make(map[string]decimal.Decimal)
	for k, v := range sdifmap {
		l := len(v)
		avg := avgmap[k]
		if avg.LessThanOrEqual(zero) {
			//WriteReport("./data/minorderdebug.log", "wrong div:"+k+"v:"+avg.String()+" \n")
			continue
		}

		/* 		if v[l-1].BaseVolumedif.LessThanOrEqual(one) {
			continue
		} */

		timesmap[k] = v[l-1].BaseVolumedif.Div(avgmap[k])
	}

	top3 := make([]TOP3Times, 3, 3)
	for k, v := range timesmap {
		for ktop, vtop := range top3 {

			if v.Sub(vtop.times).GreaterThan(zero) {
				copy(top3[ktop+1:], top3[ktop:])
				top3[ktop] = TOP3Times{
					coin:  k,
					times: v,
				}
				break
			}
		}

	}

	msg := ""
	for i := range top3 {
		l1 := len(sdifmap[top3[i].coin])
		if l1 < 1 {
			Error.Printf("top3 l1 less than 1, %v\n", top3)
			continue
		}
		msg += TimeStamp()
		msg += "coin: "
		msg += top3[i].coin
		msg += " times: "
		msg += top3[i].times.String()
		msg += " avgdif: " + avgmap[top3[i].coin].String()

		msg += " Last: " + sdifmap[top3[i].coin][l1-1].Last.String()
		msg += " High: " + sdifmap[top3[i].coin][l1-1].High.String()
		msg += " volumedif: " + sdifmap[top3[i].coin][l1-1].BaseVolumedif.String()
		msg += " \n"
	}

	WriteReport("./data/minorderdebug1try.log", msg)

	lockcoin, locked := glockcmap[top3[0].coin]
	if locked {
		if time.Since(lockcoin.locktime).Seconds() < 60*60 {
			msg := "this coin locked by reason:" + lockcoin.lockreason + "\n"
			WriteReport("./data/minorderdebug1try.log", msg)
			return
		}
	}

	//only process top 1 now, the other 2 can process later
	TRIGGLE, _ := decimal.NewFromString("10.0")

	ONE, _ := decimal.NewFromString("1.0")
	pricebar, _ := decimal.NewFromString("0.30")
	btriggled := false
	newtriglecoin := ""

	for mk, mv := range minMonitorMap {
		timedif := time.Since(mv.mtime).Seconds()
		if timedif < 5 || timedif > 65 {
			continue
		}

		temp := sdifmap[mk]
		l := len(temp)
		if l < 1 {
			continue
		}

		if temp[l-1].BaseVolumedif.GreaterThanOrEqual(ONE) {
			newtriglecoin = mk
			msg := "second time triggled succesfully:" + mk + "\n"
			WriteReport("./data/minorderdebug1try.log", TimeStamp()+msg)
			btriggled = true
			break
		}
	}

	for i := range top3 {
		Times10 := top3[i].times.Sub(TRIGGLE).GreaterThan(zero)
		l1 := len(sdifmap[top3[i].coin])
		if l1 < 1 {
			continue
		}
		dif := sdifmap[top3[i].coin][l1-1].Last.Sub(avgprice[top3[i].coin])

		if dif.Div(avgprice[top3[i].coin]).GreaterThan(pricebar) {
			msg := "Price increase over 30%, maybe not to buy: " + top3[i].coin + "\n"
			msg += "try to lock coin: " + top3[i].coin + "\n"
			WriteReport("./data/minorderdebug1try.log", msg)

			lockcoin := lockedcoin{
				coin:       top3[i].coin,
				locktime:   time.Now(),
				lockreason: "Price increase over 30%",
			}
			glockcmap[lockcoin.coin] = lockcoin

			continue
		}

		if Times10 {

			if sdifmap[top3[i].coin][l1-1].BaseVolumedif.GreaterThan(ONE) {

				minMonitorcoin := minMonitor{
					vdif:  sdifmap[top3[i].coin][l1-1].BaseVolumedif,
					mtime: time.Now(),
				}
				minMonitorMap[top3[i].coin] = minMonitorcoin
				WriteReport("./data/minorderdebug1try.log", TimeStamp()+"monitor coin: "+top3[i].coin+"\n")
			}
		}

	}

	if btriggled == false {
		return
	}

	if Triggled.coin != "" {
		msg = "Already had trigged coin: " + Triggled.coin + " "
		if top3[0].coin == "BTC-"+mincrtcy.Name {
			msg += "new triggled coin is the same. in the safe mode.\n"
		} else {
			msg += "new triggled coin is " + top3[0].coin + "\n"
		}

		WriteReport("./data/minorderdebug1try.log", msg)
		return
	}

	if mincrtcy.Name != "BTC" {
		msg = "Current coin: " + mincrtcy.Name + " is not BTC\n"
		WriteReport("./data/minorderdebug1try.log", msg)
		return
	}

	if btriggled {
		msg = "should buy but disabled: " + newtriglecoin
		WriteReport("./data/minorderdebug1try.log", TimeStamp()+msg)
		return
	}

	if btriggled {
		Triggled = TriggledMin{
			coin:         top3[0].coin,
			purchased:    zero, // price here is not right
			TriggledTime: time.Now(),
		}
		lockcoin := lockedcoin{
			coin:       top3[0].coin,
			locktime:   time.Now(),
			lockreason: "already bought",
		}
		msg := "locked coin " + lockcoin.coin + " when buying\n"
		WriteReport("./data/minorderdebug1try.log", msg)
		glockcmap[lockcoin.coin] = lockcoin
		crtcy, _ := PlaceBuyOrderformin(Triggled.coin, *mincrtcy)
		*mincrtcy = crtcy
	}

}
