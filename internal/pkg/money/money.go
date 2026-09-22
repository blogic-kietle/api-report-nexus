package money

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

func USD(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "$0.00"
	}
	r, ok := new(big.Rat).SetString(strconv.FormatFloat(v, 'f', -1, 64))
	if !ok {
		return "$0.00"
	}
	s := r.FloatString(2)
	sign := ""
	if s, ok = strings.CutPrefix(s, "-"); ok {
		sign = "-"
	}
	whole, frac, _ := strings.Cut(s, ".")
	return sign + "$" + group(whole) + "." + frac
}

func group(s string) string {
	var b strings.Builder
	for i, d := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(d)
	}
	return b.String()
}
