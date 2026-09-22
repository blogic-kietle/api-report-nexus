package salessummary

import (
	"bytes"
	"encoding/json"

	"api-report-nexus/internal/domain/daypart"
	"api-report-nexus/internal/domain/report"
)

// Input is the payload body; the four card/cash groups stay maps because their rows re-emit every input key.
type Input struct {
	NetSales                     float64               `json:"netSales"`
	SalesSummary                 Summary               `json:"salesSummary"`
	Liabilities                  Liabilities           `json:"liabilities"`
	PaymentSummary               Payments              `json:"paymentSummary"`
	ThirdParty                   ThirdParty            `json:"thirdParty"`
	SalesCategories              []Category            `json:"salesCategories"`
	SalesByItemTypes             []Category            `json:"salesByItemTypes"`
	SalesByDayPartByItemTypes    []DayPartLegacy       `json:"salesByDayPartByItemTypes"`
	DayPartSalesByItemTypes      []DayPartItemType     `json:"dayPartSalesByItemTypes"`
	SaleTypes                    []SaleType            `json:"saleTypes"`
	SaleTypesThirdPartyBreakdown []ThirdPartyBreakdown `json:"saleTypesThirdPartyBreakdown"`
	Taxes                        []Tax                 `json:"taxes"`
	Tips                         []Tip                 `json:"tips"`
	ServiceCharges               []ServiceCharge       `json:"serviceCharges"`
	VoidSummary                  Cancel                `json:"voidSummary"`
	VoidDetails                  []CancelDetail        `json:"voidDetails"`
	CancelsByReason              []Cancel              `json:"cancelsByReason"`
	CancelDetails                []CancelDetail        `json:"cancelDetails"`
	CompsByReason                []Cancel              `json:"compsByReason"`
	CompDetails                  []CancelDetail        `json:"compDetails"`
	Discounts                    []Discount            `json:"discounts"`
	CashSummary                  Cash                  `json:"cashSummary"`
	HourlyBreakdowns             []Breakdown           `json:"hourlyBreakdowns"`
	WeekdayBreakdowns            []Breakdown           `json:"weekdayBreakdowns"`
	PaidBalance                  PaidBalance           `json:"paidBalance"`
	LaborOverview                Labor                 `json:"laborOverview"`
	SalesByDayPartBreakdown      []daypart.Shift       `json:"salesByDayPartBreakdown"`
	SalesByCreditCard            report.Row            `json:"salesByCreditCard"`
	SalesByCash                  report.Row            `json:"salesByCash"`
	SalesByOnlineOrdering        report.Row            `json:"salesByOnlineOrdering"`
	SalesByQRCodeDineIn          report.Row            `json:"salesByQRCodeDineIn"`
	LabelConfigs                 []report.Config       `json:"labelConfigs"`
	TaxDetails                   []TaxDetail           `json:"taxDetails"`
	FeeTax                       *FeeTax               `json:"serviceChargesAndFeesTaxDetail"`
	CustomFee                    *CustomFee            `json:"customFeeSummary"`
}

type Summary struct {
	DineInSales, ToGoSales, OtherSales, AccountReceivables  float64
	ServiceCharges, Tips, Tax, Discounts                    float64
	TicketsCount, GuestsCount, AvgSales, AvgSalesByGuest    float64
	NetSaleHavingGuest                                      float64
	DineInSalesIncludeAR, DineInTicketCount                 float64
	DineInGuestsCount, DineInAvgSalesByTicket               float64
	DineInAvgSalesByGuest                                   float64
	QuickSalesIncludeAR, QuickGuestsCount, QuickTicketCount float64
	QuickAvgSalesByTicket, QuickAvgSalesByGuest             float64
	QuickNetSaleHavingGuest                                 float64
}

type ThirdParty struct {
	DoorDashSuccessfulDeliveryFee float64 `json:"doorDashSuccessfulDeliveryFee"`
	DoorDashFailedDeliveryFee     float64 `json:"doorDashFailedDeliveryFee"`
}

type Liabilities struct {
	GiftCardCount, GiftCardIssued, GiftCardTips, TotalGiftCardLiabilities float64
	GiftCardCredit, GiftCardCash, GiftCardOther                           float64
	DepositCount, DepositIssued, DepositTips, TotalDepositLiabilities     float64
	DepositCredit, DepositCash, DepositOther                              float64
	HouseAccountCount, HouseAccountIssued, HouseAccountTips               float64
	TotalHouseAccountLiabilities                                          float64
	HouseAccountCredit, HouseAccountCash, HouseAccountOther               float64
}

type Payment struct {
	TypeName                                                 string `json:"typeName"`
	Count, Amount, Tips, ServiceCharges, ThirdParty, Refunds float64
}

type Payments struct {
	PaymentTypes []Payment `json:"paymentTypes"`
	CreditTypes  []Payment `json:"creditTypes"`
	OtherTypes   []Payment `json:"otherTypes"`
}

type Category struct {
	Name string
	// present but empty still wins over Name
	NameAlias                                      *string `json:"nameAlias"`
	Quantity, NetSales, Discounts, GrossSales, Tax float64
}

type DayPartLegacy struct {
	ItemType                 string `json:"itemType"`
	Breakfast, Lunch, Dinner float64
}

type DayPartItemType struct {
	Name     string    `json:"name"`
	DayParts []DayPart `json:"dayParts"`
}

type DayPart struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	TotalSales float64 `json:"totalSales"`
}

type SaleType struct {
	SaleType                  string `json:"saleType"`
	OrderCount, NetSales, Tax float64
	// null means "derive from net + tax"
	TotalSales  *float64 `json:"totalSales"`
	ChannelType float64  `json:"channelType"`
}

type ThirdPartyBreakdown struct {
	SaleTypeName                                          string `json:"saleTypeName"`
	TicketCount, NetSales, Tax, NetSalesTaxAndPaidBalance float64
}

type Tax struct {
	Name                            string
	OrderCount, TaxAmount, NetSales float64
}

type Tip struct {
	Name                       string
	OrderCount, Tips, NetSales float64
}

type ServiceCharge struct {
	Name                                    string
	OrderCount, NetAmount, Tax, TotalAmount float64
}

type Cancel struct {
	Name                                         string
	Amount, OrderCount, ItemCount, ModifierCount float64
}

type CancelDetail struct {
	InvoiceNum string `json:"invoiceNum"`
	// string or number
	SaleReceiptNumber any    `json:"saleReceiptNumber"`
	Name              string `json:"name"`
	ItemCount, Amount float64
	EmployeeRequest   string `json:"employeeRequest"`
	EmployeeApproved  string `json:"employeeApproved"`
}

type Discount struct {
	Reason                         string
	Amount, TicketCount, ItemCount float64
}

type Cash struct {
	TotalCashPayments, CashIn, CashOut, CashBeforeTipouts float64
	CashGratuity, CashTip                                 float64
	CreditOrNonCashGratuity, CreditOrNonCashTips          float64
	TotalCash                                             float64
}

type Breakdown struct {
	Title                           string
	TicketCount, GuestsCount, Total float64
}

type PaidBalance struct {
	PaidBalance, TipPaidBalance float64
}

type Labor struct {
	LaborCost, NetSales, LaborCostPercent          float64
	AvgSalesByHours, AvgNetSalesPerEmployee        float64
	AvgLaborCostByHours, AvgHourlyWagesPerEmployee float64
}

type TaxDetail struct {
	TaxCodeName                                              string `json:"taxCodeName"`
	TaxRate, GrossSales, DiscountAmount, NetSales, TaxAmount float64
}

type CustomFee struct {
	Name              string `json:"name"`
	NetCustomFee, Tax float64
}

// FeeTax detail rows are kept as ordered objects: the report lists their fields in payload order.
type FeeTax struct {
	TotalTaxAmount   float64   `json:"totalTaxAmount"`
	TaxableDetails   []ordered `json:"taxableDetails"`
	NonTaxableDetail ordered   `json:"nonTaxableDetail"`
}

// ordered is a JSON object that keeps key order, standing in for Object.entries.
type ordered struct {
	keys []string
	m    map[string]any
}

func (o *ordered) UnmarshalJSON(b []byte) error {
	o.m = map[string]any{}
	dec := json.NewDecoder(bytes.NewReader(b))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		// null or non-object: leave empty
		return err
	}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return err
		}
		var v any
		if err := dec.Decode(&v); err != nil {
			return err
		}
		k, _ := key.(string)
		o.keys = append(o.keys, k)
		o.m[k] = v
	}
	return nil
}

func (o ordered) num(key string) float64 {
	f, _ := o.m[key].(float64)
	return f
}

func (o ordered) str(key string) string {
	s, _ := o.m[key].(string)
	return s
}
