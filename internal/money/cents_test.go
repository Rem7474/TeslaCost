package money

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestParseAndFormat(t *testing.T) {
	cases := map[string]Cents{
		"12.3": 1230, "0.1": 10, "0.2": 20, "999.99": 99999, "-4.5": -450,
		"1e2": 10000, "12.345": 1235, "-12.345": -1235, "0.004": 0, "33.333333": 3333,
	}
	for in, want := range cases {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	if Cents(-5).String() != "-0.05" || Cents(123456).String() != "1234.56" {
		t.Errorf("unexpected formatting: %s %s", Cents(-5), Cents(123456))
	}
}

func TestJSONRoundTripIsExact(t *testing.T) {
	var v struct {
		A Cents  `json:"a"`
		B *Cents `json:"b"`
		C *Cents `json:"c"`
	}
	// 0.1 + 0.2 style values that are inexact in float64
	if err := json.Unmarshal([]byte(`{"a": 0.30, "b": "19.99", "c": null}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.A != 30 || v.B == nil || *v.B != 1999 || v.C != nil {
		t.Fatalf("unexpected decode: %+v", v)
	}
	out, _ := json.Marshal(v)
	if string(out) != `{"a":0.30,"b":19.99,"c":null}` {
		t.Fatalf("unexpected encode: %s", out)
	}
}

func TestSplitAddsUp(t *testing.T) {
	parts := Split(99999, 4)
	var sum Cents
	for _, p := range parts {
		sum += p
	}
	if sum != 99999 || parts[0] != 24999 || parts[3] != 25002 {
		t.Fatalf("unexpected split: %v", parts)
	}
}

func TestNumericConversion(t *testing.T) {
	var c Cents
	// 1234.5678 stored with exponent -4 → rounded to the cent
	if err := c.ScanNumeric(pgtype.Numeric{Int: big.NewInt(12345678), Exp: -4, Valid: true}); err != nil || c != 123457 {
		t.Fatalf("ScanNumeric = %d, %v", c, err)
	}
	if err := c.ScanNumeric(pgtype.Numeric{Int: big.NewInt(12), Exp: 2, Valid: true}); err != nil || c != 120000 {
		t.Fatalf("ScanNumeric with positive exponent = %d, %v", c, err)
	}
	n, _ := Cents(-1999).NumericValue()
	if n.Int.Int64() != -1999 || n.Exp != -2 {
		t.Fatalf("NumericValue = %+v", n)
	}
}
