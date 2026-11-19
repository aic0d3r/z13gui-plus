// Package cssscale multiplies the pixel lengths in a CSS sheet. The gamescope
// backend uses it to derive its scaled sheet from layout.css, so the two can
// never drift apart.
package cssscale

import (
	"math"
	"regexp"
	"strconv"
)

var pxLength = regexp.MustCompile(`(-?\d*\.?\d+)px\b`)

// Px returns css with every "<number>px" length multiplied by s and rounded
// to two decimals.
func Px(css string, s float64) string {
	return pxLength.ReplaceAllStringFunc(css, func(m string) string {
		v, err := strconv.ParseFloat(m[:len(m)-2], 64)
		if err != nil {
			return m
		}
		return strconv.FormatFloat(math.Round(v*s*100)/100, 'f', -1, 64) + "px"
	})
}
