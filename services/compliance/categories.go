package compliance

// DefaultRetentionDays is used until the period in the ministry's terms
// document is confirmed for each category. Secondary sources mostly cite 90
// days; see docs/research/01-regulation.md. Sites can raise it per category.
const DefaultRetentionDays = 90

// Category is a facility category with the obligations the platform checks.
// Obliged categories come from Article 2 of the surveillance cameras law;
// the zones that need coverage come from Article 2 of its executive
// regulation. Both are reviewed against the official text before each release.
type Category struct {
	ID            string   `json:"id"`
	NameAR        string   `json:"name_ar"`
	Mandatory     bool     `json:"mandatory"`
	RetentionDays int      `json:"retention_days"`
	RequiredZones []string `json:"required_zones"` // article2 zone types that need coverage
}

// RequiredZone labels shown in the app.
var ZoneNamesAR = map[string]string{
	"entrance":   "المداخل والمخارج",
	"parking":    "المواقف",
	"cashier":    "منطقة المحاسبة",
	"atm":        "أجهزة الصرف",
	"warehouse":  "المستودع",
	"corridor":   "الممرات",
	"lobby":      "الاستقبال",
	"perimeter":  "السور الخارجي",
	"fuel_pumps": "مضخات الوقود",
	"vault":      "الخزنة",
	"elevator":   "المصاعد",
}

var categories = []Category{
	{ID: "hotel", NameAR: "فندق أو شقق مفروشة", Mandatory: true, RetentionDays: DefaultRetentionDays, RequiredZones: []string{"entrance", "lobby", "corridor", "parking", "elevator"}},
	{ID: "mall", NameAR: "مجمع تجاري", Mandatory: true, RetentionDays: DefaultRetentionDays, RequiredZones: []string{"entrance", "corridor", "parking"}},
	{ID: "retail", NameAR: "محل تجاري", Mandatory: true, RetentionDays: DefaultRetentionDays, RequiredZones: []string{"entrance", "cashier"}},
	{ID: "restaurant", NameAR: "مطعم أو مقهى", Mandatory: true, RetentionDays: DefaultRetentionDays, RequiredZones: []string{"entrance", "cashier"}},
	{ID: "fuel_station", NameAR: "محطة وقود", Mandatory: true, RetentionDays: DefaultRetentionDays, RequiredZones: []string{"entrance", "fuel_pumps", "cashier"}},
	{ID: "bank", NameAR: "مصرف أو محل صرافة", Mandatory: true, RetentionDays: DefaultRetentionDays, RequiredZones: []string{"entrance", "cashier", "atm", "vault"}},
	{ID: "jewelry", NameAR: "محل ذهب ومجوهرات", Mandatory: true, RetentionDays: DefaultRetentionDays, RequiredZones: []string{"entrance", "cashier", "vault"}},
	{ID: "health", NameAR: "منشأة صحية", Mandatory: true, RetentionDays: DefaultRetentionDays, RequiredZones: []string{"entrance", "lobby", "corridor", "parking"}},
	{ID: "education", NameAR: "منشأة تعليمية", Mandatory: true, RetentionDays: DefaultRetentionDays, RequiredZones: []string{"entrance", "perimeter", "corridor"}},
	{ID: "warehouse", NameAR: "مستودع", Mandatory: true, RetentionDays: DefaultRetentionDays, RequiredZones: []string{"entrance", "warehouse", "perimeter"}},
	{ID: "residential_compound", NameAR: "مجمع سكني", Mandatory: true, RetentionDays: DefaultRetentionDays, RequiredZones: []string{"entrance", "perimeter", "parking"}},
	{ID: "sports", NameAR: "منشأة رياضية أو ترفيهية", Mandatory: true, RetentionDays: DefaultRetentionDays, RequiredZones: []string{"entrance", "parking"}},
	{ID: "office", NameAR: "مكتب أو منشأة غير ملزمة", Mandatory: false, RetentionDays: 30, RequiredZones: []string{"entrance"}},
}

// Categories lists every category.
func Categories() []Category { return categories }

// CategoryByID returns the category or nil.
func CategoryByID(id string) *Category {
	for i := range categories {
		if categories[i].ID == id {
			return &categories[i]
		}
	}
	return nil
}
