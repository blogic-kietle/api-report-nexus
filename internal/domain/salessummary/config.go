package salessummary

import (
	"time"

	"api-report-nexus/internal/domain/report"
)

// POS versions (dates, MM.dd.yyyy) that switched report sections on or off.
const (
	// new average-sales layout
	versionAvgSales = "02.29.2024"
	// labor summary added
	versionLabor = "04.03.2024"
	// configurable day parts
	versionDayPartV2 = "05.14.2024"
	// discounts no longer include cash discount
	versionCoinDiscount = "08.23.2024"
)

type labels struct {
	area, party, auto, customFee, webCustomFee, merchantFee, dineIn, toGo, surcharge string
}

// In surcharge mode the custom fee takes the surcharge's report name.
func readLabels(configs []report.Config) labels {
	l := labels{
		area: "Area Gratuity", party: "Party Gratuity", auto: "Auto Gratuity",
		customFee: "Custom Fee", webCustomFee: "Custom Fee", merchantFee: "Merchant Fee",
		dineIn: "Dine In", toGo: "To Go", surcharge: "Surcharge Fee",
	}
	feeMode := ""
	for _, c := range configs {
		switch c.Name {
		case "SystemConfigValue.GratuityName":
			l.area = c.Value
		case "SystemConfigValue.ServiceChargeName":
			l.party = c.Value
		case "SystemConfigValue.AutoGratuityName":
			l.auto = c.Value
		case "CreditCardConfigValue.SVCFeeLabel":
			l.customFee = c.Value
		case "CreditCardConfigValue.MerchantFeeLabel":
			l.merchantFee = c.Value
		case "CreditCardConfigValue.SvcFeeLabelWeb":
			l.webCustomFee = c.Value
		case "SaleTypesConfig.DineInLabel":
			l.dineIn = c.Value
		case "SaleTypesConfig.ToGoLabel":
			l.toGo = c.Value
		case "CreditCardConfigValue.SVCFeeLabelOnReport":
			if c.Value != "" {
				l.surcharge = c.Value
			}
		case "SystemConfigValue.CreditCardFeeMode":
			feeMode = c.Value
		}
	}
	// CreditCardFeeMode.Surcharge
	if feeMode == "2" {
		l.customFee = l.surcharge
	}
	return l
}

// versionCompare orders MM.dd.yyyy versions; an empty version is the oldest release and an unparseable one compares equal.
func versionCompare(a, b string) int {
	if a == "" {
		a = "01.01.2022"
	}
	ta, errA := time.Parse("01.02.2006", a)
	tb, errB := time.Parse("01.02.2006", b)
	if errA != nil || errB != nil {
		return 0
	}
	switch {
	case ta.Before(tb):
		return -1
	case ta.After(tb):
		return 1
	}
	return 0
}

func versionGTE(v, floor string) bool { return versionCompare(v, floor) >= 0 }
func versionLT(v, floor string) bool  { return versionCompare(v, floor) < 0 }
