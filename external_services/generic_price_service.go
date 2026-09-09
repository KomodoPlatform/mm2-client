package external_services

import (
	"math"
	"mm2_client/helpers"
	"strconv"
)

// isUsablePrice treats zero and missing prices alike, regardless of formatting.
func isUsablePrice(value string) bool {
	price, err := strconv.ParseFloat(value, 64)
	return err == nil && price > 0 && !math.IsNaN(price) && !math.IsInf(price, 0)
}

func RetrieveUSDValIfSupported(coin string, expirePriceValidity int) (string, string, string) {
	//! Binance
	val, date, _, provider := BinanceRetrieveUSDValIfSupported(coin)

	elapsed := helpers.DateToTimeElapsed(date)
	expirePriceValidityF := float64(expirePriceValidity)

	//! Forex
	if !isUsablePrice(val) || (expirePriceValidity > 0 && elapsed > expirePriceValidityF) {
		val, date, provider = ForexRetrieveUSDValIfSupported(coin)
		if isUsablePrice(val) {
			return val, date, provider
		}
	}

	//! Gecko
	if !isUsablePrice(val) || (expirePriceValidity > 0 && elapsed > expirePriceValidityF) {
		val, date, provider = CoingeckoRetrieveUSDValIfSupported(coin)
		elapsed = helpers.DateToTimeElapsed(date)
	}

	//! Paprika
	if !isUsablePrice(val) || (expirePriceValidity > 0 && elapsed > expirePriceValidityF) {
		val, date, provider = CoinpaprikaRetrieveUSDValIfSupported(coin)
		if !isUsablePrice(val) {
			val, date, provider = CoinpaprikaRetrieveUSDValIfSupported(helpers.RetrieveMainTicker(coin))
		}
		elapsed = helpers.DateToTimeElapsed(date)
	}

	//! LCW
	if !isUsablePrice(val) || (expirePriceValidity > 0 && elapsed > expirePriceValidityF) {
		val, date, provider = LcwRetrieveUSDValIfSupported(coin)
		elapsed = helpers.DateToTimeElapsed(date)
	}

	//! Gleec CEX (GLEEC only)
	if !isUsablePrice(val) || (expirePriceValidity > 0 && elapsed > expirePriceValidityF) {
		val, date, provider = GleecCexRetrieveUSDValIfSupported(coin)
	}

	//! Verification
	if isUsablePrice(val) {
		return val, date, provider
	} else {
		return "0", date, "unknown"
	}
}

func RetrieveCEXRatesFromPair(base string, rel string) (string, bool, string, string) {
	//! Binance
	val, calculated, date, provider := BinanceRetrieveCEXRatesFromPair(base, rel)

	//! LWC
	if !isUsablePrice(val) {
		val, calculated, date, provider = LcwRetrieveCEXRatesFromPair(base, rel)
	}

	//! Gecko
	if !isUsablePrice(val) {
		val, calculated, date, provider = CoingeckoRetrieveCEXRatesFromPair(base, rel)
	}

	//! Paprika
	if !isUsablePrice(val) {
		val, calculated, date, provider = CoinpaprikaRetrieveCEXRatesFromPair(base, rel)
	}

	//! Verification
	if isUsablePrice(val) {
		return val, calculated, date, provider
	} else {
		return "0", calculated, date, "unknown"
	}
}

func RetrieveVolume24h(coin string) (string, string, string) {
	volume, date, provider := CoingeckoGetTotalVolume(coin)
	if volume == "0" {
		volume, date, provider = CoinpaprikaTotalVolume(coin)
	}
	if volume == "0" {
		volume, date, provider = LcwGetTotalVolume(coin)
	}
	if volume == "0" {
		volume, date, provider = GleecCexGetTotalVolume(coin)
	}
	if volume != "0" {
		return volume, date, provider
	} else {
		return volume, date, "unknown"
	}
}

func RetrieveSparkline7D(coin string) (*[]float64, string, string) {
	sparklineData, date, provider := CoingeckoGetSparkline7D(coin)
	if sparklineData == nil {
		return sparklineData, date, "unknown"
	}
	return sparklineData, date, provider
}

func RetrievePercentChange24h(coin string) (string, string, string) {
	_, date, change24h, provider := BinanceRetrieveUSDValIfSupported(coin)

	if change24h == "0" {
		change24h, date, provider = CoingeckoGetChange24h(coin)
	}

	if change24h == "0" {
		change24h, date, provider = CoinpaprikaGetChange24h(coin)
	}

	if change24h == "0" {
		change24h, date, provider = LcwGetChange24h(coin)
	}

	if change24h == "0" {
		change24h, date, provider = GleecCexGetChange24h(coin)
	}

	if change24h != "0" {
		return change24h, date, provider
	} else {
		return change24h, date, "unknown"
	}
}
