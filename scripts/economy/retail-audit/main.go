// Command retail-audit reports catalogue provider floors in nominal star units.
// It is offline; it neither submits jobs nor predicts real settlement or profit.
package main

import (
	"encoding/csv"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strconv"

	"vk-ai-aggregator/internal/service/pricingcatalog"
)

func main() {
	prices := pricingcatalog.StaticProductPrices()
	sort.Slice(prices, func(i, j int) bool { return fmt.Sprint(prices[i].Key) < fmt.Sprint(prices[j].Key) })
	w := csv.NewWriter(os.Stdout)
	if err := w.Write([]string{"model", "quality", "resolution", "seconds", "retail_stars", "provider_floor_nominal_stars", "below_nominal_floor"}); err != nil {
		panic(err)
	}
	for _, p := range prices {
		if p.RetailPolicyVersion == 0 {
			continue
		}
		retail, err := p.InternalCredits()
		if err != nil {
			panic(err)
		}
		n := new(big.Int).Mul(big.NewInt(p.Floor.Amount), big.NewInt(p.UnitConversion.InternalCreditUnits))
		d := new(big.Int).Mul(big.NewInt(p.UnitConversion.FloorUnits), big.NewInt(pricingcatalog.MinorUnitsPerCredit))
		floor := new(big.Rat).SetFrac(n, d)
		model := p.Key.ImageModelID
		if model == "" {
			model = p.Key.TextModelID
		}
		if model == "" {
			model = p.Key.AudioModelID
		}
		if model == "" {
			model = string(p.Key.VideoRouteAlias)
		}
		row := []string{model, p.Key.Quality, p.Key.Resolution, strconv.Itoa(p.Key.DurationSec), strconv.FormatInt(retail, 10), floor.FloatString(6), strconv.FormatBool(floor.Cmp(new(big.Rat).SetInt64(retail)) > 0)}
		if err := w.Write(row); err != nil {
			panic(err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		panic(err)
	}
}
