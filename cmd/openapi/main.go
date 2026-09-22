package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"unicode"

	"api-report-nexus/internal/domain/category"
	"api-report-nexus/internal/domain/daypart"
	"api-report-nexus/internal/domain/report"
	"api-report-nexus/internal/domain/salessummary"
	"api-report-nexus/internal/usecase/sendmail"
)

//go:embed head.yaml
var head string

// roots are emitted in this order; structs they reference follow as they are met.
var roots = []struct {
	name string
	v    any
}{
	{"Shift", daypart.Shift{}}, {"Config", report.Config{}},
	{"LowInventoryItem", sendmail.LowInventoryItem{}}, {"ItemSales", sendmail.ItemSales{}}, {"EmployeeSale", sendmail.EmployeeSale{}},
	{"ModifierData", sendmail.ModifierData{}}, {"ModifierCategory", sendmail.ModifierCategory{}}, {"ModifierSales", sendmail.Modifier{}},
	{"CategoryData", sendmail.CategoryData{}}, {"Category", category.Category{}}, {"Item", category.Item{}}, {"ItemModifier", category.Modifier{}},
	{"DeliveryFeeSummary", sendmail.DeliveryFeeSummary{}}, {"DeliveryFeeRow", sendmail.DeliveryFeeRow{}},
	{"SalesSummaryInput", salessummary.Input{}}, {"SSCategory", salessummary.Category{}},
}

const cardGroup = "Card or cash group as the POS sends it; every key is re-emitted. Totals are derived from totalSales, tips, serviceFee, areaFee, partyServiceFee, autoGratuityFee, deliveryFee, refund, giftCardSales, giftCardTips, depositSales, depositTips, houseAccountSales, houseAccountTips, svcFee, merchantFee and thirdParty.doorDashSuccessfulDeliveryFee / doorDashFailedDeliveryFee."

// notes say what the struct alone cannot, keyed Type.field.
var notes = map[string]string{
	"Shift.dayOfWeek":                           "Monday … Sunday; tables are ordered this way, unknown names first.",
	"Shift.dayPart":                             "Day part name, e.g. Breakfast. Rows are grouped by it and labelled with the clock of the last shift.",
	"Shift.startDate":                           "ISO-8601 with offset.",
	"Shift.endDate":                             "ISO-8601 with offset.",
	"Config.configurationName":                  "POS setting key, e.g. SaleTypesConfig.ToGoLabel.",
	"ModifierSales.name":                        "null prints an empty cell.",
	"SSCategory.nameAlias":                      "Wins over name when present, even when empty.",
	"SaleType.totalSales":                       "null means net sales + tax.",
	"CancelDetail.saleReceiptNumber":            "Receipt number; becomes the receipt link text.",
	"DeliveryFeeRow.date":                       "Day rows: the day, printed as sent. Detail rows: ticket time, ISO-8601 with offset.",
	"DeliveryFeeRow.ticketNumber":               "Detail rows only; \"-\" when empty.",
	"DeliveryFeeRow.details":                    "Tickets of the day; day rows only.",
	"SalesSummaryInput.salesByDayPartBreakdown": "When present, even empty, the workbook gets a Day Part Breakdown sheet.",
	"SalesSummaryInput.labelConfigs":            "POS label overrides read by the report: SystemConfigValue.GratuityName / ServiceChargeName / AutoGratuityName / CreditCardFeeMode, CreditCardConfigValue.SVCFeeLabel / MerchantFeeLabel / SvcFeeLabelWeb / SVCFeeLabelOnReport, SaleTypesConfig.DineInLabel / ToGoLabel.",
	"SalesSummaryInput.salesByCreditCard":       cardGroup,
	"SalesSummaryInput.salesByCash":             cardGroup,
	"SalesSummaryInput.salesByOnlineOrdering":   cardGroup,
	"SalesSummaryInput.salesByQRCodeDineIn":     cardGroup,
	"FeeTax.taxableDetails":                     "Objects whose keys are listed in payload order.",
}

var (
	names = map[reflect.Type]string{}
	queue []reflect.Type
)

func main() {
	for _, r := range roots {
		register(r.name, reflect.TypeOf(r.v))
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(head, "\n") + "\n")
	for i := 0; i < len(queue); i++ { //nolint:intrange // queue grows while emitting
		t := queue[i]
		fmt.Fprintf(&b, "    %s:\n      type: object\n      properties:\n", names[t])
		fields(&b, t, names[t], 8)
	}
	_, _ = os.Stdout.WriteString(b.String())
}

func register(name string, t reflect.Type) string {
	if n, ok := names[t]; ok {
		return n
	}
	names[t] = name
	queue = append(queue, t)
	return name
}

func fields(b *strings.Builder, t reflect.Type, owner string, ind int) {
	p := strings.Repeat(" ", ind)
	for f := range t.Fields() {
		tag, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if f.PkgPath != "" || tag == "-" {
			continue
		}
		if tag == "" {
			// encoding/json matches names case-insensitively, so document the camelCase the POS sends
			tag = lowerFirst(f.Name)
		}
		fmt.Fprintf(b, "%s%s:\n%s", p, tag, schema(f.Type, ind+2))
		if n, ok := notes[owner+"."+tag]; ok {
			q, _ := json.Marshal(n)
			fmt.Fprintf(b, "%s  description: %s\n", p, q)
		}
	}
}

func schema(t reflect.Type, ind int) string {
	p := strings.Repeat(" ", ind)
	switch t.Kind() {
	case reflect.Pointer:
		s := schema(t.Elem(), ind)
		if strings.Contains(s, "$ref") { // nullable is ignored next to $ref
			return s
		}
		return s + p + "nullable: true\n"
	case reflect.Slice:
		return p + "type: array\n" + p + "items:\n" + schema(t.Elem(), ind+2)
	case reflect.Map:
		return p + "type: object\n" + p + "additionalProperties: true\n"
	case reflect.Interface:
		// the POS sends these as a string or a number
		return p + "oneOf:\n" + p + "  - type: string\n" + p + "  - type: number\n"
	case reflect.Struct:
		if t.Name() == "" {
			var b strings.Builder
			fields(&b, t, "", ind+2)
			return p + "type: object\n" + p + "properties:\n" + b.String()
		}
		if t.NumField() > 0 && t.Field(0).PkgPath != "" { // decoded by hand, e.g. salessummary.ordered
			return p + "type: object\n" + p + "additionalProperties: true\n"
		}
		return p + "$ref: '#/components/schemas/" + register(t.Name(), t) + "'\n"
	case reflect.Float64:
		return p + "type: number\n"
	case reflect.Int:
		return p + "type: integer\n"
	case reflect.Bool:
		return p + "type: boolean\n"
	default:
		return p + "type: string\n"
	}
}

func lowerFirst(s string) string {
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}
