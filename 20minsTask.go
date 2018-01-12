package main

import (
	"fmt"

	"github.com/shopspring/decimal"
)

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
