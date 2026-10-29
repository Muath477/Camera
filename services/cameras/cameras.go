// Package cameras manages cameras, their place on the map, live view and PTZ.
package cameras

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Muath477/Camera/contract"
	"github.com/Muath477/Camera/services/audit"
	"github.com/Muath477/Camera/services/devices"
	"github.com/Muath477/Camera/services/platform"
)

type Handler struct {
	DB  *platform.DB
	Cmd devices.Commander

	thumbs sync.Map // deviceID/camera -> contract.Thumb
}

func (h *Handler) Routes(mux *http.ServeMux, wrap func(http.Handler) http.Handler) {
	mux.Handle("GET /v1/cameras", wrap(http.HandlerFunc(h.list)))
	mux.Handle("POST /v1/cameras", wrap(http.HandlerFunc(h.create)))
	mux.Handle("GET /v1/cameras/{id}", wrap(http.HandlerFunc(h.get)))
	mux.Handle("PATCH /v1/cameras/{id}/placement", wrap(http.HandlerFunc(h.place)))
	mux.Handle("DELETE /v1/cameras/{id}", wrap(http.HandlerFunc(h.remove)))
	mux.Handle("GET /v1/cameras/{id}/thumb.jpg", wrap(http.HandlerFunc(h.thumb)))
	mux.Handle("POST /v1/cameras/{id}/webrtc", wrap(http.HandlerFunc(h.webrtc)))
	mux.Handle("POST /v1/cameras/{id}/ptz", wrap(http.HandlerFunc(h.ptz)))
}

// Camera is the API view. Credentials are never part of it: they live only
// on the device.
type Camera struct {
	ID           string   `json:"id"`
	DeviceID     string   `json:"device_id"`
	SiteID       string   `json:"site_id"`
	Name         string   `json:"name"`
	DisplayName  string   `json:"display_name"`
	Vendor       string   `json:"vendor,omitempty"`
	Model        string   `json:"model,omitempty"`
	ProfileID    *string  `json:"profile_id,omitempty"`
	PTZ          bool     `json:"ptz"`
	Lat          *float64 `json:"lat,omitempty"`
	Lng          *float64 `json:"lng,omitempty"`
	FloorPlanID  *string  `json:"floor_plan_id,omitempty"`
	PlanX        *float32 `json:"plan_x,omitempty"`
	PlanY        *float32 `json:"plan_y,omitempty"`
	HeadingDeg   *float32 `json:"heading_deg,omitempty"`
	FOVDeg       *float32 `json:"fov_deg,omitempty"`
	Status       string   `json:"status"`
	Article2Zone *string  `json:"article2_zone,omitempty"`
}

const cameraColumns = `id, device_id, site_id, name, display_name, coalesce(vendor,''), coalesce(model,''), profile_id, ptz,
	ST_Y(geom::geometry), ST_X(geom::geometry), floor_plan_id::text, plan_x, plan_y, heading_deg, fov_deg, status, article2_zone`

func scanCamera(row pgx.Row) (Camera, error) {
	var c Camera
	err := row.Scan(&c.ID, &c.DeviceID, &c.SiteID, &c.Name, &c.DisplayName, &c.Vendor, &c.Model, &c.ProfileID, &c.PTZ,
		&c.Lat, &c.Lng, &c.FloorPlanID, &c.PlanX, &c.PlanY, &c.HeadingDeg, &c.FOVDeg, &c.Status, &c.Article2Zone)
	return c, err
}

// Load returns one camera within the tenant.
func Load(ctx context.Context, db *platform.DB, tenantID, id string) (Camera, error) {
	var c Camera
	err := db.InTenant(ctx, tenantID, func(tx pgx.Tx) (err error) {
		c, err = scanCamera(tx.QueryRow(ctx, `SELECT `+cameraColumns+` FROM cameras WHERE id = $1`, id))
		return err
	})
	return c, err
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	sites, all := p.SitesWith(platform.PermLiveView)
	siteFilter := r.URL.Query().Get("site_id")
	var out []Camera
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		rows, err := tx.Query(r.Context(), `SELECT `+cameraColumns+` FROM cameras
			WHERE ($1 OR site_id = ANY($2::uuid[])) AND ($3 = '' OR site_id::text = $3)
			ORDER BY display_name`, all, sites, siteFilter)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			c, err := scanCamera(rows)
			if err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	c, err := Load(r.Context(), h.DB, p.TenantID, r.PathValue("id"))
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	if !p.Can(platform.PermLiveView, c.SiteID) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	platform.JSON(w, http.StatusOK, c)
}

// create adds a camera: the device stores the credentials and starts
// recording; the cloud keeps only the description.
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	var in struct {
		DeviceID     string   `json:"device_id"`
		Name         string   `json:"name"`
		DisplayName  string   `json:"display_name"`
		Host         string   `json:"host"`
		Vendor       string   `json:"vendor"`
		Model        string   `json:"model"`
		ProfileID    string   `json:"profile_id"`
		Channel      int      `json:"channel"`
		Username     string   `json:"username"`
		Password     string   `json:"password"`
		RTSPMain     string   `json:"rtsp_main"`
		RTSPSub      string   `json:"rtsp_sub"`
		Audio        bool     `json:"audio"`
		PTZ          bool     `json:"ptz"`
		Article2Zone string   `json:"article2_zone"`
		Track        []string `json:"track"`
	}
	if err := platform.Decode(r, &in); err != nil || in.DeviceID == "" || in.Name == "" {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	if in.DisplayName == "" {
		in.DisplayName = in.Name
	}
	var siteID string
	err := h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		return tx.QueryRow(r.Context(), `SELECT site_id FROM devices WHERE id = $1 AND site_id IS NOT NULL`, in.DeviceID).Scan(&siteID)
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	if !p.Can(platform.PermCameraManage, siteID) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	ack, err := h.Cmd.Send(ctx, p.TenantID, in.DeviceID, contract.CmdAddCamera, map[string]any{
		"name": in.Name, "host": in.Host, "vendor": in.Vendor, "model": in.Model, "profile_id": in.ProfileID,
		"channel": in.Channel, "username": in.Username, "password": in.Password,
		"rtsp_main": in.RTSPMain, "rtsp_sub": in.RTSPSub, "audio": in.Audio, "ptz": in.PTZ, "track": in.Track,
	})
	if err != nil {
		platform.Fail(w, r, platform.Errorf(http.StatusGatewayTimeout, err.Error()))
		return
	}
	if !ack.OK {
		platform.Fail(w, r, platform.Errorf(http.StatusBadGateway, ack.Error))
		return
	}
	profile, _ := ack.Result["profile_id"].(string)

	var id string
	err = h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		err := tx.QueryRow(r.Context(), `
			INSERT INTO cameras (tenant_id, device_id, site_id, name, display_name, vendor, model, profile_id, ptz, article2_zone)
			VALUES ($1, $2, $3, $4, $5, nullif($6,''), nullif($7,''),
			        (SELECT id FROM camera_profiles WHERE id = $8), $9, nullif($10,''))
			ON CONFLICT (device_id, name) DO UPDATE SET display_name = excluded.display_name
			RETURNING id`,
			p.TenantID, in.DeviceID, siteID, in.Name, in.DisplayName, in.Vendor, in.Model, profile, in.PTZ, in.Article2Zone).Scan(&id)
		if err != nil {
			return err
		}
		return audit.Append(r.Context(), tx, p.TenantID, audit.FromRequest(r, "camera.add", "camera", id, map[string]any{"name": in.Name}))
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	platform.JSON(w, http.StatusCreated, map[string]string{"id": id, "profile_id": profile})
}

// place stores the camera's position, heading and field of view, which the
// map draws as a view cone.
func (h *Handler) place(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	c, err := Load(r.Context(), h.DB, p.TenantID, r.PathValue("id"))
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	if !p.Can(platform.PermCameraManage, c.SiteID) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	var in struct {
		Lat          *float64 `json:"lat"`
		Lng          *float64 `json:"lng"`
		FloorPlanID  *string  `json:"floor_plan_id"`
		PlanX        *float32 `json:"plan_x"`
		PlanY        *float32 `json:"plan_y"`
		HeadingDeg   *float32 `json:"heading_deg"`
		FOVDeg       *float32 `json:"fov_deg"`
		Article2Zone *string  `json:"article2_zone"`
		DisplayName  *string  `json:"display_name"`
	}
	if err := platform.Decode(r, &in); err != nil || (in.Lat == nil) != (in.Lng == nil) {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	err = h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		_, err := tx.Exec(r.Context(), `
			UPDATE cameras SET
			  geom = CASE WHEN $2::float8 IS NULL THEN geom ELSE ST_SetSRID(ST_MakePoint($2::float8, $3::float8), 4326)::geography END,
			  floor_plan_id = coalesce($4::uuid, floor_plan_id), plan_x = coalesce($5, plan_x), plan_y = coalesce($6, plan_y),
			  heading_deg = coalesce($7, heading_deg), fov_deg = coalesce($8, fov_deg),
			  article2_zone = coalesce($9, article2_zone), display_name = coalesce($10, display_name)
			WHERE id = $1`, c.ID, in.Lng, in.Lat, in.FloorPlanID, in.PlanX, in.PlanY, in.HeadingDeg, in.FOVDeg, in.Article2Zone, in.DisplayName)
		if err != nil {
			return err
		}
		return audit.Append(r.Context(), tx, p.TenantID, audit.FromRequest(r, "camera.place", "camera", c.ID, nil))
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	c, err := Load(r.Context(), h.DB, p.TenantID, r.PathValue("id"))
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	if !p.Can(platform.PermCameraManage, c.SiteID) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if ack, err := h.Cmd.Send(ctx, p.TenantID, c.DeviceID, contract.CmdRemoveCamera, map[string]any{"camera": c.Name}); err != nil || !ack.OK {
		platform.Fail(w, r, platform.Errorf(http.StatusBadGateway, "device did not remove the camera"))
		return
	}
	err = h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(r.Context(), `DELETE FROM cameras WHERE id = $1`, c.ID); err != nil {
			return err
		}
		return audit.Append(r.Context(), tx, p.TenantID, audit.FromRequest(r, "camera.remove", "camera", c.ID, map[string]any{"name": c.Name}))
	})
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// IngestThumb keeps the latest thumbnail per camera in memory. Thumbnails
// are never stored; in production this map is a Redis key with a short TTL.
func (h *Handler) IngestThumb(m devices.Message) {
	var t contract.Thumb
	if json.Unmarshal(m.Payload, &t) != nil {
		return
	}
	h.thumbs.Store(m.DeviceID+"/"+t.Camera, t)
}

func (h *Handler) thumb(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	c, err := Load(r.Context(), h.DB, p.TenantID, r.PathValue("id"))
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	if !p.Can(platform.PermLiveView, c.SiteID) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	v, ok := h.thumbs.Load(c.DeviceID + "/" + c.Name)
	if !ok || time.Since(v.(contract.Thumb).At) > 2*time.Minute {
		// Ask the device to start sending thumbnails for the next few minutes.
		go h.Cmd.Send(context.Background(), p.TenantID, c.DeviceID, contract.CmdThumbs, map[string]any{"minutes": 5})
		if !ok {
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=10")
	_, _ = w.Write(v.(contract.Thumb).JPEG)
}

// webrtc relays the browser's SDP offer to the device and returns its answer.
func (h *Handler) webrtc(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	c, err := Load(r.Context(), h.DB, p.TenantID, r.PathValue("id"))
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	if !p.Can(platform.PermLiveView, c.SiteID) {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	var in struct {
		SDP string `json:"sdp"`
	}
	if err := platform.Decode(r, &in); err != nil || in.SDP == "" {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	ack, err := h.Cmd.Send(ctx, p.TenantID, c.DeviceID, contract.CmdWebRTCOffer, map[string]any{"camera": c.Name, "sdp": in.SDP})
	if err != nil || !ack.OK {
		platform.Fail(w, r, platform.Errorf(http.StatusBadGateway, "device could not start the stream"))
		return
	}
	_ = h.DB.InTenant(r.Context(), p.TenantID, func(tx pgx.Tx) error {
		return audit.Append(r.Context(), tx, p.TenantID, audit.FromRequest(r, "live.view", "camera", c.ID, nil))
	})
	platform.JSON(w, http.StatusOK, map[string]any{"sdp": ack.Result["sdp"]})
}

var ptzActions = map[string]bool{
	"MOVE_LEFT": true, "MOVE_RIGHT": true, "MOVE_UP": true, "MOVE_DOWN": true,
	"ZOOM_IN": true, "ZOOM_OUT": true, "STOP": true,
}

func (h *Handler) ptz(w http.ResponseWriter, r *http.Request) {
	p := platform.PrincipalFrom(r.Context())
	c, err := Load(r.Context(), h.DB, p.TenantID, r.PathValue("id"))
	if err != nil {
		platform.Fail(w, r, err)
		return
	}
	if !p.Can(platform.PermPTZ, c.SiteID) || !c.PTZ {
		platform.Fail(w, r, platform.ErrForbidden)
		return
	}
	var in struct {
		Action string `json:"action"`
		Preset string `json:"preset"`
	}
	if err := platform.Decode(r, &in); err != nil {
		platform.Fail(w, r, platform.ErrBadInput)
		return
	}
	action := in.Action
	if in.Preset != "" {
		action = "preset_" + in.Preset
	} else if !ptzActions[action] {
		platform.Fail(w, r, platform.Errorf(http.StatusBadRequest, "unknown action"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	ack, err := h.Cmd.Send(ctx, p.TenantID, c.DeviceID, contract.CmdPTZ, map[string]any{"camera": c.Name, "action": action})
	if err != nil || !ack.OK {
		platform.Fail(w, r, platform.Errorf(http.StatusBadGateway, "ptz failed"))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
