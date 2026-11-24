package maintained_apps

import (
	"context"
	"testing"
	"time"

	"github.com/fleetdm/fleet/v4/pkg/optjson"
	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/fleetdm/fleet/v4/server/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionMatchesMajor(t *testing.T) {
	// Versions taken from ee/maintained-apps/outputs; most are not valid semver. The leading dot-segment is
	// compared as a string, so a leading-zero or bare major stays distinct from "2"/"10"/"12".
	cases := []struct {
		version      string
		majorVersion string
		want         bool
	}{
		{"149.1.91.172", "149", true},
		{"149.1.91.172", "150", false},
		{"149.1.91.172", "14", false},
		{"6.0.4.11438", "6", true},
		{"25.0.208.0", "25", true},
		{"221.0.0.0.0", "221", true},
		{"0.2026.06.10.09.27.01", "0", true},
		{"8.0.47.CE", "8", true},
		{"2.2.18d", "2", true},
		{"1.2.92.148.g882cc571", "1", true},
		{"114.0.4-release.20250509.32955", "114", true},
		{"2026.05.0+218", "2026", true},
		{"02.07.01.62", "02", true},
		{"02.07.01.62", "2", false},
		{"20250302", "20250302", true},
		{"183", "183", true},
		{"149", "149", true},
		{"1.21b", "1", true},
		{"10.0.1", "1", false},
		{"12.0", "1", false},
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, VersionMatchesMajor(c.version, c.majorVersion), "version %q caret ^%s", c.version, c.majorVersion)
	}
}

func TestUpdateDelayResolver(t *testing.T) {
	setting := func(days int) fleet.FleetMaintainedAppSettings {
		var s fleet.FleetMaintainedAppSettings
		s.UpdateDelayDays = optjson.SetInt(days)
		return s
	}
	teamID := uint(3)

	newDS := func(org, team, unassigned fleet.FleetMaintainedAppSettings) *mock.Store {
		ds := new(mock.Store)
		ds.AppConfigFunc = func(ctx context.Context) (*fleet.AppConfig, error) {
			return &fleet.AppConfig{FleetMaintainedAppSettings: org}, nil
		}
		ds.TeamLiteFunc = func(ctx context.Context, id uint) (*fleet.TeamLite, error) {
			return &fleet.TeamLite{ID: id, Config: fleet.TeamConfigLite{FleetMaintainedAppSettings: team}}, nil
		}
		ds.DefaultTeamConfigFunc = func(ctx context.Context) (*fleet.TeamConfig, error) {
			return &fleet.TeamConfig{FleetMaintainedAppSettings: unassigned}, nil
		}
		return ds
	}
	ctx := context.Background()

	t.Run("app beats fleet", func(t *testing.T) {
		r := NewUpdateDelayResolver(newDS(setting(7), setting(5), setting(5)))
		days, src, err := r.Resolve(ctx, &teamID, new(uint(2)))
		require.NoError(t, err)
		assert.Equal(t, uint(2), days)
		assert.Equal(t, fleet.FMAUpdateDelaySourceApp, src)
	})

	t.Run("fleet zero beats org", func(t *testing.T) {
		r := NewUpdateDelayResolver(newDS(setting(7), setting(0), fleet.FleetMaintainedAppSettings{}))
		days, src, err := r.Resolve(ctx, &teamID, nil)
		require.NoError(t, err)
		assert.Equal(t, uint(0), days)
		assert.Equal(t, fleet.FMAUpdateDelaySourceFleet, src)
	})

	t.Run("unset falls back to org", func(t *testing.T) {
		r := NewUpdateDelayResolver(newDS(setting(7), fleet.FleetMaintainedAppSettings{}, fleet.FleetMaintainedAppSettings{}))
		days, src, err := r.Resolve(ctx, &teamID, nil)
		require.NoError(t, err)
		assert.Equal(t, uint(7), days)
		assert.Equal(t, fleet.FMAUpdateDelaySourceOrg, src)
	})

	t.Run("unassigned reads default team config", func(t *testing.T) {
		ds := newDS(setting(7), setting(1), setting(4))
		r := NewUpdateDelayResolver(ds)
		for _, id := range []*uint{nil, new(uint(0))} {
			days, src, err := r.Resolve(ctx, id, nil)
			require.NoError(t, err)
			assert.Equal(t, uint(4), days)
			assert.Equal(t, fleet.FMAUpdateDelaySourceFleet, src)
		}
		assert.False(t, ds.TeamLiteFuncInvoked)
	})

	t.Run("memoizes config reads", func(t *testing.T) {
		ds := newDS(setting(7), fleet.FleetMaintainedAppSettings{}, fleet.FleetMaintainedAppSettings{})
		appCalls, teamCalls := 0, 0
		appFn, teamFn := ds.AppConfigFunc, ds.TeamLiteFunc
		ds.AppConfigFunc = func(ctx context.Context) (*fleet.AppConfig, error) { appCalls++; return appFn(ctx) }
		ds.TeamLiteFunc = func(ctx context.Context, id uint) (*fleet.TeamLite, error) { teamCalls++; return teamFn(ctx, id) }
		r := NewUpdateDelayResolver(ds)
		other := uint(4)
		for _, id := range []*uint{&teamID, &teamID, &other, &other, &teamID} {
			_, _, err := r.Resolve(ctx, id, nil)
			require.NoError(t, err)
		}
		assert.Equal(t, 1, appCalls)
		assert.Equal(t, 2, teamCalls)
	})
}

func TestPendingVersion(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	delay := 7 * day
	v := func(id uint, ver string, age time.Duration) fleet.FleetMaintainedVersion {
		return fleet.FleetMaintainedVersion{ID: id, Version: ver, FirstSeenAt: now.Add(-age)}
	}
	versions := []fleet.FleetMaintainedVersion{v(12, "149.0.2", 2*day), v(11, "148.0.1", 9*day), v(9, "147.0.9", 20*day)}

	cases := []struct {
		name   string
		vs     []fleet.FleetMaintainedVersion
		active uint
		pin    string
		want   uint
		wantOK bool
		delay  time.Duration
	}{
		{"newest ineligible", versions, 9, "", 12, true, delay},
		{"active is newest", versions, 12, "", 0, false, delay},
		{"older than active is not pending", versions, 11, "^147", 0, false, delay},
		{"caret skips other majors", versions, 9, "^148", 0, false, delay},
		{"exactly at boundary is eligible", []fleet.FleetMaintainedVersion{v(2, "2.0", 7*day), v(1, "1.0", 30*day)}, 1, "", 0, false, delay},
		{"just inside boundary is pending", []fleet.FleetMaintainedVersion{v(2, "2.0", 7*day-time.Second), v(1, "1.0", 30*day)}, 1, "", 2, true, delay},
		{"literal pin never pending", versions, 9, "147.0.9", 0, false, delay},
		{"zero delay never pending", versions, 9, "", 0, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := PendingVersion(tc.vs, tc.active, tc.pin, tc.delay, now)
			require.Equal(t, tc.wantOK, ok)
			if ok {
				require.Equal(t, tc.want, got.ID)
			}
		})
	}
}

func TestVersionEligibleZeroDelayIgnoresFirstSeenAt(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	future := fleet.FleetMaintainedVersion{ID: 1, Version: "1.0", FirstSeenAt: now.Add(time.Hour)}
	assert.True(t, VersionEligible(future, 0, now))
	assert.False(t, VersionEligible(future, time.Hour, now))
	assert.True(t, VersionEligible(fleet.FleetMaintainedVersion{}, 0, now))
}
