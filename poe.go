package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	raven "github.com/getsentry/raven-go"
	log "gopkg.in/inconshreveable/log15.v2"
)

const poeNinjaCurrencyEndpoint = `https://poe.ninja/api/data/currencyoverview?league=%s&type=Currency`

func formatCurrencyForSlack(pnc PoeNinjaCurrency) string {
	currencies := make(map[string]float64)
	var ret []string
	ret = append(ret, "\n")
	var currencyString string
	for _, l := range pnc.Lines {
		if stringSliceContains(conf.PoE.WantedCurrencies, l.CurrencyTypeName) {
			curName := strings.ToLower(strings.Replace(strings.Replace(l.CurrencyTypeName, " Orb", "", -1), " ", "", -1))
			currencies[curName] = l.ChaosEquivalent
		}
	}
	for _, c := range conf.PoE.WantedCurrencies {
		var extra string
		curName := strings.ToLower(strings.Replace(strings.Replace(c, " Orb", "", -1), " ", "", -1))
		if curName == "mirrorofkalandra" {
			mirrorInEx := currencies[curName] / currencies["exalted"]
			extra = fmt.Sprintf(" %.3f :exalted:", mirrorInEx)
		}
		currencyString = fmt.Sprintf(":%s: : %.3f :chaos:%s", curName, currencies[curName], extra)
		ret = append(ret, currencyString)
	}
	return strings.Join(ret, "\n")
}

func handlePoeCurrencyQuery() string {
	url := fmt.Sprintf(poeNinjaCurrencyEndpoint, conf.PoE.League)
	log.Debug("findRule: Attempting to fetch", "URL", url)
	resp, err := http.Get(url)
	if err != nil {
		raven.CaptureError(err, nil)
		log.Debug("HTTP request to Poe Currency Endpoint failed", "Error", err)
		return ""
	}
	defer resp.Body.Close()
	var pnc PoeNinjaCurrency
	if resp.StatusCode == 200 {
		if err := json.NewDecoder(resp.Body).Decode(&pnc); err != nil {
			raven.CaptureError(err, nil)
			log.Debug("Failed decoding the PoE Currency response", "Error", err)
			return ""
		}
		return formatCurrencyForSlack(pnc)
	}
	return ""
}

type PoeNinjaCurrency struct {
	Lines []struct {
		CurrencyTypeName string `json:"currencyTypeName"`
		Pay              struct {
			ID                int       `json:"id"`
			LeagueID          int       `json:"league_id"`
			PayCurrencyID     int       `json:"pay_currency_id"`
			GetCurrencyID     int       `json:"get_currency_id"`
			SampleTimeUtc     time.Time `json:"sample_time_utc"`
			Count             int       `json:"count"`
			Value             float64   `json:"value"`
			DataPointCount    int       `json:"data_point_count"`
			IncludesSecondary bool      `json:"includes_secondary"`
			ListingCount      int       `json:"listing_count"`
		} `json:"pay,omitempty"`
		Receive struct {
			ID                int       `json:"id"`
			LeagueID          int       `json:"league_id"`
			PayCurrencyID     int       `json:"pay_currency_id"`
			GetCurrencyID     int       `json:"get_currency_id"`
			SampleTimeUtc     time.Time `json:"sample_time_utc"`
			Count             int       `json:"count"`
			Value             float64   `json:"value"`
			DataPointCount    int       `json:"data_point_count"`
			IncludesSecondary bool      `json:"includes_secondary"`
			ListingCount      int       `json:"listing_count"`
		} `json:"receive"`
		PaySparkLine struct {
			Data        []interface{} `json:"data"`
			TotalChange float64       `json:"totalChange"`
		} `json:"paySparkLine"`
		ReceiveSparkLine struct {
			Data        []float64 `json:"data"`
			TotalChange float64   `json:"totalChange"`
		} `json:"receiveSparkLine"`
		ChaosEquivalent           float64 `json:"chaosEquivalent"`
		LowConfidencePaySparkLine struct {
			Data        []interface{} `json:"data"`
			TotalChange float64       `json:"totalChange"`
		} `json:"lowConfidencePaySparkLine"`
		LowConfidenceReceiveSparkLine struct {
			Data        []float64 `json:"data"`
			TotalChange float64   `json:"totalChange"`
		} `json:"lowConfidenceReceiveSparkLine"`
		DetailsID string `json:"detailsId"`
	} `json:"lines"`
	CurrencyDetails []struct {
		ID      int    `json:"id"`
		Icon    string `json:"icon"`
		Name    string `json:"name"`
		TradeID string `json:"tradeId,omitempty"`
	} `json:"currencyDetails"`
	Language struct {
		Name         string `json:"name"`
		Translations struct {
		} `json:"translations"`
	} `json:"language"`
}
