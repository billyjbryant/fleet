package fleet

import (
	"fmt"

	"github.com/fleetdm/fleet/v4/pkg/optjson"
)

// MaxFMAUpdateDelayDays is the largest allowed Fleet-maintained app update delay.
const MaxFMAUpdateDelayDays = 365

// FleetMaintainedAppSettings holds settings that apply to all Fleet-maintained
// apps at the org or fleet level.
type FleetMaintainedAppSettings struct {
	// UpdateDelayDays is how long a new version must have been seen before
	// auto-updates promote it. Unset or null inherits from the next level.
	UpdateDelayDays optjson.Int `json:"update_delay_days"`
}

// Validate checks that the settings are within allowed bounds.
func (s FleetMaintainedAppSettings) Validate() error {
	if !s.UpdateDelayDays.Valid {
		return nil
	}
	if d := s.UpdateDelayDays.Value; d < 0 || d > MaxFMAUpdateDelayDays {
		return NewInvalidArgumentError("update_delay_days",
			fmt.Sprintf("must be between 0 and %d", MaxFMAUpdateDelayDays))
	}
	return nil
}

// Equal reports whether both settings have the same effective delay.
func (s FleetMaintainedAppSettings) Equal(o FleetMaintainedAppSettings) bool {
	return s.UpdateDelayDays.Valid == o.UpdateDelayDays.Valid &&
		s.UpdateDelayDays.Value == o.UpdateDelayDays.Value
}

// FMAUpdateDelaySource identifies which level an effective delay came from.
type FMAUpdateDelaySource string

const (
	FMAUpdateDelaySourceApp     FMAUpdateDelaySource = "app"
	FMAUpdateDelaySourceFleet   FMAUpdateDelaySource = "fleet"
	FMAUpdateDelaySourceOrg     FMAUpdateDelaySource = "org"
	FMAUpdateDelaySourceDefault FMAUpdateDelaySource = "default"
)

// ResolveFMAUpdateDelay returns the effective update delay, preferring the
// per-app value, then the fleet setting, then the org setting, then 0. A
// level that is not Valid (unset or null) is skipped.
func ResolveFMAUpdateDelay(appDays *uint, fleetSettings, orgSettings FleetMaintainedAppSettings) (days uint, source FMAUpdateDelaySource) {
	switch {
	case appDays != nil:
		return *appDays, FMAUpdateDelaySourceApp
	case fleetSettings.UpdateDelayDays.Valid:
		return uint(fleetSettings.UpdateDelayDays.Value), FMAUpdateDelaySourceFleet
	case orgSettings.UpdateDelayDays.Valid:
		return uint(orgSettings.UpdateDelayDays.Value), FMAUpdateDelaySourceOrg
	default:
		return 0, FMAUpdateDelaySourceDefault
	}
}
