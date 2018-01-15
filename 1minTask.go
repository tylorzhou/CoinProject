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
	FIVE, _ := decimal.NewFromString("5.0")

	btriggled := false

	l1 := len(sdifmap[top3[0].coin])
	if l1 > 1 {
		pricebar, _ := decimal.NewFromString("0.30")
		btriggled = Times20 && sdifmap[top3[0].coin][l1-1].BaseVolumedif.GreaterThan(FIVE)
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
