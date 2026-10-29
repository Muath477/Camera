package platform

// Permission is an action a role may take.
type Permission string

const (
	PermTenantManage   Permission = "tenant.manage"
	PermSiteManage     Permission = "site.manage"
	PermCameraManage   Permission = "camera.manage"
	PermDeviceManage   Permission = "device.manage"
	PermLiveView       Permission = "live.view"
	PermRecordingView  Permission = "recording.view"
	PermEventsView     Permission = "events.view"
	PermSnapshotCreate Permission = "snapshot.create"
	PermExport         Permission = "export"
	PermAlertAck       Permission = "alert.ack"
	PermAlertManage    Permission = "alert.manage"
	PermPTZ            Permission = "ptz"
	PermComplianceView Permission = "compliance.view"
	PermAuditView      Permission = "audit.view"
)

var rolePerms = map[string][]Permission{
	"owner": {
		PermTenantManage, PermSiteManage, PermCameraManage, PermDeviceManage, PermLiveView,
		PermRecordingView, PermEventsView, PermSnapshotCreate, PermExport, PermAlertAck,
		PermAlertManage, PermPTZ, PermComplianceView, PermAuditView,
	},
	"site_manager": {
		PermCameraManage, PermDeviceManage, PermLiveView, PermRecordingView, PermEventsView,
		PermSnapshotCreate, PermAlertAck, PermAlertManage, PermPTZ, PermComplianceView,
	},
	"operator":   {PermLiveView, PermEventsView, PermSnapshotCreate, PermAlertAck, PermPTZ},
	"viewer":     {PermLiveView},
	"auditor":    {PermComplianceView, PermAuditView},
	"integrator": {PermCameraManage, PermDeviceManage, PermLiveView},
}

// RoleAllows reports whether a role grants a permission.
func RoleAllows(role string, perm Permission) bool {
	for _, p := range rolePerms[role] {
		if p == perm {
			return true
		}
	}
	return false
}

// Can reports whether the caller may take perm on a site. An empty siteID
// asks about the tenant as a whole and needs a tenant-wide role.
func (p *Principal) Can(perm Permission, siteID string) bool {
	if p == nil {
		return false
	}
	if RoleAllows(p.Roles[""], perm) {
		return true
	}
	if siteID == "" {
		return false
	}
	return RoleAllows(p.Roles[siteID], perm)
}

// SitesWith returns the sites where the caller holds perm, and whether the
// permission is tenant-wide (in which case the list is empty and every site
// qualifies).
func (p *Principal) SitesWith(perm Permission) (sites []string, all bool) {
	if RoleAllows(p.Roles[""], perm) {
		return nil, true
	}
	for site, role := range p.Roles {
		if site != "" && RoleAllows(role, perm) {
			sites = append(sites, site)
		}
	}
	return sites, false
}
