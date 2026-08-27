package external_services

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mm2_client/constants"
	"mm2_client/helpers"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kpango/glg"
)

//! Gleec CEX public API (HitBTC v3 compatible) - https://api.exchange.gleec.com/#prices
//! This provider is voluntarily restricted to the GLEEC coin, every other ticker is
//! answered as "not found" ("0" / "unknown") so the generic price service can fallback.

const gGleecCexEndpoint = "https://api.exchange.gleec.com/api/3/public"
const gGleecCexSupportedTicker = "GLEEC"
const gGleecCexProvider = "gleeccex"

// Markets used to compute the aggregated 24h volume of GLEEC
const (
	gGleecCexUsdtMarket = "GLEECUSDT" //! GLEEC is the base, volume_quote is in USDT
	gGleecCexBtcMarket  = "GLEECBTC"  //! GLEEC is the base, volume_quote is in BTC
)

type GleecCexRate struct {
	Currency  string `json:"currency"`
	Price     string `json:"price"`
	Timestamp string `json:"timestamp"`
}

type GleecCexTicker struct {
	Ask         string `json:"ask"`
	Bid         string `json:"bid"`
	Last        string `json:"last"`
	Low         string `json:"low"`
	High        string `json:"high"`
	Open        string `json:"open"`
	Volume      string `json:"volume"`
	VolumeQuote string `json:"volume_quote"`
	Timestamp   string `json:"timestamp"`
}

type GleecCexAnswer struct {
	Price      string `json:"price"`
	PriceDate  string `json:"price_date"`
	Volume24h  string `json:"volume_24h"`
	VolumeDate string `json:"volume_date"`
	Change24h  string `json:"change_24h"`
	ChangeDate string `json:"change_date"`
}

var GleecCexRegistry sync.Map

func gleecCexNow() string {
	return helpers.GetDateFromTimestampStandard(time.Now().UnixNano())
}

// The exchange answers with milliseconds (2006-01-02T15:04:05.000Z), normalize it
// to the same RFC3339 layout than the other providers.
func gleecCexDate(date string) string {
	parsed, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return gleecCexNow()
	}
	return helpers.GetDateFromTime(parsed)
}

func gleecCexGet(url string, out interface{}) error {
	client := &http.Client{Timeout: time.Second * 20}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("http status not OK: %s", bodyBytes)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// GET /api/3/public/price/rate?from=GLEEC,BTC&to=USDT
func processGleecCexRates() map[string]GleecCexRate {
	out := make(map[string]GleecCexRate)
	url := gGleecCexEndpoint + "/price/rate?from=" + gGleecCexSupportedTicker + ",BTC&to=USDT"
	_ = glg.Infof("Processing gleeccex request: %s", url)
	if err := gleecCexGet(url, &out); err != nil {
		_ = glg.Errorf("gleeccex rate request failed: %v", err)
		return nil
	}
	return out
}

// GET /api/3/public/ticker?symbols=GLEECUSDT,GLEECBTC
func processGleecCexTickers() map[string]GleecCexTicker {
	out := make(map[string]GleecCexTicker)
	symbols := strings.Join([]string{gGleecCexUsdtMarket, gGleecCexBtcMarket}, ",")
	url := gGleecCexEndpoint + "/ticker?symbols=" + symbols
	_ = glg.Infof("Processing gleeccex request: %s", url)
	if err := gleecCexGet(url, &out); err != nil {
		_ = glg.Errorf("gleeccex ticker request failed: %v", err)
		return nil
	}
	return out
}

// 24h volume of GLEEC expressed in USD: every GLEEC market quote volume is
// converted to USD (USDT is considered as USD, as everywhere else in this client).
func gleecCexComputeVolume(tickers map[string]GleecCexTicker, btcUsd float64) (string, string) {
	volume := 0.0
	date := ""
	functorAppend := func(market string, usdRate float64) {
		cur, ok := tickers[market]
		if !ok {
			return
		}
		volume += helpers.AsFloat(cur.VolumeQuote) * usdRate
		if date == "" {
			date = gleecCexDate(cur.Timestamp)
		}
	}
	functorAppend(gGleecCexUsdtMarket, 1.0)
	functorAppend(gGleecCexBtcMarket, btcUsd)
	if date == "" {
		date = gleecCexNow()
	}
	return fmt.Sprintf("%.10f", volume), date
}

// 24h change of GLEEC, computed from the rolling 24h open/last of the GLEEC/USDT market.
func gleecCexComputeChange24h(tickers map[string]GleecCexTicker) (string, string) {
	if cur, ok := tickers[gGleecCexUsdtMarket]; ok {
		open := helpers.AsFloat(cur.Open)
		last := helpers.AsFloat(cur.Last)
		if open > 0 && last > 0 {
			return fmt.Sprintf("%f", (last-open)/open*100.0), gleecCexDate(cur.Timestamp)
		}
	}
	return "0", gleecCexNow()
}

func processGleecCex() *GleecCexAnswer {
	rates := processGleecCexRates()
	tickers := processGleecCexTickers()
	if rates == nil && tickers == nil {
		return nil
	}

	answer := &GleecCexAnswer{Price: "0", PriceDate: gleecCexNow(),
		Volume24h: "0", VolumeDate: gleecCexNow(),
		Change24h: "0", ChangeDate: gleecCexNow()}

	//! Price - price/rate first, last traded price of the GLEEC/USDT market as a fallback
	if rate, ok := rates[gGleecCexSupportedTicker]; ok && helpers.AsFloat(rate.Price) > 0 {
		answer.Price = fmt.Sprintf("%.10f", helpers.AsFloat(rate.Price))
		answer.PriceDate = gleecCexDate(rate.Timestamp)
	} else if cur, curOk := tickers[gGleecCexUsdtMarket]; curOk && helpers.AsFloat(cur.Last) > 0 {
		answer.Price = fmt.Sprintf("%.10f", helpers.AsFloat(cur.Last))
		answer.PriceDate = gleecCexDate(cur.Timestamp)
	}

	if helpers.AsFloat(answer.Price) <= 0 {
		//! Without a usable price the whole answer is worthless, keep the previous one
		return nil
	}

	if tickers != nil {
		answer.Volume24h, answer.VolumeDate = gleecCexComputeVolume(tickers,
			helpers.AsFloat(rates["BTC"].Price))
		answer.Change24h, answer.ChangeDate = gleecCexComputeChange24h(tickers)
	}
	return answer
}

// Refresh the registry once, returns false when the exchange couldn't be reached.
func UpdateGleecCexRegistry() bool {
	if resp := processGleecCex(); resp != nil {
		GleecCexRegistry.Store(gGleecCexSupportedTicker, resp)
		return true
	}
	return false
}

func StartGleecCexService() {
	for {
		if UpdateGleecCexRegistry() {
			glg.Info("Gleec CEX request successfully processed")
		} else {
			glg.Error("Something went wrong when processing Gleec CEX request")
		}
		time.Sleep(constants.GPricesLoopTime)
	}
}

// Only the GLEEC ticker of coins_config.json is served here, every other coin
// (GLEEC-OLD, GLEECT, ...) is reported as not found so the caller can fallback.
func gleecCexRetrieve(coin string) (*GleecCexAnswer, bool) {
	if coin != gGleecCexSupportedTicker {
		return nil, false
	}
	val, ok := GleecCexRegistry.Load(gGleecCexSupportedTicker)
	if !ok {
		return nil, true
	}
	return val.(*GleecCexAnswer), true
}

func GleecCexRetrieveUSDValIfSupported(coin string) (string, string, string) {
	if answer, supported := gleecCexRetrieve(coin); supported {
		if answer != nil {
			return answer.Price, answer.PriceDate, gGleecCexProvider
		}
		return "0", gleecCexNow(), gGleecCexProvider
	}
	return "0", gleecCexNow(), "unknown"
}

func GleecCexGetTotalVolume(coin string) (string, string, string) {
	if answer, supported := gleecCexRetrieve(coin); supported {
		if answer != nil {
			return answer.Volume24h, answer.VolumeDate, gGleecCexProvider
		}
		return "0", gleecCexNow(), gGleecCexProvider
	}
	return "0", gleecCexNow(), "unknown"
}

func GleecCexGetChange24h(coin string) (string, string, string) {
	if answer, supported := gleecCexRetrieve(coin); supported {
		if answer != nil {
			return answer.Change24h, answer.ChangeDate, gGleecCexProvider
		}
		return "0", gleecCexNow(), gGleecCexProvider
	}
	return "0", gleecCexNow(), "unknown"
}
