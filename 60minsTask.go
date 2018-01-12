package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	bittrex "github.com/go-bittrex"
	"github.com/shopspring/decimal"
)

type HourRec struct {
	smap  map[string][]*MarketSummary
	Top15 []SummaryReport
}

var HourRecSlice = make([]HourRec, 0, 10)

func SummaryHourReport(smap map[string][]*MarketSummary) []SummaryReport {

	Top15 := make([]SummaryReport, 15, 15)
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

		for i := range Top15 {
			if rpt.Vchg.Sub(Top15[i].Vchg).GreaterThan(zero) {
				copy(Top15[i+1:], Top15[i:])
				Top15[i] = rpt
				break
			}
		}
	}

	smaptemp := make(map[string][]*MarketSummary)

	for k, v := range smap {

		smaptemp[k] = append(smaptemp[k], v...)
	}

	rec := HourRec{
		smap:  smaptemp,
		Top15: Top15,
	}
	HourRecSlice = append(HourRecSlice, rec)
	if len(HourRecSlice) > 10 {
		HourRecSlice = HourRecSlice[1:]
	}

	emailText := ""

	lenhour := len(HourRecSlice)
	for i := range Top15 {
		emailText += fmt.Sprintf("                  %s                   \n", Top15[i].MarketName)
		emailText += fmt.Sprintf("Price     : %s => %s (%%%s) \n", Top15[i].Pfrom.String(), Top15[i].Pto.String(), (Top15[i].Pchg.Div(Top15[i].Pfrom)).Mul(percent).String())
		emailText += fmt.Sprintf("BaseVolume: %s => %s (VolumeChange %s, %%%s) \n", Top15[i].Vfrom.String(), Top15[i].Vto.String(), Top15[i].Vchg.String(),
			Top15[i].Vchg.Div(Top15[i].Vfrom).Mul(percent).String())

		seqtext := ""
		if lenhour > 1 {

			for ihour, v := range HourRecSlice {

				found := 0
				for isum := range v.Top15 {
					if v.Top15[isum].MarketName == Top15[i].MarketName {
						found = isum + 1
						break
					}
				}

				vLastSlice := v.smap[Top15[i].MarketName]
				vlastlen := len(vLastSlice)

				if vlastlen < 1 {
					continue
				}

				vLastchg := vLastSlice[vlastlen-1].BaseVolume.Sub(vLastSlice[0].BaseVolume)

				pLastchg := vLastSlice[vlastlen-1].Last.Sub(vLastSlice[0].Last)
				pchRate := pLastchg.Div(vLastSlice[0].Last).Mul(percent)

				pricechg := ""
				if pchRate.IntPart() > 0 {
					pricechg = fmt.Sprintf("%d", pchRate.IntPart())
				} else {
					pricechg = fmt.Sprintf("%s", pchRate.String()[:4])
				}

				seq := "x"
				if found != 0 {
					seq = strconv.Itoa(found)
				}

				if ihour == 0 {
					emailText += fmt.Sprintf("%d(%s)", vLastchg.IntPart(), pricechg)
					seqtext += fmt.Sprintf("%s", seq)
				} else {
					emailText += fmt.Sprintf("=>%d(%s)", vLastchg.IntPart(), pricechg)
					seqtext += fmt.Sprintf(" %s", seq)
				}

			}

		}

		if seqtext != "" {
			emailText += "\n" + seqtext
		}

		emailText += fmt.Sprintf("\n\n")

		if i == 14 {
			Topic += Top15[i].MarketName
		} else {
			Topic += (Top15[i].MarketName + ",")
		}
	}

	Topic = strings.Replace(Topic, "BTC-", "", -1)

	sendemail(emailText, "Hour report(bittrex):"+Topic)
	History.Print("*************************************************\n")
	History.Print(emailText)
	// only returen top 5
	return Top15[0:5]
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
