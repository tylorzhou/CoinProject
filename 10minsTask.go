package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	bittrex "github.com/go-bittrex"
	"github.com/shopspring/decimal"
)

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
