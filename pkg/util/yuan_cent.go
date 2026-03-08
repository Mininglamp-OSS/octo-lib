package util

import "fmt"

// YuanToCent 元转分
func YuanToCent(yuan float64) int64 {
	dec, err := NewFromString(fmt.Sprintf("%0.2f", yuan))
	MustNoErr(err)
	m, err := NewFromString("100")
	MustNoErr(err)

	return dec.Mul(m).IntPart()
}

// CentToYuan 分转元
func CentToYuan(cent int64) float64 {
	centDec, err := NewFromString(fmt.Sprintf("%d", cent))
	MustNoErr(err)
	mDec, err := NewFromString(fmt.Sprintf("%d", 100))
	MustNoErr(err)

	result, _ := centDec.Div(mDec).Round(2).Float64()
	MustNoErr(err)
	return result
}
