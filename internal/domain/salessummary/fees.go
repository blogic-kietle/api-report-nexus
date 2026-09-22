package salessummary

import (
	"maps"
	"slices"

	"api-report-nexus/internal/domain/report"
)

// Card, cash and online sections stay maps: Node re-emitted every input key with derived totals added.

func num(r report.Row, key string) float64 {
	switch v := r[key].(type) {
	case float64:
		return v
	case bool:
		if v {
			return 1
		}
	}
	return 0
}

func third(r report.Row, key string) float64 {
	nested, _ := r["thirdParty"].(map[string]any)
	f, _ := nested[key].(float64)
	return f
}

// merchant adds the totals a card or cash group needs and flips the merchant fee negative (parseMerchantFeeSectionFn).
func merchant(src report.Row) report.Row {
	if src == nil {
		return nil
	}
	r := report.Row{}
	maps.Copy(r, src)
	fees := num(r, "serviceFee") + num(r, "areaFee") + num(r, "partyServiceFee") + num(r, "autoGratuityFee") + num(r, "deliveryFee")
	if num(r, "serviceFee") != 0 || num(r, "areaFee") != 0 || num(r, "partyServiceFee") != 0 ||
		num(r, "autoGratuityFee") != 0 || num(r, "deliveryFee") != 0 {
		r["salesServiceFee"] = num(r, "totalSales") + fees
	}
	r["totalSalesTip"] = num(r, "totalSales") + fees + num(r, "refund") + num(r, "tips")
	dd1, dd2 := third(r, "doorDashSuccessfulDeliveryFee"), third(r, "doorDashFailedDeliveryFee")
	r["totalApproved"] = num(r, "totalSalesTip") + num(r, "giftCardSales") + num(r, "giftCardTips") +
		num(r, "depositSales") + num(r, "depositTips") + num(r, "houseAccountSales") + num(r, "houseAccountTips") +
		num(r, "svcFee") + dd1 + dd2
	r["merchantFee"] = -1 * num(r, "merchantFee")
	r["depositAmount"] = num(r, "totalApproved") + num(r, "merchantFee")
	r["hasData"] = num(r, "totalSales") != 0 || num(r, "tips") != 0 || num(r, "serviceFee") != 0 ||
		num(r, "areaFee") != 0 || num(r, "partyServiceFee") != 0 || num(r, "autoGratuityFee") != 0 ||
		num(r, "deliveryFee") != 0 || num(r, "svcFee") != 0 || num(r, "merchantFee") != 0 || dd1 != 0 || dd2 != 0
	return r
}

func feeLabels(r report.Row, l labels) []report.Label {
	var out []report.Label
	add := func(cond bool, prop, name string) {
		if cond {
			out = append(out, currency(prop, name))
		}
	}
	add(num(r, "serviceFee") != 0, "serviceFee", "Service Charge")
	add(num(r, "areaFee") != 0, "areaFee", l.area)
	add(num(r, "partyServiceFee") != 0, "partyServiceFee", l.party)
	add(num(r, "autoGratuityFee") != 0, "autoGratuityFee", l.auto)
	add(num(r, "deliveryFee") != 0, "deliveryFee", "Online Delivery Fee")
	return out
}

func saleLabels(r report.Row, l labels, sales string, inclServiceFee bool) []report.Label {
	labels := []report.Label{currency("totalSales", sales)}
	labels = append(labels, feeLabels(r, l)...)
	if inclServiceFee && num(r, "salesServiceFee") != 0 {
		labels = append(labels, currency("salesServiceFee", "Sales (incl service fees)"))
	}
	labels = append(labels, currency("tips", "Tips"))
	if num(r, "refund") != 0 {
		labels = append(labels, currency("refund", "Refund"))
	}
	return labels
}

func settlementLabels(r report.Row, l labels) []report.Label {
	var labels []report.Label
	if num(r, "salePaidOut") != 0 {
		labels = append(labels, currency("salePaidOut", "Sales Paid Out"))
	}
	labels = append(labels, report.Label{Prop: "totalSalesTip", Name: "Total Sales", Type: "currency", Bold: true})
	labels = append(labels, liabilityLabels(r)...)
	if num(r, "svcFee") != 0 {
		labels = append(labels, currency("svcFee", l.customFee))
	}
	return labels
}

func liabilityLabels(r report.Row) []report.Label {
	var out []report.Label
	for _, x := range []struct{ prop, name string }{
		{"giftCardSales", "Gift Card Sales"}, {"giftCardTips", "Gift Card Tips"},
		{"depositSales", "Deposit Sales"}, {"depositTips", "Deposit Tips"},
		{"houseAccountSales", "House Account Sales"}, {"houseAccountTips", "House Account Tips"},
	} {
		if num(r, x.prop) != 0 {
			out = append(out, currency(x.prop, x.name))
		}
	}
	return out
}

func hasLiabilityOrFee(r report.Row) bool {
	return num(r, "svcFee") != 0 || num(r, "giftCardSales") != 0 || num(r, "giftCardTips") != 0 ||
		num(r, "depositSales") != 0 || num(r, "depositTips") != 0 ||
		num(r, "houseAccountSales") != 0 || num(r, "houseAccountTips") != 0
}

func creditCard(in Input, l labels) report.Table {
	r := merchant(in.SalesByCreditCard)
	if r == nil {
		return empty("Credit Card")
	}
	labels := slices.Concat(saleLabels(r, l, "Sales (w/o tip)", true), settlementLabels(r, l))
	if hasLiabilityOrFee(r) {
		labels = append(labels, report.Label{Prop: "totalApproved", Name: "Total Approved", Type: "currency", Bold: true})
	}
	if num(r, "merchantFee") != 0 {
		labels = append(labels, currency("merchantFee", l.merchantFee),
			report.Label{Prop: "depositAmount", Name: "Deposit Amount", Type: "currency", Bold: true})
	}
	return report.Table{Title: "Credit Card", Orientation: "horizontal", Labels: labels, Rows: []report.Row{r}}
}

func cash(in Input, l labels) report.Table {
	r := merchant(in.SalesByCash)
	if r == nil {
		return empty("Cash")
	}
	labels := slices.Concat(saleLabels(r, l, "Sales", false), settlementLabels(r, l))
	if hasLiabilityOrFee(r) {
		labels = append(labels, currency("totalApproved", "Total"))
	}
	return report.Table{Title: "Cash", Orientation: "horizontal", Labels: labels, Rows: []report.Row{r}}
}

// onlineSales also breaks out the DoorDash fees.
func onlineSales(src report.Row, title string, l labels) report.Table {
	r := merchant(src)
	if hasData, _ := r["hasData"].(bool); r == nil || !hasData {
		return empty(title)
	}
	labels := append(saleLabels(r, l, "Sales (w/o tip)", true), report.Label{Prop: "totalSalesTip", Name: "Total Sales", Type: "currency", Bold: true})
	dd1, dd2 := third(r, "doorDashSuccessfulDeliveryFee"), third(r, "doorDashFailedDeliveryFee")
	if dd1 != 0 {
		r["doorDashSuccessfulDeliveryFee"] = dd1
		labels = append(labels, currency("doorDashSuccessfulDeliveryFee", "DoorDash Successful Delivery Fee"))
	}
	if dd2 != 0 {
		r["doorDashFailedDeliveryFee"] = dd2
		labels = append(labels, currency("doorDashFailedDeliveryFee", "DoorDash Failed Delivery Fee"))
	}
	if num(r, "svcFee") != 0 {
		labels = append(labels, currency("svcFee", l.webCustomFee))
	}
	if num(r, "svcFee") != 0 || dd1 != 0 || dd2 != 0 {
		labels = append(labels, report.Label{Prop: "totalApproved", Name: "Total Approved", Type: "currency", Bold: true})
	}
	return report.Table{Title: title, Orientation: "horizontal", Labels: labels, Rows: []report.Row{r}}
}

type feeTaxTables struct {
	taxable, nonTaxable report.Table
}

var (
	taxableRows    = []string{"serviceCharge", "autoServiceCharge", "partyServiceCharge", "areaServiceCharge", "customFee"}
	nonTaxableRows = []string{"serviceCharge", "autoServiceCharge", "partyServiceCharge", "areaServiceCharge"}
)

// feeTaxSections nests taxable rows under their tax code, then flattens so heading and children are both rows.
func feeTaxSections(in Input, l labels) feeTaxTables {
	names := map[string]string{
		"serviceCharge": "Service Charge", "autoServiceCharge": l.auto, "partyServiceCharge": l.party,
		"areaServiceCharge": l.area, "customFee": l.customFee,
	}
	taxableLabels := []report.Label{text("name", "Taxable"), currency("netAmount", "Net Amount"), currency("tax", "Tax")}
	nonTaxableLabels := []report.Label{text("name", "Nontaxable"), currency("netAmount", "Net Amount"), currency("tax", "Tax")}

	var taxable, nonTaxable []report.Row
	totalTax := 0.0
	if ft := in.FeeTax; ft != nil {
		totalTax = ft.TotalTaxAmount
		for _, d := range ft.TaxableDetails {
			if d.str("taxCodeID") == "" {
				continue
			}
			var children []report.Row
			// payload order, like Object.entries
			for _, key := range d.keys {
				v, isNum := d.m[key].(float64)
				if !isNum || v <= 0 || !slices.Contains(taxableRows, key) {
					continue
				}
				var tax any = ""
				if t := d.num(key + "Tax"); t != 0 {
					tax = t
				}
				children = append(children, report.Row{"name": names[key], "netAmount": v, "tax": tax})
			}
			parent := report.Row{
				"name": d.str("taxCodeName") + " (" + jsNum(d.num("taxRate")) + "%)", "netAmount": "empty",
				"tax": d.num("taxAmount"), "isBold": true, "datasets": children,
			}
			taxable = append(taxable, parent)
			taxable = append(taxable, children...)
		}
		for _, key := range ft.NonTaxableDetail.keys {
			v, isNum := ft.NonTaxableDetail.m[key].(float64)
			if !isNum || v <= 0 || !slices.Contains(nonTaxableRows, key) {
				continue
			}
			nonTaxable = append(nonTaxable, report.Row{"name": names[key], "netAmount": v, "tax": 0.0})
		}
	}

	if len(taxable) == 0 && len(nonTaxable) == 0 {
		return feeTaxTables{
			taxable:    report.Table{Labels: taxableLabels, Rows: []report.Row{}},
			nonTaxable: report.Table{Labels: nonTaxableLabels, Rows: []report.Row{}},
		}
	}
	var summary []report.Row
	if len(taxable) > 0 {
		summary = []report.Row{
			{"name": "", "netAmount": "empty", "tax": "empty"},
			{"name": "Summary", "netAmount": "empty", "isTotal": true, "tax": totalTax},
		}
	}
	const title = "Service Charge & Fee's Tax"
	out := feeTaxTables{
		taxable:    report.Table{Title: title, Labels: taxableLabels, Rows: taxable},
		nonTaxable: report.Table{Labels: nonTaxableLabels, Rows: []report.Row{}},
	}
	if len(nonTaxable) == 0 {
		out.taxable.Rows = append(out.taxable.Rows, summary...)
	} else {
		out.nonTaxable.Rows = slices.Concat(nonTaxable, summary)
	}
	if len(taxable) == 0 {
		out.nonTaxable.Title = title
	}
	return out
}
