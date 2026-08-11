package external_services

import (
	"mm2_client/helpers"
	"testing"
)

func TestGleecCexOnlyServesGleec(t *testing.T) {
	GleecCexRegistry.Store(gGleecCexSupportedTicker, &GleecCexAnswer{
		Price: "0.0500000000", PriceDate: "2026-08-11T14:00:00Z",
		Volume24h: "1234.0000000000", VolumeDate: "2026-08-11T14:00:00Z",
		Change24h: "1.500000", ChangeDate: "2026-08-11T14:00:00Z"})

	price, date, provider := GleecCexRetrieveUSDValIfSupported("GLEEC")
	if price != "0.0500000000" || provider != gGleecCexProvider || date != "2026-08-11T14:00:00Z" {
		t.Errorf("unexpected GLEEC price answer: %s %s %s", price, date, provider)
	}
	if volume, _, volumeProvider := GleecCexGetTotalVolume("GLEEC"); volume != "1234.0000000000" || volumeProvider != gGleecCexProvider {
		t.Errorf("unexpected GLEEC volume answer: %s %s", volume, volumeProvider)
	}
	if change, _, changeProvider := GleecCexGetChange24h("GLEEC"); change != "1.500000" || changeProvider != gGleecCexProvider {
		t.Errorf("unexpected GLEEC change answer: %s %s", change, changeProvider)
	}

	for _, cur := range []string{"BTC", "KMD", "GLEEC-OLD", "GLEECT", "GLEEC-ERC20", "gleec"} {
		if price, _, provider := GleecCexRetrieveUSDValIfSupported(cur); price != "0" || provider != "unknown" {
			t.Errorf("%s should not be served by gleeccex, got %s %s", cur, price, provider)
		}
		if volume, _, provider := GleecCexGetTotalVolume(cur); volume != "0" || provider != "unknown" {
			t.Errorf("%s should not be served by gleeccex, got %s %s", cur, volume, provider)
		}
		if change, _, provider := GleecCexGetChange24h(cur); change != "0" || provider != "unknown" {
			t.Errorf("%s should not be served by gleeccex, got %s %s", cur, change, provider)
		}
	}
}

func TestGleecCexComputeVolume(t *testing.T) {
	tickers := map[string]GleecCexTicker{
		gGleecCexUsdtMarket: {VolumeQuote: "150.5", Timestamp: "2026-08-11T14:00:00.000Z"},
		gGleecCexBtcMarket:  {VolumeQuote: "0.001", Timestamp: "2026-08-11T14:00:00.000Z"},
	}
	//! 150.5 USDT + 0.001 BTC * 60000 USD = 210.5 USD
	volume, date := gleecCexComputeVolume(tickers, 60000.0)
	if helpers.AsFloat(volume) != 210.5 {
		t.Errorf("expected an aggregated volume of 210.5, got %s", volume)
	}
	if date != "2026-08-11T14:00:00Z" {
		t.Errorf("expected the market timestamp, got %s", date)
	}

	//! No trade over the last 24h
	if volume, _ := gleecCexComputeVolume(map[string]GleecCexTicker{}, 60000.0); helpers.AsFloat(volume) != 0 {
		t.Errorf("expected a null volume, got %s", volume)
	}
}

func TestGleecCexComputeChange24h(t *testing.T) {
	change, date := gleecCexComputeChange24h(map[string]GleecCexTicker{
		gGleecCexUsdtMarket: {Open: "0.05", Last: "0.055", Timestamp: "2026-08-11T14:00:00.000Z"}})
	if helpers.AsFloat(change) != 10 {
		t.Errorf("expected a 10%% change, got %s", change)
	}
	if date != "2026-08-11T14:00:00Z" {
		t.Errorf("expected the market timestamp, got %s", date)
	}

	//! An empty market cannot give any change
	if change, _ := gleecCexComputeChange24h(map[string]GleecCexTicker{
		gGleecCexUsdtMarket: {Open: "0", Last: "0"}}); change != "0" {
		t.Errorf("expected a null change, got %s", change)
	}
}
