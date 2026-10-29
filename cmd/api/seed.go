package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Muath477/Camera/services/audit"
	"github.com/Muath477/Camera/services/devices"
	"github.com/Muath477/Camera/services/platform"
)

// seed loads a demo tenant for local development: three branches of a
// restaurant chain with cameras placed on the map and a few days of events.
// It prints a development token for the owner.
func seed(ctx context.Context, db *platform.DB, log *slog.Logger) error {
	const tenantID = "11111111-1111-4111-8111-111111111111"
	const ownerSub = "dev-owner"

	type cam struct {
		name, display, zone string
		dLat, dLng, heading float64
	}
	sites := []struct {
		name, category, address string
		lat, lng                float64
		cams                    []cam
	}{
		{"فرع العليا", "restaurant", "طريق العليا، الرياض", 24.6948, 46.6853, []cam{
			{"entrance", "المدخل الرئيسي", "entrance", 0.00012, 0.00005, 200},
			{"cashier", "الكاشير", "cashier", -0.00004, 0.00010, 90},
			{"kitchen", "المطبخ", "", -0.00010, -0.00008, 320},
			{"parking", "المواقف", "parking", 0.00020, -0.00015, 160},
		}},
		{"فرع التحلية", "restaurant", "شارع التحلية، جدة", 21.5533, 39.1520, []cam{
			{"entrance", "المدخل", "entrance", 0.00010, 0.00004, 240},
			{"cashier", "الكاشير", "cashier", -0.00005, 0.00009, 60},
		}},
		{"مستودع الدمام", "warehouse", "المدينة الصناعية الثانية، الدمام", 26.3570, 50.0888, []cam{
			{"gate", "البوابة", "entrance", 0.00030, 0.00010, 180},
			{"dock", "رصيف التحميل", "warehouse", -0.00010, 0.00025, 90},
			{"fence_north", "السور الشمالي", "perimeter", 0.00045, -0.00020, 0},
		}},
	}

	labels := []string{"person", "person", "person", "car", "car", "motorcycle", "glass", "fire_alarm"}

	err := db.InTenant(ctx, tenantID, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM tenants WHERE id = $1)`, tenantID).Scan(&exists); err != nil {
			return err
		}
		if exists {
			log.Info("demo tenant already present")
			return nil
		}
		if _, err := tx.Exec(ctx, `INSERT INTO tenants (id, name, cr_number, plan) VALUES ($1, 'مطاعم النخلة', '1010123456', 'pro')`, tenantID); err != nil {
			return err
		}
		var ownerID string
		if err := tx.QueryRow(ctx, `INSERT INTO users (tenant_id, keycloak_sub, name, phone) VALUES ($1, $2, 'معاذ', '+966500000000') RETURNING id`,
			tenantID, ownerSub).Scan(&ownerID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO memberships (tenant_id, user_id, role) VALUES ($1, $2, 'owner')`, tenantID, ownerID); err != nil {
			return err
		}

		now := time.Now().UTC()
		for i, s := range sites {
			var siteID, deviceID string
			if err := tx.QueryRow(ctx, `INSERT INTO sites (tenant_id, name, facility_category, address, geom, retention_days_required)
				VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint($5, $6), 4326)::geography, 90) RETURNING id`,
				tenantID, s.name, s.category, s.address, s.lng, s.lat).Scan(&siteID); err != nil {
				return err
			}
			retention := []float64{104, 61, 97}[i]
			// Fixed ids so the device simulator can be pointed at them.
			if err := tx.QueryRow(ctx, `INSERT INTO devices (id, tenant_id, site_id, serial, hw_model, fw_version, status, last_seen_at, disk_total, disk_used, retention_days)
				VALUES ($1, $2, $3, $4, 'N100-16G', '0.1.0', 'active', now(), 8000000000000, 5200000000000, $5) RETURNING id`,
				fmt.Sprintf("22222222-2222-4222-8222-%012d", i+1), tenantID, siteID, fmt.Sprintf("CAM-DEMO-%03d", i+1), retention).Scan(&deviceID); err != nil {
				return err
			}
			for j, c := range s.cams {
				status := "online"
				if i == 1 && j == 1 {
					status = "offline"
				}
				var camID string
				if err := tx.QueryRow(ctx, `INSERT INTO cameras (tenant_id, device_id, site_id, name, display_name, vendor, profile_id, geom, heading_deg, fov_deg, status, article2_zone)
					VALUES ($1, $2, $3, $4, $5, 'hikvision', NULL, ST_SetSRID(ST_MakePoint($6, $7), 4326)::geography, $8, 90, $9, nullif($10,'')) RETURNING id`,
					tenantID, deviceID, siteID, c.name, c.display, s.lng+c.dLng, s.lat+c.dLat, c.heading, status, c.zone).Scan(&camID); err != nil {
					return err
				}
				for k := 0; k < 30; k++ {
					t := now.Add(-time.Duration(rand.IntN(72*3600)) * time.Second)
					label := labels[rand.IntN(len(labels))]
					kind := "object"
					sev := "info"
					if label == "glass" || label == "fire_alarm" {
						kind, sev = "audio", "critical"
					}
					end := t.Add(time.Duration(5+rand.IntN(60)) * time.Second)
					if _, err := tx.Exec(ctx, `INSERT INTO events (time, tenant_id, site_id, device_id, camera_id, edge_id, kind, label, score, zones, end_time, severity)
						VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
						t, tenantID, siteID, deviceID, camID, fmt.Sprintf("%d.%d-%s", t.Unix(), k, c.name), kind, label,
						0.6+rand.Float64()*0.38, []string{c.zone}, end, sev); err != nil {
						return err
					}
				}
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO alert_rules (tenant_id, name, conditions, severity, channels)
			VALUES ($1, 'شخص بعد الإغلاق', '{"labels":["person"],"schedule":{"from_hour":1,"to_hour":6,"timezone":"Asia/Riyadh"}}', 'warning', '{push,sms}'),
			       ($1, 'كسر زجاج أو إنذار حريق', '{"labels":["glass","fire_alarm"]}', 'critical', '{push,sms,whatsapp}')`, tenantID); err != nil {
			return err
		}
		return audit.Append(ctx, tx, tenantID, audit.Entry{ActorType: "system", ActorID: "seed", Action: "tenant.seed"})
	})
	if err != nil {
		return err
	}

	// A device in stock, ready to be claimed with code 123456.
	_, err = db.Pool.Exec(ctx, `INSERT INTO device_inventory (serial, hw_model, claim_code_hash) VALUES ('CAM-STOCK-001', 'N100-16G', $1)
		ON CONFLICT DO NOTHING`, devices.HashClaimCode("123456"))
	if err != nil {
		log.Warn("inventory seed skipped (needs the migration role)", "err", err)
	}

	secret := os.Getenv("DEV_JWT_SECRET")
	if secret == "" {
		fmt.Println("set DEV_JWT_SECRET to print a development token")
		return nil
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": ownerSub, "tenant_id": tenantID, "exp": time.Now().Add(30 * 24 * time.Hour).Unix(),
	})
	signed, err := tok.SignedString([]byte(secret))
	if err != nil {
		return err
	}
	fmt.Println(signed)
	return nil
}
