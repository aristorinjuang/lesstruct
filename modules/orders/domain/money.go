package orders

import (
	"strconv"
	"strings"
)

// zeroDecimalCurrencies lists the zero-decimal currencies Lesstruct supports:
// the smallest denomination is the unit itself, so the ledger's integer is
// already the whole amount: "IDR 598000" is 598,000 rupiah, never 5,980.00.
// Every other supported currency is treated as a two-decimal currency; the
// ledger has no three-decimal (ISO 4217 exponent 3) handling. The storefront's
// money formatter and the Midtrans driver follow the same convention.
var zeroDecimalCurrencies = map[string]bool{
	"IDR": true,
	"JPY": true,
	"KRW": true,
	"VND": true,
}

// groupThousands splits a digits-only string into comma groups of three.
func groupThousands(digits string) string {
	var out strings.Builder
	for i, digit := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(digit)
	}
	return out.String()
}

// Money is one monetary amount in minor units paired with its currency code:
// the domain's value representation of money. The ledger stores integers —
// never floats — and every rendered amount comes from Format, so an amount
// can never drift from the currency it is quoted in.
type Money struct {
	Amount   int64
	Currency string
}

// Format renders the amount for display next to its currency code:
// "IDR 598,000" for zero-decimal currencies and "USD 59.80" for the rest.
// The emails and the invoice PDF carry these display strings; the ledger
// itself stays integer, so every amount renders from one rule.
func (m Money) Format() string {
	rendered := strconv.FormatInt(m.Amount, 10)
	negative := strings.HasPrefix(rendered, "-")
	digits := strings.TrimPrefix(rendered, "-")
	value := groupThousands(digits)
	if !zeroDecimalCurrencies[strings.ToUpper(strings.TrimSpace(m.Currency))] {
		if len(digits) < 3 {
			digits = strings.Repeat("0", 3-len(digits)) + digits
		}
		value = groupThousands(digits[:len(digits)-2]) + "." + digits[len(digits)-2:]
	}
	if negative {
		value = "-" + value
	}
	return m.Currency + " " + value
}

// NewMoney pairs an integer-ledger amount with its currency code.
func NewMoney(amount int64, currency string) Money {
	return Money{
		Amount:   amount,
		Currency: currency,
	}
}
