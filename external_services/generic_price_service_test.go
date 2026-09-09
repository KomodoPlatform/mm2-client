package external_services

import (
	"encoding/json"
	"math"
	"mm2_client/config"
	"mm2_client/helpers"
	"sync"
	"testing"
	"time"
)

func isolatePriceRegistries(t *testing.T) string {
	t.Helper()
	original := config.GCFGRegistry
	config.GCFGRegistry = map[string]*config.DesktopCFG{
		"GLEEC": {Coin: "GLEEC", CoingeckoID: "gleec-coin", LcwId: "GLEEC"},
		"REL":   {Coin: "REL", CoingeckoID: "rel", LcwId: "REL"},
	}
	t.Cleanup(func() { config.GCFGRegistry = original })
	for _, registry := range []*sync.Map{&BinancePriceRegistry, &ForexPriceRegistry, &CoingeckoPriceRegistry, &CoinpaprikaRegistry, &LcwPriceRegistry, &GleecCexRegistry} {
		r := registry
		saved := map[interface{}]interface{}{}
		r.Range(func(k, v interface{}) bool { saved[k] = v; r.Delete(k); return true })
		t.Cleanup(func() {
			r.Range(func(k, v interface{}) bool { r.Delete(k); return true })
			for k, v := range saved {
				r.Store(k, v)
			}
		})
	}
	return helpers.GetDateFromTimestampStandard(time.Now().UnixNano())
}

func TestIsUsablePrice(t *testing.T) {
	for _, s := range []string{"0", "0.0000000000", "-0.000000", "0e10", "", "null", "invalid", "-1", "NaN", "+Inf", "-Inf", "1e999"} {
		if isUsablePrice(s) {
			t.Errorf("accepted invalid price %q", s)
		}
	}
	for _, s := range []string{"0.0490160000", "1", "1e-12"} {
		if !isUsablePrice(s) {
			t.Errorf("rejected positive price %q", s)
		}
	}
}

func TestUSDPriceFallbackThroughEveryProvider(t *testing.T) {
	date := isolatePriceRegistries(t)
	forexID := "GLEEC"
	config.GCFGRegistry["GLEEC"].ForexId = &forexID
	BinancePriceRegistry.Store("GLEECUSDT", []string{"0.00000000", date, "-1"})
	ForexPriceRegistry.Store("Forex", &ForexAnswer{Rates: map[string]float64{"GLEEC": 0}})
	CoingeckoPriceRegistry.Store("gleec-coin", CoingeckoAnswer{CurrentPrice: 0, LastUpdated: date})
	paprika := CoinpaprikaAnswer{LastUpdated: date}
	CoinpaprikaRegistry.Store("GLEEC", paprika)
	var lcw LcwAnswer
	if err := json.Unmarshal([]byte(`{"code":"GLEEC","rate":null,"delta":null}`), &lcw); err != nil {
		t.Fatal(err)
	}
	LcwPriceRegistry.Store("GLEEC", lcw)
	GleecCexRegistry.Store("GLEEC", &GleecCexAnswer{Price: "0.0490000000", PriceDate: date})
	assertProvider := func(want string) {
		t.Helper()
		for _, expiry := range []int{0, 21600} {
			v, _, p := RetrieveUSDValIfSupported("GLEEC", expiry)
			if p != want || !isUsablePrice(v) {
				t.Fatalf("expiry %d: got %q/%s, want %s", expiry, v, p, want)
			}
		}
	}
	assertProvider("gleeccex")
	lcw.Rate = .05
	LcwPriceRegistry.Store("GLEEC", lcw)
	assertProvider("livecoinwatch")
	paprika.Quotes.USD.Price = .06
	CoinpaprikaRegistry.Store("GLEEC", paprika)
	assertProvider("coinpaprika")
	CoingeckoPriceRegistry.Store("gleec-coin", CoingeckoAnswer{CurrentPrice: .07, LastUpdated: date})
	assertProvider("coingecko")
	ForexPriceRegistry.Store("Forex", &ForexAnswer{Rates: map[string]float64{"GLEEC": 10}, Timestamp: time.Now().Unix()})
	assertProvider("forex")
	BinancePriceRegistry.Store("GLEECUSDT", []string{"0.09", date, "-1"})
	assertProvider("binance")
}

func TestUnavailableUSDPriceIsCanonical(t *testing.T) {
	date := isolatePriceRegistries(t)
	LcwPriceRegistry.Store("GLEEC", LcwAnswer{Rate: 0})
	for _, price := range []string{"0.0000000000", "", "-1", "NaN", "+Inf"} {
		GleecCexRegistry.Store("GLEEC", &GleecCexAnswer{Price: price, PriceDate: date})
		v, _, p := RetrieveUSDValIfSupported("GLEEC", 0)
		if v != "0" || p != "unknown" {
			t.Errorf("price %q: got %q/%s", price, v, p)
		}
	}
}

func TestPairPriceFallbackThroughProviders(t *testing.T) {
	date := isolatePriceRegistries(t)
	// A missing numerator divided by a valid denominator yields a formatted zero.
	BinancePriceRegistry.Store("RELUSDT", []string{"2", date, "0"})
	LcwPriceRegistry.Store("REL", LcwAnswer{Rate: 2})
	CoingeckoPriceRegistry.Store("rel", CoingeckoAnswer{CurrentPrice: 2, LastUpdated: date})
	p := CoinpaprikaAnswer{LastUpdated: date}
	p.Quotes.USD.Price = 2
	CoinpaprikaRegistry.Store("REL", p)
	v, _, _, provider := RetrieveCEXRatesFromPair("GLEEC", "REL")
	if v != "0" || provider != "unknown" {
		t.Fatalf("missing pair: %q/%s", v, provider)
	}
	p.Quotes.USD.Price = 1
	CoinpaprikaRegistry.Store("GLEEC", p)
	check := func(want string) {
		t.Helper()
		v, _, _, provider := RetrieveCEXRatesFromPair("GLEEC", "REL")
		if !isUsablePrice(v) || provider != want {
			t.Fatalf("got %q/%s, want %s", v, provider, want)
		}
	}
	check("coinpaprika")
	CoingeckoPriceRegistry.Store("gleec-coin", CoingeckoAnswer{CurrentPrice: 1, LastUpdated: date})
	check("coingecko")
	LcwPriceRegistry.Store("GLEEC", LcwAnswer{Rate: 1})
	check("livecoinwatch")
	BinancePriceRegistry.Store("GLEECUSDT", []string{"1", date, "0"})
	check("binance")
}

func TestNullableChangeFields(t *testing.T) {
	isolatePriceRegistries(t)
	LcwPriceRegistry.Store("GLEEC", LcwAnswer{})
	if v, _, _ := LcwGetChange24h("GLEEC"); v != "0" {
		t.Errorf("missing delta: %s", v)
	}
	change := -2.5
	CoingeckoPriceRegistry.Store("gleec-coin", CoingeckoAnswer{PriceChangePercentage24H: &change})
	if v, _, _ := CoingeckoGetChange24h("GLEEC"); v != "0" {
		t.Errorf("missing currency change: %s", v)
	}
	CoingeckoPriceRegistry.Store("gleec-coin", CoingeckoAnswer{PriceChangePercentage24HInCurrency: &change})
	if v, _, _ := CoingeckoGetChange24h("GLEEC"); math.Abs(helpers.AsFloat(v)-change) > 1e-9 {
		t.Errorf("negative change: %s", v)
	}
}
