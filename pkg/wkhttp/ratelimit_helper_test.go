package wkhttp

import (
	"testing"
)

func TestParseRPSFromEnv(t *testing.T) {
	cases := []struct {
		name string
		val  string
		set  bool
		want float64
	}{
		{"unset", "", false, 1.5},
		{"empty", "", true, 1.5},
		{"valid", "3.0", true, 3.0},
		{"negative", "-1", true, 1.5},
		{"zero", "0", true, 1.5},
		{"garbage", "abc", true, 1.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := "WKHTTP_TEST_RPS"
			if tc.set {
				t.Setenv(key, tc.val)
			}
			got := ParseRPSFromEnv(key, 1.5)
			if got != tc.want {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestParseBurstFromEnv(t *testing.T) {
	cases := []struct {
		name string
		val  string
		set  bool
		want int
	}{
		{"unset", "", false, 10},
		{"valid", "25", true, 25},
		{"negative", "-3", true, 10},
		{"zero", "0", true, 10},
		{"garbage", "x", true, 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := "WKHTTP_TEST_BURST"
			if tc.set {
				t.Setenv(key, tc.val)
			}
			got := ParseBurstFromEnv(key, 10)
			if got != tc.want {
				t.Errorf("got %v want %v", got, tc.want)
			}
		})
	}
}
