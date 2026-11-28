package billing

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/go-pdf/fpdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/furkanmeclis/messora-app-go/internal/db"
)

// The proforma PDF (Go fonts, BSD licensed, cover Turkish characters), in
// Messora's brand: an Indigo header with the wordmark and a Mint accent,
// the seller and the buyer, the lines with VAT apart, the bank accounts
// and the payment reference to write in the transfer description.

var (
	indigo = [3]int{0x3B, 0x4C, 0xCA}
	mint   = [3]int{0x2D, 0xD4, 0xA7}
	night  = [3]int{0x0B, 0x12, 0x20}
	muted  = [3]int{0x4A, 0x54, 0x68}
	fog    = [3]int{0xF4, 0xF6, 0xFB}
)

// tl renders kuruş the Turkish way: 3.588,00 TL.
func tl(k int64) string {
	s := FormatKurus(k)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	whole, frac, _ := strings.Cut(s, ".")
	var b strings.Builder
	for i, r := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	out := b.String() + "," + frac + " TL"
	if neg {
		out = "-" + out
	}
	return out
}

func statusTR(status string) string {
	switch status {
	case "paid":
		return "Ödendi"
	case "partially_paid":
		return "Kısmen ödendi"
	case "void":
		return "İptal"
	}
	return "Ödeme bekliyor"
}

// WriteProformaPDF writes f as a one-page PDF.
func WriteProformaPDF(w io.Writer, f db.Proforma) error {
	var seller, buyer party
	_ = json.Unmarshal(f.Seller, &seller)
	_ = json.Unmarshal(f.Buyer, &buyer)
	var lines []line
	_ = json.Unmarshal(f.Lines, &lines)

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("go", "", goregular.TTF)
	pdf.AddUTF8FontFromBytes("go", "B", gobold.TTF)
	pdf.SetTitle("Proforma "+f.Number, true)
	pdf.SetCreator("Messora", true)
	pdf.SetMargins(16, 16, 16)
	pdf.SetAutoPageBreak(true, 16)
	pdf.AddPage()
	color := func(c [3]int) { pdf.SetTextColor(c[0], c[1], c[2]) }
	fill := func(c [3]int) { pdf.SetFillColor(c[0], c[1], c[2]) }

	// Header: Indigo band, the wordmark with its Mint dot, the title.
	fill(indigo)
	pdf.Rect(0, 0, 210, 30, "F")
	fill(mint)
	pdf.Rect(0, 30, 210, 1.5, "F")
	pdf.Circle(19.5, 15, 2.2, "F")
	pdf.SetXY(24, 9)
	pdf.SetFont("go", "B", 20)
	color([3]int{255, 255, 255})
	pdf.CellFormat(60, 12, "messora", "", 0, "L", false, 0, "")
	pdf.SetXY(110, 8)
	pdf.SetFont("go", "B", 14)
	pdf.CellFormat(84, 7, "PROFORMA FATURA", "", 2, "R", false, 0, "")
	pdf.SetFont("go", "", 9)
	pdf.CellFormat(84, 5, "Proforma invoice · "+f.Number, "", 2, "R", false, 0, "")
	pdf.CellFormat(84, 5, "Tarih "+dateTR(f.CreatedAt)+" · Son ödeme "+dateTR(f.DueAt), "", 2, "R", false, 0, "")

	// Seller and buyer.
	top := 40.0
	block := func(x float64, title string, p party) {
		pdf.SetXY(x, top)
		pdf.SetFont("go", "B", 8)
		color(muted)
		pdf.CellFormat(85, 5, title, "", 2, "L", false, 0, "")
		pdf.SetFont("go", "B", 10)
		color(night)
		pdf.MultiCell(85, 5, p.Name, "", "L", false)
		pdf.SetX(x)
		pdf.SetFont("go", "", 9)
		var rows []string
		if p.TaxOffice != "" || p.TaxNumber != "" {
			label := "VKN"
			if len(p.TaxNumber) == 11 {
				label = "TCKN"
			}
			rows = append(rows, strings.TrimSpace(strings.TrimPrefix(p.TaxOffice+" V.D. · ", " V.D. · ")+label+" "+p.TaxNumber))
		}
		if p.MersisNo != "" {
			rows = append(rows, "MERSİS "+p.MersisNo)
		}
		addr := p.Address
		if p.City != "" {
			addr += ", " + p.City
		}
		rows = append(rows, addr)
		if p.Email != "" {
			rows = append(rows, p.Email)
		}
		for _, r := range rows {
			pdf.SetX(x)
			pdf.MultiCell(85, 4.5, r, "", "L", false)
		}
	}
	block(16, "SATICI / SELLER", seller)
	block(110, "ALICI / BUYER", buyer)

	// Lines.
	pdf.SetXY(16, 86)
	fill(fog)
	color(muted)
	pdf.SetFont("go", "B", 8)
	pdf.CellFormat(110, 7, "AÇIKLAMA / DESCRIPTION", "", 0, "L", true, 0, "")
	pdf.CellFormat(16, 7, "ADET", "", 0, "R", true, 0, "")
	pdf.CellFormat(26, 7, "BİRİM FİYAT", "", 0, "R", true, 0, "")
	pdf.CellFormat(26, 7, "TUTAR", "", 1, "R", true, 0, "")
	color(night)
	for _, l := range lines {
		y := pdf.GetY()
		pdf.SetFont("go", "", 9)
		pdf.MultiCell(110, 5, l.DescriptionTR, "", "L", false)
		pdf.SetX(16)
		pdf.SetFont("go", "", 7.5)
		color(muted)
		pdf.MultiCell(110, 4, l.DescriptionEN, "", "L", false)
		color(night)
		end := pdf.GetY()
		pdf.SetXY(126, y)
		pdf.SetFont("go", "", 9)
		pdf.CellFormat(16, 5, fmt.Sprint(l.Quantity), "", 0, "R", false, 0, "")
		pdf.CellFormat(26, 5, tl(l.UnitKurus), "", 0, "R", false, 0, "")
		pdf.CellFormat(26, 5, tl(l.NetKurus), "", 0, "R", false, 0, "")
		pdf.SetXY(16, end+1.5)
	}
	pdf.Line(16, pdf.GetY(), 194, pdf.GetY())
	pdf.Ln(2)
	total := func(label, value string, bold bool) {
		style := ""
		if bold {
			style = "B"
		}
		pdf.SetX(110)
		pdf.SetFont("go", style, 9.5)
		pdf.CellFormat(58, 6, label, "", 0, "R", false, 0, "")
		pdf.CellFormat(26, 6, value, "", 1, "R", false, 0, "")
	}
	total("Ara toplam (KDV hariç) / Subtotal", tl(f.NetKurus), false)
	total("KDV %"+FormatRateBP(f.VatRateBp)+" / VAT", tl(f.VatKurus), false)
	total("Genel toplam (KDV dahil) / Total", tl(f.TotalKurus), true)
	if f.PaidKurus > 0 {
		total("Ödenen / Paid", tl(f.PaidKurus), false)
		total("Kalan / Due", tl(max(f.TotalKurus-f.PaidKurus, 0)), true)
	}

	// Payment: reference box and the bank accounts.
	pdf.Ln(4)
	y := pdf.GetY()
	fill(fog)
	pdf.Rect(16, y, 178, 22, "F")
	fill(mint)
	pdf.Rect(16, y, 1.5, 22, "F")
	pdf.SetXY(21, y+3)
	pdf.SetFont("go", "B", 8)
	color(muted)
	pdf.CellFormat(80, 4, "ÖDEME REFERANSI / PAYMENT REFERENCE", "", 2, "L", false, 0, "")
	pdf.SetFont("go", "B", 16)
	color(indigo)
	pdf.CellFormat(80, 9, DisplayReference(f.Reference), "", 2, "L", false, 0, "")
	pdf.SetXY(100, y+3)
	pdf.SetFont("go", "", 8.5)
	color(night)
	pdf.MultiCell(90, 4.2, "Havale/EFT açıklamasına bu referansı yazın; ödemeniz bu sayede eşleşir. "+
		"Write this reference in the transfer description so we can match your payment. Durum: "+statusTR(f.Status), "", "L", false)
	pdf.SetXY(16, y+26)
	pdf.SetFont("go", "B", 9)
	pdf.CellFormat(0, 6, "Banka hesapları / Bank accounts", "", 1, "L", false, 0, "")
	for _, b := range seller.Banks {
		pdf.SetFont("go", "B", 9)
		pdf.CellFormat(50, 5, b.BankName, "", 0, "L", false, 0, "")
		pdf.SetFont("go", "", 9)
		pdf.CellFormat(70, 5, FormatIBAN(b.IBAN), "", 0, "L", false, 0, "")
		pdf.CellFormat(0, 5, b.AccountHolder, "", 1, "L", false, 0, "")
	}
	pdf.Ln(3)
	pdf.SetFont("go", "", 8)
	color(muted)
	note := "Bu belge bir ödeme talebidir (proforma), mali değeri yoktur. Resmî e-Fatura / e-Arşiv faturanız ödeme tahsil edildiği gün TL olarak düzenlenir. " +
		"This is a request for payment, not a tax invoice; the official invoice is issued in TL on the day the payment is received."
	if seller.Note != "" {
		note += " " + seller.Note
	}
	pdf.MultiCell(0, 4, note, "", "L", false)
	return pdf.Output(w)
}
