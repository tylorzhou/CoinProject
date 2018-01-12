package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
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
