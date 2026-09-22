package salessummary

import (
	"sort"
	"strconv"

	"api-report-nexus/internal/domain/report"
	"api-report-nexus/internal/pkg/datetime"
	"api-report-nexus/internal/pkg/money"
)

// Section is one named block; the Summary sheet lists sections in the order Map returns them.
type Section struct {
	Key string
	report.Table
}

type Options struct {
	// POS version, MM.dd.yyyy
	Version string
	StoreID string
	// base URL for receipt links
	ClientURL string
}

const (
	fmtInt      = "#,##0"
	noteNetTax  = "Net Sales + Tax"
	noteNoTaxes = "Taxes and service charges not included"
)

// Map returns every section in the order Node's object literal declared them; inapplicable ones have no rows.
func Map(in Input, o Options) []Section {
	l := readLabels(in.LabelConfigs)
	if o.ClientURL == "" {
		o.ClientURL = "https://blogicview.com"
	}
	feeTax := feeTaxSections(in, l)

	sections := []Section{
		{"summary1stLine", summary1(in, l)},
		{"summary2ndLine", summary2(in, o.Version)},
		{"summary3rdLine", summary3(in, o.Version, l)},
		{"summary4rdLine", summary4(in, o.Version, l)},
		{"paidBalance", paidBalance(in)},
		{"thirdParty", thirdParty(in)},
		{"liabilities1stLine", giftCards1(in)},
		{"liabilities2ndLine", giftCards2(in)},
		{"liabilities3rdLine", houseAccounts1(in)},
		{"liabilities4thLine", houseAccounts2(in)},
		{"deposit1stLine", deposits1(in)},
		{"deposit2ndLine", deposits2(in)},
		{"payments", payments(in, o.Version)},
		{"paymentCreditTypes", paymentGroup(in.PaymentSummary.CreditTypes, "Credit Types", "Unknown")},
		{"paymentAlternateTypes", paymentGroup(in.PaymentSummary.OtherTypes, "Other Types", "")},
		{"creditCard", creditCard(in, l)},
		{"cash", cash(in, l)},
		{"laborSummary", labor(in, o.Version)},
		{"categories", categories(in.SalesCategories, o.Version, "Sales Categories", "Category", true)},
		{"itemTypes", categories(in.SalesByItemTypes, o.Version, "Sales by Item Type", "Item Type", false)},
		{"itemsByDatePart", dayPartsLegacy(in, o.Version)},
		{"itemsByDatePartV2", dayPartsV2(in, o.Version)},
		{"saleTypes", saleTypes(in, l)},
		{"salesByChannel", salesByChannel(in)},
		{"saleTypesThirdPartyBreakdown", thirdPartyBreakdown(in)},
		{"taxes", taxes(in)},
		{"customFeeSummary", customFeeSummary(in)},
		{"taxableBreakDetails", taxableBreakDetails(in)},
		{"tips", tips(in)},
		{"serviceCharges", serviceCharges(in)},
		{"taxableDatasets", feeTax.taxable},
		{"nonTaxableDatasets", feeTax.nonTaxable},
		{"voids", voids(in)},
		{"voidsDetail", receipts(in.VoidDetails, "Void Detail", o, "", false)},
		{"cancelled", cancelled(in)},
		{"cancelledDetail", receipts(in.CancelDetails, "Cancel Detail", o, "?includedCanceledItem=true", true)},
		{"comps", comps(in)},
		{"compsDetail", receipts(in.CompDetails, "Comp Detail", o, "", true)},
		{"discounts", discounts(in)},
		{"cashOut", cashOut(in)},
		{"onlineOrdering", onlineSales(in.SalesByOnlineOrdering, "Credit Card Online Order", l)},
		{"QRCodeDineIn", onlineSales(in.SalesByQRCodeDineIn, "Credit Card QRCode Dine-in", l)},
		{"hourlyBreakdown", breakdown(in.HourlyBreakdowns, "Hourly Breakdown", "Hour", true)},
		{"weekdayBreakdown", breakdown(in.WeekdayBreakdowns, "Weekday Breakdown", "Weekday", false)},
	}
	return sections
}

func acc[T any](xs []T, f func(T) float64) float64 {
	total := 0.0
	for _, x := range xs {
		total += f(x)
	}
	return total
}

func currency(prop, name string) report.Label {
	return report.Label{Prop: prop, Name: name, Type: "currency"}
}

func currencyNote(prop, name, note string) report.Label {
	return report.Label{Prop: prop, Name: name, Type: "currency", Note: note}
}

func count(prop, name string) report.Label {
	return report.Label{Prop: prop, Name: name, Type: "number", Format: fmtInt}
}

func text(prop, name string) report.Label {
	return report.Label{Prop: prop, Name: name, Type: "string"}
}

func percent(prop string) report.Label {
	return report.Label{Prop: prop, Name: "Percent", Type: "percent"}
}

func empty(title string) report.Table {
	return report.Table{Title: title, Labels: []report.Label{}, Rows: []report.Row{}}
}

// jsNum renders a number as JavaScript template interpolation would.
func jsNum(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func summary1(in Input, l labels) report.Table {
	s := in.SalesSummary
	labels := []report.Label{
		currencyNote("dineInSales", l.dineIn+" Sales", noteNoTaxes),
		currencyNote("toGoSales", l.toGo+" Sales", noteNoTaxes),
	}
	if s.OtherSales != 0 {
		labels = append(labels, currency("otherSales", "Other Sales"))
	}
	labels = append(labels,
		currencyNote("accountReceivables", "Accounts Receivables", noteNoTaxes),
		currencyNote("netSales", "Net Sales ("+l.dineIn+" + "+l.toGo+" + AR)",
			"Net Sales = "+l.dineIn+" Sales + "+l.toGo+" Sales + Accounts Receivables"),
		currency("totalNetSales", "Total Net Sales"),
	)
	return report.Table{Title: "Sales Summary", Labels: labels, Rows: []report.Row{{
		"dineInSales": s.DineInSales, "toGoSales": s.ToGoSales, "otherSales": s.OtherSales,
		"accountReceivables": s.AccountReceivables,
		"netSales":           s.DineInSales + s.ToGoSales + s.AccountReceivables,
		"totalNetSales":      in.NetSales,
	}}}
}

func summary2(in Input, version string) report.Table {
	s := in.SalesSummary
	revenue := in.NetSales + s.ServiceCharges + s.Tips
	labels := []report.Label{
		currencyNote("serviceCharges", "Service Charges", "Service Charges Tax not included"),
		currencyNote("tips", "Tips", "Includes AR Tips & Liabilities Tips"),
		currencyNote("totalRevenue", "Total Revenue", "Total Revenue = Net Sales + SVC + Tips"),
		currencyNote("tax", "Tax", "Includes Service Charges Tax"),
		currencyNote("gross", "Gross Sales", "Gross Sales = Total Revenue + Tax"),
	}
	if versionGTE(version, versionAvgSales) {
		labels = append(labels, currency("discounts", "Discounts"))
	}
	return report.Table{Labels: labels, Rows: []report.Row{{
		"serviceCharges": s.ServiceCharges, "tips": s.Tips, "totalRevenue": revenue,
		"tax": s.Tax, "gross": revenue + s.Tax, "discounts": s.Discounts,
	}}}
}

func summary3(in Input, version string, l labels) report.Table {
	s := in.SalesSummary
	var labels []report.Label
	if !versionGTE(version, versionAvgSales) {
		labels = []report.Label{
			currency("discounts", "Discounts"),
			count("ticketsCount", "Tickets Count"),
			count("guestsCount", "Guests Count"),
			currency("avgSales", "Avg Sales"),
			currency("avgSalesByGuest", "Avg Sales/Guest"),
		}
	} else {
		labels = []report.Label{
			count("ticketsCount", "Tickets Count"),
			count("guestsCount", "Total Guests Count"),
			currencyNote("avgSales", "Avg Net Sales/Ticket", "Avg Net Sales = Total Net Sales / Tickets Count"),
			count("dineInGuestsCount", l.dineIn+" Guests Count"),
			currencyNote("dineInAvgSalesByTicket", l.dineIn+" Avg Net Sales/Ticket",
				"Net Sales (Accounts Receivables included) = "+money.USD(s.DineInSalesIncludeAR)+
					"\nTickets Count = "+jsNum(s.DineInTicketCount)),
			currencyNote("dineInAvgSalesByGuest", l.dineIn+" Avg Net Sales/Guest",
				"Net Sales (Accounts Receivables included) = "+money.USD(s.NetSaleHavingGuest)+
					"\n(Net sales are only calculated for tickets with guests.)"),
		}
	}
	return report.Table{Labels: labels, Rows: []report.Row{{
		"ticketsCount": s.TicketsCount, "guestsCount": s.GuestsCount, "avgSales": s.AvgSales,
		"avgSalesByGuest": s.AvgSalesByGuest, "dineInGuestsCount": s.DineInGuestsCount,
		"dineInAvgSalesByTicket": s.DineInAvgSalesByTicket, "dineInAvgSalesByGuest": s.DineInAvgSalesByGuest,
		"discounts": s.Discounts,
	}}}
}

func summary4(in Input, version string, l labels) report.Table {
	if !versionGTE(version, versionAvgSales) {
		return report.Table{Labels: []report.Label{}, Rows: []report.Row{}}
	}
	s := in.SalesSummary
	return report.Table{Labels: []report.Label{
		currencyNote("avgSalesByGuest", "Avg Total Net Sales/Guest",
			"Total Net Sales = "+money.USD(s.NetSaleHavingGuest)+
				"\n(Net sales are only calculated for tickets with guests.)"),
		count("quickGuestsCount", l.toGo+" Guests Count"),
		currencyNote("quickAvgSalesByTicket", l.toGo+" Avg Net Sales/Ticket",
			"Net Sales (Accounts Receivables included) = "+money.USD(s.QuickSalesIncludeAR)+
				"\nTickets Count = "+jsNum(s.QuickTicketCount)),
		currencyNote("quickAvgSalesByGuest", l.toGo+" Avg Net Sales/Guest",
			"Net Sales (Accounts Receivables included) = "+money.USD(s.QuickNetSaleHavingGuest)+
				"\n(Net sales are only calculated for tickets with guests.)"),
	}, Rows: []report.Row{{
		"avgSalesByGuest": s.AvgSalesByGuest, "quickGuestsCount": s.QuickGuestsCount,
		"quickAvgSalesByTicket": s.QuickAvgSalesByTicket, "quickAvgSalesByGuest": s.QuickAvgSalesByGuest,
	}}}
}

func paidBalance(in Input) report.Table {
	p := in.PaidBalance
	rows := []report.Row{}
	if p.PaidBalance != 0 || p.TipPaidBalance != 0 {
		rows = append(rows, report.Row{"balance": p.PaidBalance, "tip": p.TipPaidBalance, "total": p.PaidBalance + p.TipPaidBalance})
	}
	return report.Table{Title: "Paid Balance", Labels: []report.Label{
		currency("balance", "Paid Balance"), currency("tip", "Tip"), currency("total", "Total"),
	}, Rows: rows}
}

func thirdParty(in Input) report.Table {
	t := in.ThirdParty
	if t.DoorDashSuccessfulDeliveryFee == 0 && t.DoorDashFailedDeliveryFee == 0 {
		return empty("Third Party")
	}
	var labels []report.Label
	if t.DoorDashSuccessfulDeliveryFee != 0 {
		labels = append(labels, currency("doorDashSuccessfulDeliveryFee", "DoorDash Successful Delivery Fee"))
	}
	if t.DoorDashFailedDeliveryFee != 0 {
		labels = append(labels, currency("doorDashFailedDeliveryFee", "DoorDash Failed Delivery Fee"))
	}
	return report.Table{Title: "Third Party", Labels: labels, Rows: []report.Row{{
		"doorDashSuccessfulDeliveryFee": t.DoorDashSuccessfulDeliveryFee,
		"doorDashFailedDeliveryFee":     t.DoorDashFailedDeliveryFee,
	}}}
}

func liability(title string, present bool, labels []report.Label, row report.Row) report.Table {
	rows := []report.Row{}
	if present {
		rows = append(rows, row)
	}
	return report.Table{Title: title, Labels: labels, Rows: rows}
}

func giftCards1(in Input) report.Table {
	x := in.Liabilities
	return liability("Liabilities", x.GiftCardCount > 0, []report.Label{
		count("giftCardCount", "Gift Card Count"), currency("giftCardIssued", "Gift Card Issued"),
		currency("giftCardTips", "Gift Card Tips"), currency("giftCardTotal", "Total Gift Card Liabilities"),
	}, report.Row{"giftCardCount": x.GiftCardCount, "giftCardIssued": x.GiftCardIssued,
		"giftCardTips": x.GiftCardTips, "giftCardTotal": x.TotalGiftCardLiabilities})
}

func giftCards2(in Input) report.Table {
	x := in.Liabilities
	return liability("", x.GiftCardCount > 0, []report.Label{
		currency("giftCardCredit", "Gift Card Credit"), currency("giftCardCash", "Gift Card Cash"),
		currency("giftCardOther", "Gift Card Other"),
	}, report.Row{"giftCardCredit": x.GiftCardCredit, "giftCardCash": x.GiftCardCash, "giftCardOther": x.GiftCardOther})
}

func houseAccounts1(in Input) report.Table {
	x := in.Liabilities
	return liability("", x.HouseAccountCount > 0, []report.Label{
		count("houseAccountCount", "House Account Count"), currency("houseAccountIssued", "House Account Add Fund"),
		currency("houseAccountTips", "House Account Tips"),
		currency("totalHouseAccountLiabilities", "Total House Account Liabilities"),
	}, report.Row{"houseAccountCount": x.HouseAccountCount, "houseAccountIssued": x.HouseAccountIssued,
		"houseAccountTips": x.HouseAccountTips, "totalHouseAccountLiabilities": x.TotalHouseAccountLiabilities})
}

func houseAccounts2(in Input) report.Table {
	x := in.Liabilities
	return liability("", x.HouseAccountCount > 0, []report.Label{
		currency("houseAccountCredit", "House Account Credit"), currency("houseAccountCash", "House Account Cash"),
		currency("houseAccountOther", "House Account Other"),
	}, report.Row{"houseAccountCredit": x.HouseAccountCredit, "houseAccountCash": x.HouseAccountCash,
		"houseAccountOther": x.HouseAccountOther})
}

func deposits1(in Input) report.Table {
	x := in.Liabilities
	title := "Liabilities"
	if x.GiftCardCount > 0 {
		// the gift card block already carried the heading
		title = ""
	}
	return liability(title, x.DepositCount > 0, []report.Label{
		count("depositCount", "Deposit Count"), currency("depositIssued", "Deposit Issued"),
		currency("depositTips", "Deposit Tips"), currency("totalDepositLiabilities", "Total Deposit Liabilities"),
	}, report.Row{"depositCount": x.DepositCount, "depositIssued": x.DepositIssued,
		"depositTips": x.DepositTips, "totalDepositLiabilities": x.TotalDepositLiabilities})
}

func deposits2(in Input) report.Table {
	x := in.Liabilities
	return liability("", x.DepositCount > 0, []report.Label{
		currency("depositCredit", "Deposits Credit"), currency("depositCash", "Deposit Cash"),
		currency("depositOther", "Deposit Other"),
	}, report.Row{"depositCredit": x.DepositCredit, "depositCash": x.DepositCash, "depositOther": x.DepositOther})
}

func paymentTotal(p Payment) float64 {
	return p.Amount + p.Tips + p.ServiceCharges + p.ThirdParty + p.Refunds
}

func paymentRow(p Payment, name string, total, grand float64) report.Row {
	return report.Row{
		"name": name, "count": p.Count, "amount": p.Amount, "tips": p.Tips,
		"serviceCharges": p.ServiceCharges, "thirdParty": p.ThirdParty, "refunds": p.Refunds,
		"total": total, "percent": report.Percent(total, grand),
	}
}

func paymentLabels(nameNote, amountNote string) []report.Label {
	return []report.Label{
		{Prop: "name", Name: "Payment Type", Type: "string", Note: nameNote},
		count("count", "Count"),
		currencyNote("amount", "Amount", amountNote),
		currency("tips", "Tips"), currency("serviceCharges", "Service Charges"),
		currency("thirdParty", "3rd Party"), currency("refunds", "Refunds"),
		currency("total", "Total"), percent("percent"),
	}
}

func payments(in Input, version string) report.Table {
	types := in.PaymentSummary.PaymentTypes
	grand := acc(types, paymentTotal)
	rows := []report.Row{}
	for _, p := range types {
		rows = append(rows, paymentRow(p, p.TypeName, paymentTotal(p), grand))
	}
	if len(rows) > 0 {
		rows = append(rows, report.Row{
			"name": "Total", "isTotal": true,
			"count":          acc(types, func(p Payment) float64 { return p.Count }),
			"amount":         acc(types, func(p Payment) float64 { return p.Amount }),
			"tips":           acc(types, func(p Payment) float64 { return p.Tips }),
			"serviceCharges": acc(types, func(p Payment) float64 { return p.ServiceCharges }),
			"thirdParty":     acc(types, func(p Payment) float64 { return p.ThirdParty }),
			"refunds":        acc(types, func(p Payment) float64 { return p.Refunds }),
			"total":          grand, "percent": 1.0,
		})
	}
	amountNote := "Amount = Net Sales + Tax"
	if versionLT(version, versionCoinDiscount) {
		amountNote = "Amount = Net Sales + Tax + Cash Discount"
	}
	return report.Table{Title: "Payment Summary", Labels: paymentLabels("Liabilities payments included", amountNote), Rows: rows}
}

func paymentGroup(types []Payment, title, fallbackName string) report.Table {
	grand := acc(types, paymentTotal)
	rows := make([]report.Row, 0, len(types))
	for _, p := range types {
		name := p.TypeName
		if name == "" {
			name = fallbackName
		}
		rows = append(rows, paymentRow(p, name, paymentTotal(p), grand))
	}
	return report.Table{Title: title, Labels: paymentLabels("", ""), Rows: rows}
}

func labor(in Input, version string) report.Table {
	if !versionGTE(version, versionLabor) {
		return empty("Labor Summary")
	}
	x := in.LaborOverview
	return report.Table{Title: "Labor Summary", Labels: []report.Label{
		currencyNote("laborCost", "Labor Cost", "Total wages of clocked in employees"),
		currency("netSales", "Net Sales"),
		{Prop: "laborCostPercent", Name: "Labor %", Type: "percent", Note: "Labor Cost / Net Sales"},
		currency("avgLaborCostByHours", "Avg Labor Cost / Hour"),
		currency("avgSalesByHours", "Avg Net Sales / Hour"),
		currencyNote("avgHourlyWagesPerEmployee", "Avg Hourly Wages / Employee",
			"Avg Hourly Wages / Employee = Total Hourly Wages / Total clocked in employees"),
		currencyNote("avgNetSalesPerEmployee", "Avg Net Sales / Employee",
			"Avg Net Sales / Employee = Total Net Sales / Total clocked in employees"),
	}, Rows: []report.Row{{
		"laborCost": x.LaborCost, "netSales": x.NetSales, "laborCostPercent": x.LaborCostPercent / 100,
		"avgLaborCostByHours": x.AvgLaborCostByHours, "avgSalesByHours": x.AvgSalesByHours,
		"avgHourlyWagesPerEmployee": x.AvgHourlyWagesPerEmployee, "avgNetSalesPerEmployee": x.AvgNetSalesPerEmployee,
	}}}
}

func categories(cats []Category, version, title, nameHeader string, useAlias bool) report.Table {
	total := func(c Category) float64 { return c.NetSales + c.Tax }
	grand := acc(cats, total)
	rows := []report.Row{}
	for _, c := range cats {
		name := c.Name
		if useAlias && c.NameAlias != nil {
			name = *c.NameAlias
		}
		rows = append(rows, report.Row{
			"name": name, "quantity": c.Quantity, "netSales": c.NetSales, "discounts": c.Discounts,
			"grossSales": c.GrossSales, "tax": c.Tax, "total": total(c), "percent": report.Percent(total(c), grand),
		})
	}
	if len(rows) > 0 {
		rows = append(rows, report.Row{
			"name": "Total", "isTotal": true,
			"quantity":   acc(cats, func(c Category) float64 { return c.Quantity }),
			"netSales":   acc(cats, func(c Category) float64 { return c.NetSales }),
			"discounts":  acc(cats, func(c Category) float64 { return c.Discounts }),
			"grossSales": acc(cats, func(c Category) float64 { return c.GrossSales }),
			"tax":        acc(cats, func(c Category) float64 { return c.Tax }),
			"total":      grand, "percent": 1.0,
		})
	}
	discountNote := ""
	if versionLT(version, versionCoinDiscount) {
		discountNote = "Cash discount included with CD 2.0"
	}
	return report.Table{Title: title, Labels: []report.Label{
		text("name", nameHeader), count("quantity", "Quantity"), currency("netSales", "Net Sales"),
		currencyNote("discounts", "Discounts", discountNote), currency("grossSales", "Gross Sales"),
		currency("tax", "Tax"), currency("total", "Total"), percent("percent"),
	}, Rows: rows}
}

func dayPartsLegacy(in Input, version string) report.Table {
	const title = "Sales by Day Part by Item Type"
	if versionGTE(version, versionDayPartV2) {
		return empty(title)
	}
	rows := []report.Row{}
	for _, d := range in.SalesByDayPartByItemTypes {
		total := d.Breakfast + d.Lunch + d.Dinner
		rows = append(rows, report.Row{
			"name": d.ItemType, "breakfast": d.Breakfast, "breakfastPercent": report.Percent(d.Breakfast, total),
			"lunch": d.Lunch, "lunchPercent": report.Percent(d.Lunch, total),
			"dinner": d.Dinner, "dinnerPercent": report.Percent(d.Dinner, total), "total": total,
		})
	}
	return report.Table{Title: title, Labels: []report.Label{
		text("name", "Item Type"), currency("breakfast", "Breakfast"), percent("breakfastPercent"),
		currency("lunch", "Lunch"), percent("lunchPercent"),
		currency("dinner", "Dinner"), percent("dinnerPercent"), currency("total", "Total"),
	}, Rows: rows}
}

func dayPartsV2(in Input, version string) report.Table {
	const title = "Sales by Day Part by Item Type"
	if !versionGTE(version, versionDayPartV2) {
		return empty(title)
	}
	labels := []report.Label{text("name", "Item Type")}
	seen := map[string]bool{}
	rows := []report.Row{}
	for _, it := range in.DayPartSalesByItemTypes {
		total := acc(it.DayParts, func(d DayPart) float64 { return d.TotalSales })
		row := report.Row{"name": it.Name, "total": total}
		for _, d := range it.DayParts {
			if !seen[d.ID] {
				labels = append(labels, currency(d.ID+".totalSales", d.Name), percent(d.ID+".percent"))
				seen[d.ID] = true
			}
			row[d.ID+".totalSales"] = d.TotalSales
			row[d.ID+".percent"] = report.Percent(d.TotalSales, total)
		}
		rows = append(rows, row)
	}
	if len(seen) == 0 {
		return empty(title)
	}
	labels = append(labels, currency("total", "Total"))
	return report.Table{Title: title, Labels: labels, Rows: rows}
}

func saleTypeTotal(s SaleType) float64 {
	if s.TotalSales != nil {
		return *s.TotalSales
	}
	return s.NetSales + s.Tax
}

func saleTypeRow(name string, xs []SaleType, total, percent float64) report.Row {
	return report.Row{
		"name":       name,
		"orderCount": acc(xs, func(s SaleType) float64 { return s.OrderCount }),
		"netSales":   acc(xs, func(s SaleType) float64 { return s.NetSales }),
		"tax":        acc(xs, func(s SaleType) float64 { return s.Tax }),
		"total":      total, "percent": percent,
	}
}

func saleTypeLabels(nameHeader string) []report.Label {
	return []report.Label{
		text("name", nameHeader), count("orderCount", "Order Count"), currency("netSales", "Net Sales"),
		currency("tax", "Tax"), currency("total", "Total Sales"), percent("percent"),
	}
}

func saleTypes(in Input, l labels) report.Table {
	types := in.SaleTypes
	grand := acc(types, saleTypeTotal)
	rows := []report.Row{}
	for _, s := range types {
		rows = append(rows, report.Row{
			"name": s.SaleType, "orderCount": s.OrderCount, "netSales": s.NetSales, "tax": s.Tax,
			"total": saleTypeTotal(s), "percent": report.Percent(saleTypeTotal(s), grand),
		})
	}
	// Dine In and To Go float to the top, in that order.
	priority := func(r report.Row) int {
		switch r["name"] {
		case l.dineIn:
			return 1
		case l.toGo:
			return 0
		}
		return -1
	}
	sort.SliceStable(rows, func(i, j int) bool { return priority(rows[i]) > priority(rows[j]) })
	if len(rows) > 0 {
		total := saleTypeRow("Total", types, grand, 1.0)
		total["isTotal"] = true
		rows = append(rows, total)
	}
	return report.Table{Title: "Sale Types", Labels: saleTypeLabels("Sale Type"), Rows: rows}
}

func salesByChannel(in Input) report.Table {
	types := in.SaleTypes
	table := report.Table{Title: "Sales By Channel", Labels: saleTypeLabels("Sale Type"), Rows: []report.Row{}}
	hasChannel := false
	for _, s := range types {
		hasChannel = hasChannel || s.ChannelType != 0
	}
	if !hasChannel {
		return table
	}
	grand := acc(types, saleTypeTotal)
	group := func(channel float64, name string) {
		var members []SaleType
		for _, s := range types {
			if s.ChannelType == channel {
				members = append(members, s)
			}
		}
		if len(members) == 0 {
			return
		}
		total := acc(members, saleTypeTotal)
		table.Rows = append(table.Rows, saleTypeRow(name, members, total, report.Percent(total, grand)))
	}
	group(1000, "In-Store")
	group(2000, "Online")
	if len(table.Rows) > 0 {
		table.Rows = append(table.Rows, saleTypeRow("Total", types, grand, 1.0))
	}
	return table
}

func thirdPartyBreakdown(in Input) report.Table {
	xs := in.SaleTypesThirdPartyBreakdown
	rows := []report.Row{}
	for _, x := range xs {
		rows = append(rows, report.Row{
			"name": x.SaleTypeName, "ticketCount": x.TicketCount, "netSales": x.NetSales,
			"tax": x.Tax, "netSalesTaxAndPaidBalance": x.NetSalesTaxAndPaidBalance,
		})
	}
	if len(rows) > 0 {
		rows = append(rows, report.Row{
			"name":                      "Total",
			"ticketCount":               acc(xs, func(x ThirdPartyBreakdown) float64 { return x.TicketCount }),
			"netSales":                  acc(xs, func(x ThirdPartyBreakdown) float64 { return x.NetSales }),
			"tax":                       acc(xs, func(x ThirdPartyBreakdown) float64 { return x.Tax }),
			"netSalesTaxAndPaidBalance": acc(xs, func(x ThirdPartyBreakdown) float64 { return x.NetSalesTaxAndPaidBalance }),
		})
	}
	return report.Table{Title: "Third Party Provider Break Details", Labels: []report.Label{
		text("name", "OrderType"), count("ticketCount", "Order Count"), currency("netSales", "Net Sales"),
		currency("tax", "Tax"), currency("netSalesTaxAndPaidBalance", "Total Sales"),
	}, Rows: rows}
}

func taxes(in Input) report.Table {
	xs := in.Taxes
	orders := acc(xs, func(t Tax) float64 {
		if t.Name == "Service Charges Tax" {
			// Node nulled this count before summing
			return 0
		}
		return t.OrderCount
	})
	amount := acc(xs, func(t Tax) float64 { return t.TaxAmount })
	rows := []report.Row{}
	if orders != 0 || amount != 0 {
		for _, t := range xs {
			var orderCount any = t.OrderCount
			if t.Name == "Service Charges Tax" {
				// one order can carry several taxes, so the count misleads
				orderCount = nil
			}
			rows = append(rows, report.Row{"name": t.Name, "orderCount": orderCount, "taxAmount": t.TaxAmount, "netSales": t.NetSales})
		}
	}
	if len(rows) > 0 {
		rows = append(rows, report.Row{
			"name": "Total", "isTotal": true, "orderCount": nil, "taxAmount": amount,
			"netSales": acc(xs, func(t Tax) float64 { return t.NetSales }),
		})
	}
	return report.Table{Title: "Taxes", Labels: []report.Label{
		text("name", "Tax"), count("orderCount", "Order Count"),
		currency("taxAmount", "Tax Amount"), currency("netSales", "Net Sales"),
	}, Rows: rows}
}

func customFeeSummary(in Input) report.Table {
	// what the Node template printed for a missing object
	name := "undefined"
	rows := []report.Row{}
	if in.CustomFee != nil {
		name = in.CustomFee.Name
		if in.CustomFee.Tax > 0 {
			rows = append(rows, report.Row{"name": in.CustomFee.Name, "netCustomFee": in.CustomFee.NetCustomFee, "tax": in.CustomFee.Tax})
		}
	}
	return report.Table{Labels: []report.Label{
		currency("tax", name+" Tax"), currency("netCustomFee", name),
	}, Rows: rows}
}

func taxableBreakDetails(in Input) report.Table {
	rows := make([]report.Row, 0, len(in.TaxDetails))
	for _, d := range in.TaxDetails {
		rows = append(rows, report.Row{
			"name": d.TaxCodeName, "rate": d.TaxRate / 100, "grossSales": d.GrossSales,
			"discount": d.DiscountAmount, "netSales": d.NetSales, "taxAmount": d.TaxAmount,
		})
	}
	return report.Table{Title: "Taxable Break Details", Labels: []report.Label{
		text("name", "Name"),
		{Prop: "rate", Name: "Rate", Type: "percent", Format: "#.##,0########%"},
		currency("grossSales", "Gross Sales"), currency("discount", "Discount"),
		currency("netSales", "Net Sales"), currency("taxAmount", "Tax Collected"),
	}, Rows: rows}
}

func tips(in Input) report.Table {
	xs := in.Tips
	orders := acc(xs, func(t Tip) float64 { return t.OrderCount })
	net := acc(xs, func(t Tip) float64 { return t.NetSales })
	total := acc(xs, func(t Tip) float64 { return t.Tips })
	rows := []report.Row{}
	if orders != 0 || total != 0 {
		for _, t := range xs {
			rows = append(rows, report.Row{
				"name": t.Name, "orderCount": t.OrderCount, "netSales": t.NetSales, "tips": t.Tips,
				"percent": report.Percent(t.Tips, t.NetSales),
			})
		}
	}
	if len(rows) > 0 {
		rows = append(rows, report.Row{
			"name": "Total", "isTotal": true, "orderCount": orders, "netSales": net, "tips": total,
			"percent": report.Percent(total, net),
		})
	}
	return report.Table{Title: "Tips", Labels: []report.Label{
		text("name", "Tips"), count("orderCount", "Order Count"), currency("tips", "Tip Amount"),
		currency("netSales", "Net Sales"), percent("percent"),
	}, Rows: rows}
}

func serviceCharges(in Input) report.Table {
	xs := in.ServiceCharges
	total := acc(xs, func(s ServiceCharge) float64 { return s.TotalAmount })
	rows := []report.Row{}
	for _, s := range xs {
		rows = append(rows, report.Row{
			"name": s.Name, "orderCount": s.OrderCount, "netAmount": s.NetAmount, "tax": s.Tax,
			"totalAmount": s.TotalAmount, "percent": report.Percent(s.TotalAmount, total),
		})
	}
	if len(rows) > 0 {
		rows = append(rows, report.Row{
			"name": "Total", "isTotal": true, "orderCount": nil,
			"netAmount":   acc(xs, func(s ServiceCharge) float64 { return s.NetAmount }),
			"tax":         acc(xs, func(s ServiceCharge) float64 { return s.Tax }),
			"totalAmount": total, "percent": 1.0,
		})
	}
	return report.Table{Title: "Service Charges", Labels: []report.Label{
		text("name", "Service Charge"), count("orderCount", "Order Count"), currency("netAmount", "Net Amount"),
		currency("tax", "Tax"), currency("totalAmount", "Total Amount"), percent("percent"),
	}, Rows: rows}
}

func voids(in Input) report.Table {
	v := in.VoidSummary
	rows := []report.Row{}
	if v.Amount > 0 {
		rows = append(rows, report.Row{
			"amount": v.Amount, "orderCount": v.OrderCount, "itemCount": v.ItemCount,
			"percent": report.Percent(v.Amount, in.NetSales),
		})
	}
	return report.Table{Title: "Voids", Labels: []report.Label{
		currencyNote("amount", "Amount", noteNetTax), count("orderCount", "Order Count"),
		count("itemCount", "Item Count"),
		{Prop: "percent", Name: "Percent", Type: "percent", Note: "Percent = Void Amount / Net Sales"},
	}, Rows: rows}
}

func receipts(xs []CancelDetail, title string, o Options, query string, withReason bool) report.Table {
	rows := make([]report.Row, 0, len(xs))
	for _, d := range xs {
		row := report.Row{
			"saleReceiptNumber": orBlank(d.SaleReceiptNumber),
			"employeeRequest":   orZero(d.EmployeeRequest),
			"employeeApproved":  orZero(d.EmployeeApproved),
			"amount":            d.Amount, "itemCount": d.ItemCount,
			"link": o.ClientURL + "/receipt/" + o.StoreID + "/" + d.InvoiceNum + query,
		}
		if withReason {
			row["name"] = d.Name
		}
		rows = append(rows, row)
	}
	labels := []report.Label{{Prop: "saleReceiptNumber", Name: "Ticket #", Type: "link"}}
	if withReason {
		labels = append(labels, text("name", "Reason"))
	}
	labels = append(labels,
		text("employeeRequest", "Employee Request"), text("employeeApproved", "Employee Approved"),
		count("itemCount", "Item Count"), currencyNote("amount", "Amount", noteNetTax),
	)
	return report.Table{Title: title, Labels: labels, Rows: rows}
}

// orBlank is JavaScript's `x || ”` for a number or string.
func orBlank(v any) any {
	switch n := v.(type) {
	case nil:
		return ""
	case float64:
		if n == 0 {
			return ""
		}
	case string:
		if n == "" {
			return ""
		}
	}
	return v
}

// orZero is JavaScript's `x || 0`: an empty name becomes the number 0.
func orZero(s string) any {
	if s == "" {
		return 0.0
	}
	return s
}

func reasonLabels(first report.Label, amountNote string) []report.Label {
	return []report.Label{
		first, currencyNote("amount", "Amount", amountNote),
		count("orderCount", "Order Count"), count("itemCount", "Item Count"), percent("percent"),
	}
}

func reasonRows(xs []Cancel, total float64) []report.Row {
	rows := make([]report.Row, 0, len(xs)+1)
	for _, c := range xs {
		rows = append(rows, report.Row{
			"name": c.Name, "amount": c.Amount, "orderCount": c.OrderCount, "itemCount": c.ItemCount,
			"percent": report.Percent(c.Amount, total),
		})
	}
	return rows
}

func cancelled(in Input) report.Table {
	xs := in.CancelsByReason
	total := acc(xs, func(c Cancel) float64 { return c.Amount })
	rows := reasonRows(xs, total)
	if len(rows) > 0 {
		rows = append(rows, report.Row{
			"isTotal": true, "name": "Total", "amount": total,
			"itemCount":  acc(xs, func(c Cancel) float64 { return c.ItemCount }),
			"orderCount": acc(xs, func(c Cancel) float64 { return c.OrderCount }),
		})
	}
	return report.Table{Title: "Cancel", Labels: reasonLabels(text("name", "Reason"), noteNetTax), Rows: rows}
}

func comps(in Input) report.Table {
	xs := in.CompsByReason
	total := acc(xs, func(c Cancel) float64 { return c.Amount })
	rows := reasonRows(xs, total)
	if len(rows) > 0 {
		rows = append(rows, report.Row{
			"name": "Total", "isTotal": true, "amount": total,
			"orderCount": nil, "itemCount": nil, "percent": nil,
		})
	}
	return report.Table{Title: "Comps", Labels: reasonLabels(text("name", "Reason"), noteNetTax), Rows: rows}
}

func discounts(in Input) report.Table {
	xs := in.Discounts
	total := acc(xs, func(d Discount) float64 { return d.Amount })
	rows := []report.Row{}
	for _, d := range xs {
		rows = append(rows, report.Row{
			"reason": d.Reason, "amount": d.Amount, "ticketCount": d.TicketCount, "itemCount": d.ItemCount,
			"percent": report.Percent(d.Amount, total),
		})
	}
	if len(rows) > 0 {
		rows = append(rows, report.Row{
			"reason": "Total", "isTotal": true, "amount": total,
			"ticketCount": nil, "itemCount": nil, "percent": nil,
		})
	}
	return report.Table{Title: "Discounts", Labels: []report.Label{
		text("reason", "Reason"), currency("amount", "Amount"), count("ticketCount", "Ticket Count"),
		count("itemCount", "Item Count"), percent("percent"),
	}, Rows: rows}
}

func cashOut(in Input) report.Table {
	c := in.CashSummary
	bold := func(prop, name string) report.Label {
		return report.Label{Prop: prop, Name: name, Type: "currency", Bold: true}
	}
	return report.Table{Title: "Cash Summary", Orientation: "horizontal", Labels: []report.Label{
		bold("totalCashPayments", "Total Cash Payments"), currency("cashIn", "Cash in"), currency("cashOut", "Cash out"),
		bold("cashBeforeTipouts", "Cash before Tipouts"), currency("cashGratuity", "Cash Gratuity"),
		currency("cashTip", "Cash Tip"), currency("creditOrNonCashGratuity", "Credit / Non-Cash gratuity"),
		currency("creditOrNonCashTips", "Credit / Non-Cash tips"), bold("totalCash", "Deposit Amount"),
	}, Rows: []report.Row{{
		"totalCashPayments": c.TotalCashPayments, "cashIn": c.CashIn, "cashOut": c.CashOut,
		"cashBeforeTipouts": c.CashBeforeTipouts, "cashGratuity": c.CashGratuity, "cashTip": c.CashTip,
		"creditOrNonCashGratuity": c.CreditOrNonCashGratuity, "creditOrNonCashTips": c.CreditOrNonCashTips,
		"totalCash": c.TotalCash,
	}}}
}

// The totals row is always present, even for empty input, as in Node.
func breakdown(xs []Breakdown, title, firstHeader string, byHour bool) report.Table {
	grand := acc(xs, func(b Breakdown) float64 { return b.Total })
	rows := make([]report.Row, 0, len(xs)+1)
	for _, b := range xs {
		label := b.Title
		if byHour {
			if label == "" {
				label = "00"
			}
			label = datetime.FormatHour(label)
		}
		rows = append(rows, report.Row{
			"title": label, "ticketCount": b.TicketCount, "guestsCount": b.GuestsCount, "total": b.Total,
			"percent": report.Percent(b.Total, grand),
		})
	}
	rows = append(rows, report.Row{
		"title": "Total", "isTotal": true,
		"ticketCount": acc(xs, func(b Breakdown) float64 { return b.TicketCount }),
		"guestsCount": acc(xs, func(b Breakdown) float64 { return b.GuestsCount }),
		"total":       grand, "percent": 1.0,
	})
	return report.Table{Title: title, Labels: []report.Label{
		text("title", firstHeader), count("ticketCount", "Order Count"), count("guestsCount", "Guest Count"),
		currency("total", "Net Sales + Tax"), percent("percent"),
	}, Rows: rows}
}
