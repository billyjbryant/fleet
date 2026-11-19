package fleet

import (
	"encoding/json"
	"testing"

	"github.com/fleetdm/fleet/v4/pkg/optjson"
	"github.com/stretchr/testify/require"
)

func TestResolveFMAUpdateDelay(t *testing.T) {
	u := func(v uint) *uint { return &v }
	set := func(v int) FleetMaintainedAppSettings {
		return FleetMaintainedAppSettings{UpdateDelayDays: optjson.SetInt(v)}
	}
	var nullSet FleetMaintainedAppSettings
	require.NoError(t, json.Unmarshal([]byte(`{"update_delay_days":null}`), &nullSet))
	require.True(t, nullSet.UpdateDelayDays.Set)

	cases := []struct {
		name       string
		app        *uint
		fleet, org FleetMaintainedAppSettings
		wantDays   uint
		wantSource FMAUpdateDelaySource
	}{
		{"app wins", u(3), set(5), set(7), 3, FMAUpdateDelaySourceApp},
		{"fleet wins", nil, set(5), set(7), 5, FMAUpdateDelaySourceFleet},
		{"fleet zero opts out", nil, set(0), set(7), 0, FMAUpdateDelaySourceFleet},
		{"org fallback", nil, FleetMaintainedAppSettings{}, set(7), 7, FMAUpdateDelaySourceOrg},
		{"explicit null inherits", nil, nullSet, set(7), 7, FMAUpdateDelaySourceOrg},
		{"all unset", nil, FleetMaintainedAppSettings{}, FleetMaintainedAppSettings{}, 0, FMAUpdateDelaySourceDefault},
		{"app zero", u(0), FleetMaintainedAppSettings{}, set(7), 0, FMAUpdateDelaySourceApp},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			days, source := ResolveFMAUpdateDelay(tc.app, tc.fleet, tc.org)
			require.Equal(t, tc.wantDays, days)
			require.Equal(t, tc.wantSource, source)
		})
	}
}

func TestFleetMaintainedAppSettingsValidate(t *testing.T) {
	var nullSet FleetMaintainedAppSettings
	require.NoError(t, json.Unmarshal([]byte(`{"update_delay_days":null}`), &nullSet))

	require.NoError(t, FleetMaintainedAppSettings{}.Validate())
	require.NoError(t, nullSet.Validate())
	require.NoError(t, FleetMaintainedAppSettings{UpdateDelayDays: optjson.SetInt(0)}.Validate())
	require.NoError(t, FleetMaintainedAppSettings{UpdateDelayDays: optjson.SetInt(MaxFMAUpdateDelayDays)}.Validate())
	for _, v := range []int{-1, MaxFMAUpdateDelayDays + 1} {
		err := FleetMaintainedAppSettings{UpdateDelayDays: optjson.SetInt(v)}.Validate()
		require.ErrorContains(t, err, "update_delay_days")
	}
}

func TestFleetMaintainedAppSettingsEqual(t *testing.T) {
	a := FleetMaintainedAppSettings{UpdateDelayDays: optjson.SetInt(3)}
	require.True(t, a.Equal(FleetMaintainedAppSettings{UpdateDelayDays: optjson.SetInt(3)}))
	require.False(t, a.Equal(FleetMaintainedAppSettings{UpdateDelayDays: optjson.SetInt(4)}))
	require.False(t, a.Equal(FleetMaintainedAppSettings{}))
	require.True(t, FleetMaintainedAppSettings{}.Equal(FleetMaintainedAppSettings{UpdateDelayDays: optjson.Int{Set: true}}))
}

func TestFleetMaintainedAppSettingsSurviveCopyAndToLite(t *testing.T) {
	want := FleetMaintainedAppSettings{UpdateDelayDays: optjson.SetInt(14)}

	ac := &AppConfig{FleetMaintainedAppSettings: want}
	b, err := json.Marshal(ac.Copy())
	require.NoError(t, err)
	require.Contains(t, string(b), `"fleet_maintained_app_settings":{"update_delay_days":14}`)
	var gotAC AppConfig
	require.NoError(t, json.Unmarshal(b, &gotAC))
	require.Equal(t, want, gotAC.FleetMaintainedAppSettings)

	tc := TeamConfig{FleetMaintainedAppSettings: want}
	require.Equal(t, want, tc.Copy().FleetMaintainedAppSettings)
	lite := tc.ToLite()
	require.Equal(t, want, lite.FleetMaintainedAppSettings)
	b, err = json.Marshal(lite)
	require.NoError(t, err)
	var gotLite TeamConfigLite
	require.NoError(t, json.Unmarshal(b, &gotLite))
	require.Equal(t, want, gotLite.FleetMaintainedAppSettings)
}
