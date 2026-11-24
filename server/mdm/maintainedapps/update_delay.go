package maintained_apps

import (
	"context"
	"strings"
	"time"

	"github.com/fleetdm/fleet/v4/server/contexts/ctxerr"
	"github.com/fleetdm/fleet/v4/server/fleet"
)

// VersionMatchesMajor reports whether version's leading dot-separated segment
// equals majorVersion. Versions aren't reliably semver, so it is compared as a
// string.
func VersionMatchesMajor(version string, majorVersion string) bool {
	versionMajor, _, _ := strings.Cut(version, ".")
	return versionMajor == majorVersion
}

// UpdateDelayDuration converts a delay in days to a duration.
func UpdateDelayDuration(days uint) time.Duration {
	return time.Duration(days) * 24 * time.Hour
}

// VersionEligible reports whether v has been cached for at least delay as of
// now, so it may be promoted to the active version. With no delay every version
// is eligible: first_seen_at is the database's clock and can read later than
// now, so it must not gate a version that has nothing to wait for.
func VersionEligible(v fleet.FleetMaintainedVersion, delay time.Duration, now time.Time) bool {
	if delay <= 0 {
		return true
	}
	return !v.FirstSeenAt.After(now.Add(-delay))
}

// PendingVersion returns the newest version the pin allows that is newer than
// the active one and hasn't yet reached its delay. versions must be ordered most
// recently downloaded first. A literal pin never advances, so it has no pending
// version.
func PendingVersion(versions []fleet.FleetMaintainedVersion, activeID uint, pin string, delay time.Duration, now time.Time) (fleet.FleetMaintainedVersion, bool) {
	if pin != "" && !strings.HasPrefix(pin, "^") {
		return fleet.FleetMaintainedVersion{}, false
	}
	newer := versions
	for i, v := range versions {
		if v.ID == activeID {
			newer = versions[:i]
			break
		}
	}
	major := strings.TrimPrefix(pin, "^")
	for _, v := range newer {
		if pin != "" && !VersionMatchesMajor(v.Version, major) {
			continue
		}
		if !VersionEligible(v, delay, now) {
			return v, true
		}
	}
	return fleet.FleetMaintainedVersion{}, false
}

// UpdateDelayResolver resolves the effective update delay for an app, caching
// the org and per-fleet settings it reads. Build one per cron run or request.
type UpdateDelayResolver struct {
	ds       fleet.Datastore
	org      *fleet.FleetMaintainedAppSettings
	byTeamID map[uint]fleet.FleetMaintainedAppSettings
}

// NewUpdateDelayResolver returns a resolver backed by ds.
func NewUpdateDelayResolver(ds fleet.Datastore) *UpdateDelayResolver {
	return &UpdateDelayResolver{ds: ds, byTeamID: map[uint]fleet.FleetMaintainedAppSettings{}}
}

// Resolve returns the effective delay in days and where it came from. A nil
// teamID or 0 means Unassigned. appDays is the per-app delay, or nil when unset.
func (r *UpdateDelayResolver) Resolve(ctx context.Context, teamID *uint, appDays *uint) (uint, fleet.FMAUpdateDelaySource, error) {
	if appDays != nil {
		days, source := fleet.ResolveFMAUpdateDelay(appDays, fleet.FleetMaintainedAppSettings{}, fleet.FleetMaintainedAppSettings{})
		return days, source, nil
	}

	fleetSettings, err := r.fleetSettings(ctx, teamID)
	if err != nil {
		return 0, "", err
	}
	orgSettings, err := r.orgSettings(ctx)
	if err != nil {
		return 0, "", err
	}
	days, source := fleet.ResolveFMAUpdateDelay(nil, fleetSettings, orgSettings)
	return days, source, nil
}

func (r *UpdateDelayResolver) orgSettings(ctx context.Context) (fleet.FleetMaintainedAppSettings, error) {
	if r.org == nil {
		cfg, err := r.ds.AppConfig(ctx)
		if err != nil {
			return fleet.FleetMaintainedAppSettings{}, ctxerr.Wrap(ctx, err, "getting app config for update delay")
		}
		r.org = &cfg.FleetMaintainedAppSettings
	}
	return *r.org, nil
}

func (r *UpdateDelayResolver) fleetSettings(ctx context.Context, teamID *uint) (fleet.FleetMaintainedAppSettings, error) {
	var id uint
	if teamID != nil {
		id = *teamID
	}
	if s, ok := r.byTeamID[id]; ok {
		return s, nil
	}

	var s fleet.FleetMaintainedAppSettings
	if id == 0 {
		cfg, err := r.ds.DefaultTeamConfig(ctx)
		if err != nil {
			return s, ctxerr.Wrap(ctx, err, "getting unassigned config for update delay")
		}
		s = cfg.FleetMaintainedAppSettings
	} else {
		team, err := r.ds.TeamLite(ctx, id)
		if err != nil {
			return s, ctxerr.Wrap(ctx, err, "getting fleet for update delay")
		}
		s = team.Config.FleetMaintainedAppSettings
	}
	r.byTeamID[id] = s
	return s, nil
}
