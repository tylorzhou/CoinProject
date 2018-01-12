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
