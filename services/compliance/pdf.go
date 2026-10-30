package compliance

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/abdullahdiaa/garabic"
	"github.com/go-pdf/fpdf"
)

//go:embed fonts/IBMPlexSansArabic-Regular.ttf
var fontRegular []byte

//go:embed fonts/IBMPlexSansArabic-Bold.ttf
var fontBold []byte

// The PDF engine lays glyphs out left to right. garabic.Shape joins the
// Arabic letters into their contextual forms and returns the line in visual
// order: Arabic runs reversed, digits and Latin left as they are, so the
// result can be drawn as-is.
func visual(s string) string {
	return garabic.Shape(s)
}

type doc struct {
	pdf   *fpdf.Fpdf
	left  float64
	right float64
	width float64
}

func newDoc() *doc {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(16, 16, 16)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddUTF8FontFromBytes("plex", "", fontRegular)
	pdf.AddUTF8FontFromBytes("plex", "B", fontBold)
	w, _ := pdf.GetPageSize()
	return &doc{pdf: pdf, left: 16, right: w - 16, width: w - 32}
}

// text writes one right-aligned Arabic line whose right edge is at x.
func (d *doc) text(x, y, size float64, style, s string) {
	d.pdf.SetFont("plex", style, size)
	v := visual(s)
	w := d.pdf.GetStringWidth(v)
	d.pdf.Text(x-w, y, v)
}

// wrap breaks logical text into lines that fit width, measuring each in its
// visual form so the right words go together.
func (d *doc) wrap(s string, size float64, style string, width float64) []string {
	d.pdf.SetFont("plex", style, size)
	words := strings.Fields(s)
	var lines []string
	cur := ""
	for _, w := range words {
		try := strings.TrimSpace(cur + " " + w)
		if cur != "" && d.pdf.GetStringWidth(visual(try)) > width {
			lines = append(lines, cur)
			cur = w
			continue
		}
		cur = try
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

func (d *doc) paragraph(x, y, width, size, lineH float64, style, s string) float64 {
	for _, line := range d.wrap(s, size, style, width) {
		d.text(x, y, size, style, line)
		y += lineH
	}
	return y
}

var statusAR = map[string]string{Pass: "مستوفى", Warn: "تنبيه", Fail: "غير مستوفى"}

func statusColor(s string) (int, int, int) {
	switch s {
	case Pass:
		return 11, 127, 88
	case Warn:
		return 180, 83, 9
	}
	return 190, 18, 60
}

// RenderPDF lays the readiness report out as a printable Arabic document the
// owner hands to the inspector.
func RenderPDF(rep *Report) ([]byte, error) {
	d := newDoc()
	pdf := d.pdf
	pdf.SetTitle("تقرير جاهزية الامتثال — "+rep.SiteName, true)
	pdf.SetAuthor("Camera", true)
	pdf.AddPage()

	// Header band.
	pdf.SetFillColor(11, 16, 22)
	pdf.Rect(0, 0, 210, 34, "F")
	pdf.SetTextColor(255, 255, 255)
	d.text(d.right, 15, 17, "B", "تقرير جاهزية الامتثال")
	pdf.SetTextColor(170, 184, 196)
	d.text(d.right, 24, 9.5, "", "نظام استخدام كاميرات المراقبة الأمنية")
	pdf.SetFont("plex", "B", 12)
	pdf.SetTextColor(255, 255, 255)
	pdf.Text(d.left, 16, "Camera")
	pdf.SetFont("plex", "", 8)
	pdf.SetTextColor(170, 184, 196)
	pdf.Text(d.left, 23, rep.Generated.In(riyadh).Format("2006-01-02 15:04"))

	// Score ring on the left, facts on the right.
	y := 46.0
	cx, cy, rr := d.left+18, y+12, 14.0
	pdf.SetLineWidth(3.2)
	pdf.SetDrawColor(226, 232, 236)
	pdf.Circle(cx, cy, rr, "D")
	r, g, b := 190, 18, 60
	if rep.Score >= 80 {
		r, g, b = 15, 159, 110
	} else if rep.Score >= 50 {
		r, g, b = 217, 119, 6
	}
	pdf.SetDrawColor(r, g, b)
	if rep.Score > 0 {
		pdf.Arc(cx, cy, rr, rr, 0, 90-float64(rep.Score)*3.6, 90, "D")
	}
	pdf.SetTextColor(r, g, b)
	pdf.SetFont("plex", "B", 13)
	score := fmt.Sprintf("%d%%", rep.Score)
	pdf.Text(cx-pdf.GetStringWidth(score)/2, cy+2, score)
	pdf.SetTextColor(100, 116, 128)
	d.text(cx+rr+2, cy+22, 8, "", "نسبة الجاهزية")

	pdf.SetTextColor(11, 16, 22)
	d.text(d.right, y+2, 14, "B", rep.SiteName)
	rows := [][2]string{}
	if rep.Category != nil {
		rows = append(rows, [2]string{"الفئة", rep.Category.NameAR})
		rows = append(rows, [2]string{"مدة الحفظ المطلوبة", fmt.Sprintf("%d يوماً", rep.Category.RetentionDays)})
	}
	rows = append(rows, [2]string{"تاريخ التقرير", fmtDate(rep.Generated)})
	ry := y + 10
	for _, row := range rows {
		pdf.SetTextColor(100, 116, 128)
		d.text(d.right, ry, 9, "", row[0])
		pdf.SetTextColor(11, 16, 22)
		d.text(d.right-42, ry, 9.5, "B", row[1])
		ry += 6.5
	}

	// Checks table: item | status | detail, right to left.
	y = ry + 10
	colItem, colStatus := 46.0, 24.0
	colDetail := d.width - colItem - colStatus
	pdf.SetFillColor(244, 246, 248)
	pdf.Rect(d.left, y-5.5, d.width, 8, "F")
	pdf.SetTextColor(100, 116, 128)
	d.text(d.right-2, y, 8.5, "B", "البند")
	d.text(d.right-colItem-2, y, 8.5, "B", "الحالة")
	d.text(d.right-colItem-colStatus-2, y, 8.5, "B", "التفاصيل")
	y += 8
	for _, c := range rep.Checks {
		lines := d.wrap(c.DetailAR, 9, "", colDetail-4)
		h := float64(max(len(lines), 1))*5.2 + 4
		if y+h > 275 {
			pdf.AddPage()
			y = 20
		}
		pdf.SetTextColor(11, 16, 22)
		d.text(d.right-2, y+1, 9.5, "B", c.TitleAR)
		sr, sg, sb := statusColor(c.Status)
		pdf.SetFillColor(sr, sg, sb)
		pdf.Circle(d.right-colItem-4, y-0.6, 1.4, "F")
		pdf.SetTextColor(sr, sg, sb)
		d.text(d.right-colItem-7, y+1, 9, "B", statusAR[c.Status])
		pdf.SetTextColor(55, 65, 81)
		ly := y + 1
		for _, line := range lines {
			d.text(d.right-colItem-colStatus-2, ly, 9, "", line)
			ly += 5.2
		}
		y += h
		pdf.SetDrawColor(232, 236, 240)
		pdf.SetLineWidth(0.2)
		pdf.Line(d.left, y-2.5, d.right, y-2.5)
	}

	// Footer note.
	y += 6
	pdf.SetTextColor(100, 116, 128)
	y = d.paragraph(d.right, y, d.width, 8.5, 5, "",
		"هذا التقرير مولّد من المنصة بناءً على حالة الأجهزة والكاميرات وقت إصداره. سجل الوصول والأعطال محفوظ بسلسلة تجزئة ويمكن تقديمه عند الطلب.")
	pdf.SetFont("plex", "", 7.5)
	pdf.Text(d.left, 288, "Camera · "+rep.SiteID)
	d.text(d.right, 288, 7.5, "", fmt.Sprintf("صفحة %d", pdf.PageNo()))

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var riyadh = time.FixedZone("AST", 3*3600)

var monthsAR = [...]string{"يناير", "فبراير", "مارس", "أبريل", "مايو", "يونيو", "يوليو", "أغسطس", "سبتمبر", "أكتوبر", "نوفمبر", "ديسمبر"}

func fmtDate(t time.Time) string {
	t = t.In(riyadh)
	return fmt.Sprintf("%d %s %d، %02d:%02d", t.Day(), monthsAR[t.Month()-1], t.Year(), t.Hour(), t.Minute())
}

// FileName is an ASCII-safe attachment name; the Arabic title lives inside.
func FileName(rep *Report) string {
	name := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return '-'
	}, rep.SiteID[:8])
	return "compliance-" + name + "-" + rep.Generated.In(riyadh).Format("20060102") + ".pdf"
}
