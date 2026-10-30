package compliance

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestVisualKeepsNumbersReadable(t *testing.T) {
	// Visual order reverses Arabic runs but leaves digits and Latin in place,
	// so "61 يوماً" prints with 61 still reading as sixty-one.
	got := visual("تكفي 61 يوماً بنسبة 90%")
	for _, want := range []string{"61", "90%"} {
		if !strings.Contains(got, want) {
			t.Errorf("visual lost %q in %q", want, got)
		}
	}
	if strings.Contains(got, "16") || strings.Contains(got, "09") {
		t.Errorf("digits were reversed: %q", got)
	}
	// Drawn left to right, the end of the sentence comes first.
	if strings.Index(got, "90%") > strings.Index(got, "61") {
		t.Errorf("runs were not reversed for right-to-left reading: %q", got)
	}
}

func TestVisualParentheses(t *testing.T) {
	// Reversed, the closing bracket is drawn first (on the left) and the
	// opening one last (on the right), which is how the eye expects them.
	got := visual("(تجريبي)")
	if !strings.HasPrefix(got, ")") || !strings.HasSuffix(got, "(") {
		t.Errorf("parentheses out of place: %q", got)
	}
}

func TestRenderPDF(t *testing.T) {
	rep := &Report{
		SiteID: "0dd90dcc-fc4b-4e5b-8233-8cd125d60680", SiteName: "فرع العليا",
		Category: CategoryByID("restaurant"), Generated: time.Now(), Score: 67,
		Checks: []CheckResult{
			{Type: "retention", TitleAR: "مدة حفظ التسجيلات", Status: Fail, DetailAR: "السعة الحالية تكفي 61 يوماً والمطلوب 90 يوماً"},
			{Type: "signage", TitleAR: "لوحات التنبيه", Status: Warn, DetailAR: "لم يُرفع إثبات وجود اللوحات"},
			{Type: "cameras", TitleAR: "تشغيل الكاميرات", Status: Pass, DetailAR: "كل الكاميرات تعمل"},
		},
	}
	out, err := RenderPDF(rep)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) || len(out) < 10_000 {
		t.Fatalf("unexpected output: %d bytes", len(out))
	}
	if name := FileName(rep); !strings.HasPrefix(name, "compliance-0dd90dcc-") || !strings.HasSuffix(name, ".pdf") {
		t.Errorf("file name %q", name)
	}
}
