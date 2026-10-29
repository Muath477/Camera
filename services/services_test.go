package services_test

import (
	"context"
	"crypto/ed25519"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Muath477/Camera/services/audit"
	"github.com/Muath477/Camera/services/compliance"
	"github.com/Muath477/Camera/services/events"
	"github.com/Muath477/Camera/services/media"
	"github.com/Muath477/Camera/services/platform"
)

func TestPermissions(t *testing.T) {
	p := &platform.Principal{Roles: map[string]string{"site-a": "operator", "site-b": "site_manager"}}
	if !p.Can(platform.PermLiveView, "site-a") || p.Can(platform.PermExport, "site-a") {
		t.Error("operator permissions wrong")
	}
	if p.Can(platform.PermLiveView, "site-c") || p.Can(platform.PermLiveView, "") {
		t.Error("site role leaked to other sites")
	}
	sites, all := p.SitesWith(platform.PermCameraManage)
	if all || len(sites) != 1 || sites[0] != "site-b" {
		t.Errorf("SitesWith = %v %v", sites, all)
	}
	owner := &platform.Principal{Roles: map[string]string{"": "owner"}}
	if !owner.Can(platform.PermExport, "any") {
		t.Error("owner should be tenant-wide")
	}
	auditor := &platform.Principal{Roles: map[string]string{"": "auditor"}}
	if auditor.Can(platform.PermLiveView, "any") {
		t.Error("auditor must not watch live video")
	}
}

func TestRuleMatches(t *testing.T) {
	score := float32(0.8)
	cam := "cam-1"
	ev := events.Event{SiteID: "s1", CameraID: &cam, Label: "person", Score: &score, Zones: []string{"door"}}
	r := events.Rule{Enabled: true, Conditions: events.Conditions{Labels: []string{"person"}, Zones: []string{"door"}, MinScore: 0.7}}
	if !r.Matches(ev, time.Now()) {
		t.Error("should match")
	}
	r.Conditions.MinScore = 0.9
	if r.Matches(ev, time.Now()) {
		t.Error("score below minimum")
	}
	r.Conditions.MinScore = 0
	r.Conditions.Schedule = &struct {
		FromHour int    `json:"from_hour"`
		ToHour   int    `json:"to_hour"`
		Timezone string `json:"timezone"`
	}{23, 6, "Asia/Riyadh"}
	night := time.Date(2026, 10, 29, 22, 30, 0, 0, time.UTC) // 01:30 in Riyadh
	day := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)     // 12:00 in Riyadh
	if !r.Matches(ev, night) || r.Matches(ev, day) {
		t.Error("schedule across midnight")
	}
}

func TestComplianceEvaluate(t *testing.T) {
	now := time.Now()
	seen := now.Add(-time.Minute)
	low := 40.0
	f := compliance.SiteFacts{
		Category: "restaurant", RetentionNeeded: 90, Now: now,
		Devices: []compliance.DeviceFact{{ID: "d", LastSeen: &seen, RetentionDays: &low}},
		Cameras: []compliance.CameraFact{{Name: "a", Status: "online", Zone: "entrance"}},
	}
	got := map[string]string{}
	for _, c := range compliance.Evaluate(f) {
		got[c.Type] = c.Status
	}
	want := map[string]string{"retention": "fail", "connectivity": "pass", "uptime": "pass", "coverage": "fail", "signage": "warn", "access_log": "pass"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %s, want %s", k, got[k], v)
		}
	}
	if s := compliance.Score(compliance.Evaluate(f)); s != 58 {
		t.Errorf("score = %d", s)
	}
}

func TestSnapshotSignature(t *testing.T) {
	pub, key, _ := ed25519.GenerateKey(nil)
	f := media.SignedFacts{Tenant: "t", Site: "s", Camera: "c", TakenAt: time.Unix(1730000000, 0).UTC(), SHA256: "ab", RequestedBy: "u"}
	sig := media.Sign(key, f)
	if !media.VerifySignature(pub, f, sig) {
		t.Fatal("valid signature rejected")
	}
	f.SHA256 = "cd"
	if media.VerifySignature(pub, f, sig) {
		t.Fatal("tampered facts accepted")
	}
}

// The tests below need a migrated database and a non-superuser login that is
// a member of camera_app, e.g.
// CAMERA_TEST_DATABASE_URL=postgres://camera_api:camera_api@localhost:5432/camera
func testDB(t *testing.T) *platform.DB {
	dsn := os.Getenv("CAMERA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("CAMERA_TEST_DATABASE_URL not set")
	}
	db, err := platform.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	return db
}

func newTenant(t *testing.T, db *platform.DB, ctx context.Context) string {
	var id string
	if err := db.Pool.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	err := db.InTenant(ctx, id, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO tenants (id, name) VALUES ($1, 'test')`, id); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO sites (tenant_id, name, facility_category) VALUES ($1, 'site', 'retail')`, id)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestTenantIsolation(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	a, b := newTenant(t, db, ctx), newTenant(t, db, ctx)

	var n int
	err := db.InTenant(ctx, b, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM sites WHERE tenant_id = $1`, a).Scan(&n)
	})
	if err != nil || n != 0 {
		t.Fatalf("tenant b sees %d sites of tenant a (err %v)", n, err)
	}
	err = db.InTenant(ctx, b, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO sites (tenant_id, name, facility_category) VALUES ($1, 'x', 'retail')`, a)
		return err
	})
	if err == nil {
		t.Fatal("tenant b wrote into tenant a")
	}
	if err := db.Pool.QueryRow(ctx, `SELECT count(*) FROM sites`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("no tenant set should see nothing, saw %d", n)
	}
}

func TestAuditChain(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	tenant := newTenant(t, db, ctx)
	err := db.InTenant(ctx, tenant, func(tx pgx.Tx) error {
		for _, action := range []string{"live.view", "snapshot.create", "recording.export"} {
			if err := audit.Append(ctx, tx, tenant, audit.Entry{ActorType: "user", ActorID: "u1", Action: action,
				Details: map[string]any{"camera": "c1"}}); err != nil {
				return err
			}
		}
		checked, broken, err := audit.Verify(ctx, tx, tenant)
		if err != nil {
			return err
		}
		if checked != 3 || broken != 0 {
			t.Errorf("checked=%d broken=%d", checked, broken)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = db.InTenant(ctx, tenant, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE audit_log SET action = 'nothing' WHERE tenant_id = $1`, tenant)
		return err
	})
	if err == nil {
		t.Fatal("audit rows must not be editable")
	}
}
