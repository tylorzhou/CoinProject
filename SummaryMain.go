package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"runtime/debug"
	"strings"
	"time"

	bittrex "github.com/go-bittrex"
	"github.com/shopspring/decimal"
)

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

type CurtCurrency struct {
	Name     string
	Quantity string
}

type CurtCurrencyMem struct {
	Name     string
	Quantity decimal.Decimal
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
