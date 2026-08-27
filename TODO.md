# TODO

## Price fallback chains compare prices as strings against `"0"`

**Where:** `external_services/generic_price_service.go`, the whole file.

- `RetrieveUSDValIfSupported` (lines 15, 17, 24, 30, 32, 39, 45, 50)
- `RetrieveVolume24h` (lines 86, 89, 92, 95)
- `RetrievePercentChange24h` (lines 113, 117, 121, 125, 129)

**Problem:** every fallback step decides whether the current provider answered
with `val == "0"` / `val != "0"`, i.e. a *string* comparison against the literal
`"0"`. The providers however format their values with `fmt.Sprintf("%.10f", ...)`
or `fmt.Sprintf("%f", ...)`, so a zero value reaches the chain as
`"0.0000000000"` or `"0.000000"`, which is not equal to `"0"`. The chain then
treats that zero as a valid answer, stops there, and reports the provider name
instead of falling through to the next source.

Providers returning a formatted zero (the literal `"0"` is only returned when the
coin is missing from the config or from the provider cache):

| Function | Zero value returned |
| --- | --- |
| `LcwRetrieveUSDValIfSupported` (`lcw_service.go:127`) | `"0.0000000000"` |
| `LcwGetTotalVolume` (`lcw_service.go:148`) | `"0.0000000000"` |
| `LcwGetChange24h` (`lcw_service.go:162`) | `"0.0000000000"` |
| `CoingeckoRetrieveUSDValIfSupported` (`coingecko_service.go:126`) | `"0.0000000000"` |
| `CoingeckoGetTotalVolume` (`coingecko_service.go:154`) | `"0.0000000000"` |
| `CoingeckoGetChange24h` (`coingecko_service.go:187`) | `"0.000000"` |
| `CoinpaprikaRetrieveUSDValIfSupported` (`paprika_service.go:114`) | `"0.0000000000"` |
| `CoinpaprikaTotalVolume` (`paprika_service.go:140`) | `"0.0000000000"` |
| `CoinpaprikaGetChange24h` (`paprika_service.go:156`) | `"0.000000"` |

**Observed in production** (`GET https://prices.gleec.com/api/v2/tickers?expire_at=21600`,
2026-08-11):

```json
"GLEEC": {
    "last_price": "0.0274861129", "price_provider": "livecoinwatch",
    "volume24h": "0.0000000000", "volume_provider": "livecoinwatch",
    "change_24h": "0.0000000000", "change_24h_provider": "livecoinwatch"
}
```

Coingecko and Coinpaprika have no GLEEC entry cached, so they returned the
literal `"0"` and the chain moved on. LiveCoinWatch does have the coin, formatted
its null volume/change as `"0.0000000000"`, and the chain accepted it: the
remaining providers (including `gleeccex`, which does have GLEEC data) were never
queried, and the answer advertises `livecoinwatch` where it should have been
`unknown`.

**Fix:** compare numerically in `generic_price_service.go` (`helpers` is already
imported there):

```go
if helpers.AsFloat(volume) == 0 { ... }   // instead of volume == "0"
if helpers.AsFloat(volume) > 0 { ... }    // instead of volume != "0"
```

**Impact to review before applying:** this is not GLEEC specific. Every coin
whose current provider answers with a formatted zero will now fall through to the
next provider, so some tickers will start reporting a real value from a later
source, and others will end up with `"0"` / `"unknown"` instead of
`"0.0000000000"` / a provider name. For `/api/v2/tickers` the price change also
matters: coins whose price is a formatted zero are currently kept out of the
answer anyway by the `AsFloat(resp.LastPrice) > 0` filter in
`mm2_tools_server/tickers_all_infos.go:96`, but they may now get a price from a
later provider and appear in the output.
