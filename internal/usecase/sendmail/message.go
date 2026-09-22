package sendmail

import (
	"encoding/base64"
	"html/template"
	"strings"

	"api-report-nexus/internal/domain/email"
	"api-report-nexus/internal/pkg/datetime"
)

// Data URI prefixes the License API expects, as in the Node payloads.
const (
	excelURIPrefix = "data:application/vnd.openxmlformats-officedocument.spreadsheetml.sheet;charset=utf-8;base64,"
	pdfURIPrefix   = "data:application/pdf;base64,"
)

// Notes share this frame byte for byte with Node; only the paragraphs between differ.
const (
	noteHead = `<!DOCTYPE html>
        <html lang="en">
          <head>
            <meta charset="UTF-8">
            <meta http-equiv="X-UA-Compatible" content="IE=edge">
            <meta name="viewport" content="width=device-width, initial-scale=1.0">
          </head>
          <body>
            <p>Dear {{.Store}},</p>
`
	noteTail = `            <p>Should you have any questions or require further clarification regarding the report, please do not hesitate to reach out to us. We are here to assist you in any way possible.</p>
            <p>Thank you,</p>
            <p>BLogic Systems</p>
          </body>
        </html>`
)

// Unlike Node the templates escape the store name: an unescaped "&" or "<" would corrupt the HTML.
var bodyTemplate = template.Must(template.New("body").Parse("\n      " + noteHead +
	`            <p>Attached to this email, you will find the {{.Report}}{{.Period}}.</p>
` + noteTail))

func body(store, reportName, from, to string) (string, error) {
	period := ""
	if span := plainPeriod(from, to); span != "" {
		period = " covering the period from " + span
	}
	var out strings.Builder
	err := bodyTemplate.Execute(&out, struct{ Store, Report, Period string }{
		Store: storeOrCustomer(store), Report: reportName, Period: period,
	})
	return out.String(), err
}

func plainPeriod(from, to string) string {
	f, okF := datetime.ParseISO(from)
	t, okT := datetime.ParseISO(to)
	if !okF || !okT {
		return ""
	}
	const layout = "01/02/2006 03:04 PM"
	return f.Format(layout) + " to " + t.Format(layout)
}

func subjectPeriod(from, to string) string {
	if span := plainPeriod(from, to); span != "" {
		return " - From " + span
	}
	return ""
}

func fileStamp(from, to string) string {
	f, okF := datetime.ParseISO(from)
	t, okT := datetime.ParseISO(to)
	if !okF || !okT {
		return ""
	}
	const layout = "01-02-2006 03-04-PM"
	return " - From " + f.Format(layout) + " to " + t.Format(layout)
}

func storeOrCustomer(name string) string {
	if name == "" {
		return "Customer"
	}
	return name
}

func attachExcel(data []byte) string {
	return excelURIPrefix + base64.StdEncoding.EncodeToString(data)
}

func attachPDF(data []byte) string {
	return pdfURIPrefix + base64.StdEncoding.EncodeToString(data)
}

func excelMail(r Request, title, note, fileName string, data []byte) email.Message {
	return email.Message{
		ToEmails:    r.Emails,
		Subject:     r.Meta.StoreName + " - " + title + subjectPeriod(r.Meta.FromDate, r.Meta.ToDate),
		Body:        note,
		FileName:    fileName,
		FileContent: attachExcel(data),
	}
}
