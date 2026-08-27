package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"mm2_client/config"
	"mm2_client/external_services"
	"mm2_client/helpers"
	"mm2_client/mm2_tools_generics"
)

//! Standalone check of the gleeccex price provider: it fetches the Gleec CEX public API
//! and renders the result with the very same logic than GET /api/v2/tickers?expire_at=N
//! (mm2_tools_server/tickers_all_infos.go). None of the other price services is started,
//! so every value shown here can only come from the gleeccex provider.

// Used when no coins_config.json is available next to the binary.
const gFallbackCfg = `{
    "GLEEC": {
        "coin": "GLEEC",
        "type": "GRC-20",
        "name": "Gleec",
        "coinpaprika_id": "gleec-gleec-coin",
        "coingecko_id": "gleec-coin",
        "livecoinwatch_id": "GLEEC",
        "explorer_url": "https://evm-explorer.gleec.com/",
        "active": false,
        "is_testnet": false,
        "currently_enabled": false,
        "wallet_only": false
    },
    "BTC": {
        "coin": "BTC",
        "type": "UTXO",
        "name": "Bitcoin",
        "coinpaprika_id": "btc-bitcoin",
        "coingecko_id": "bitcoin",
        "livecoinwatch_id": "BTC",
        "explorer_url": "https://blockstream.info/",
        "active": true,
        "is_testnet": false,
        "currently_enabled": false,
        "wallet_only": false
    },
    "KMD": {
        "coin": "KMD",
        "type": "Smart Chain",
        "name": "Komodo",
        "coinpaprika_id": "kmd-komodo",
        "coingecko_id": "komodo",
        "livecoinwatch_id": "KMD",
        "explorer_url": "https://kmdexplorer.io/",
        "active": true,
        "is_testnet": false,
        "currently_enabled": false,
        "wallet_only": false
    }
}`

func loadRegistry(path string) {
	if _, err := os.Stat(path); err == nil {
		if config.ParseDesktopRegistryFromFile(path) {
			fmt.Printf("Loaded %d coins from %s\n\n", len(config.GCFGRegistry), path)
			return
		}
		fmt.Printf("Cannot parse %s\n", path)
	}
	if !config.ParseDesktopRegistryFromString(gFallbackCfg) {
		fmt.Println("Cannot load the embedded fallback cfg")
		os.Exit(1)
	}
	fmt.Printf("%s not usable, using the embedded fallback cfg (%d coins)\n\n", path, len(config.GCFGRegistry))
}

func printRawAnswer() {
	fmt.Println("== 1. Raw answer of the gleeccex provider ==")
	if !external_services.UpdateGleecCexRegistry() {
		fmt.Println("FAILED: Gleec CEX didn't answer anything usable")
		os.Exit(1)
	}
	val, ok := external_services.GleecCexRegistry.Load("GLEEC")
	if !ok {
		fmt.Println("FAILED: nothing stored in the gleeccex registry")
		os.Exit(1)
	}
	b, _ := json.MarshalIndent(val, "", "  ")
	fmt.Printf("%s\n\n", b)
}

func printProviderAnswers(coins []string) {
	fmt.Println("== 2. Provider level answers (GLEEC vs the other coins) ==")
	for _, cur := range coins {
		price, priceDate, priceProvider := external_services.GleecCexRetrieveUSDValIfSupported(cur)
		volume, _, volumeProvider := external_services.GleecCexGetTotalVolume(cur)
		change, _, changeProvider := external_services.GleecCexGetChange24h(cur)
		fmt.Printf("%-14s price=%-16s [%s] volume24h=%-18s [%s] change24h=%-12s [%s] last_updated=%s\n",
			cur, price, priceProvider, volume, volumeProvider, change, changeProvider, priceDate)
	}
	fmt.Println()
}

// Same loop than mm2_tools_server.TickerAllInfosV2
func printTickersV2(expireAt int) {
	fmt.Printf("== 3. /api/v2/tickers?expire_at=%d rendering ==\n", expireAt)
	var out = make(map[string]*mm2_tools_generics.TickerInfosAnswer)
	var memoization = make(map[string]bool)
	for _, cur := range config.GCFGRegistry {
		if memoization[helpers.RetrieveMainTicker(cur.Coin)] == false && cur.IsTestNet == false {
			resp := mm2_tools_generics.GetTickerInfos(cur.Coin, expireAt)
			resp.Ticker = helpers.RetrieveMainTicker(resp.Ticker)
			resp.Sparkline7D = nil
			if helpers.AsFloat(resp.LastPrice) > 0 {
				out[resp.Ticker] = resp
				memoization[resp.Ticker] = true
			}
		}
	}
	b, _ := json.MarshalIndent(out, "", "    ")
	fmt.Printf("%s\n\n", b)
	fmt.Printf("%d coin(s) answered out of %d in the cfg (only gleeccex is running here)\n",
		len(out), len(config.GCFGRegistry))
}

func controlCoins() []string {
	out := []string{"GLEEC"}
	for _, cur := range config.GCFGRegistry {
		if helpers.RetrieveMainTicker(cur.Coin) != "GLEEC" {
			out = append(out, cur.Coin)
		}
		if len(out) == 6 {
			break
		}
	}
	sort.Strings(out[1:])
	return out
}

func main() {
	cfgPath := flag.String("cfg", "coins_config.json", "path to coins_config.json")
	expireAt := flag.Int("expire_at", 21600, "price validity in seconds, as in /api/v2/tickers?expire_at=")
	flag.Parse()

	loadRegistry(*cfgPath)
	printRawAnswer()
	printProviderAnswers(controlCoins())
	printTickersV2(*expireAt)
}
