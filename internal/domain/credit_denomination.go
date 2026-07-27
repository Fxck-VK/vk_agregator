package domain

import (
	"errors"
	"fmt"
	"math"
)

const (
	// LegacyCreditDenominationVersion is the original one-credit-per-ruble
	// ledger denomination.
	LegacyCreditDenominationVersion = 1
	// CurrentCreditDenominationVersion is the public star denomination.
	CurrentCreditDenominationVersion = 2
	// StarKopecks fixes the public exchange rate without floating-point math.
	StarKopecks int64 = 50
	// StarsPerLegacyCredit preserves the ruble value of legacy balances.
	StarsPerLegacyCredit int64 = 2
)

var ErrInvalidCreditDenomination = errors.New("invalid credit denomination conversion")

// NormalizeCreditDenominationVersion treats missing legacy snapshots as v1.
func NormalizeCreditDenominationVersion(version int) int {
	if version == 0 {
		return LegacyCreditDenominationVersion
	}
	return version
}

// ConvertCreditAmount converts an exact integer amount between supported
// denominations. Lossy and overflowing conversions fail closed.
func ConvertCreditAmount(amount int64, fromVersion, toVersion int) (int64, error) {
	fromVersion = NormalizeCreditDenominationVersion(fromVersion)
	toVersion = NormalizeCreditDenominationVersion(toVersion)
	if fromVersion == toVersion {
		if !supportedCreditDenomination(fromVersion) {
			return 0, fmt.Errorf("%w: version %d", ErrInvalidCreditDenomination, fromVersion)
		}
		return amount, nil
	}
	switch {
	case fromVersion == LegacyCreditDenominationVersion && toVersion == CurrentCreditDenominationVersion:
		if amount > math.MaxInt64/StarsPerLegacyCredit || amount < math.MinInt64/StarsPerLegacyCredit {
			return 0, fmt.Errorf("%w: amount overflow", ErrInvalidCreditDenomination)
		}
		return amount * StarsPerLegacyCredit, nil
	case fromVersion == CurrentCreditDenominationVersion && toVersion == LegacyCreditDenominationVersion:
		if amount%StarsPerLegacyCredit != 0 {
			return 0, fmt.Errorf("%w: lossy conversion", ErrInvalidCreditDenomination)
		}
		return amount / StarsPerLegacyCredit, nil
	default:
		return 0, fmt.Errorf(
			"%w: version %d to %d",
			ErrInvalidCreditDenomination,
			fromVersion,
			toVersion,
		)
	}
}

// CurrentCreditAmount returns an amount expressed in the current public stars.
func CurrentCreditAmount(amount int64, version int) (int64, error) {
	return ConvertCreditAmount(amount, version, CurrentCreditDenominationVersion)
}

func supportedCreditDenomination(version int) bool {
	return version == LegacyCreditDenominationVersion || version == CurrentCreditDenominationVersion
}
