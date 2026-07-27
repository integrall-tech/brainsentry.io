package domain

import (
	"encoding/json"
	"time"
)

// RetentionPolicy says how long a memory survives AFTER its validity window
// closed — the question RFC-014 §10 left open.
//
// It is per tenant and lives in tenants.settings, not in code: the answer is
// legal before it is technical, it differs per customer, and hard-coding a
// number would make every future legal answer a deploy.
//
// Zero value = retention disabled. A tenant that has not declared a policy
// keeps everything, which is the safe direction to be wrong in: deleting on a
// default nobody chose is unrecoverable, keeping is not.
type RetentionPolicy struct {
	// PurgeAfterValidToDays is the grace period between valid_to and actual
	// removal. 0 disables the sweep entirely.
	PurgeAfterValidToDays int `json:"purgeAfterValidToDays"`
	// MaxPerRun bounds one sweep so a first run on a large tenant cannot
	// become an unbounded delete. 0 uses the service default.
	MaxPerRun int `json:"maxPerRun,omitempty"`
}

// Enabled reports whether the sweep should run at all.
func (p RetentionPolicy) Enabled() bool {
	return p.PurgeAfterValidToDays > 0
}

// CutoffFrom returns the timestamp before which an expired memory is due for
// removal.
func (p RetentionPolicy) CutoffFrom(now time.Time) time.Time {
	return now.AddDate(0, 0, -p.PurgeAfterValidToDays)
}

// tenantSettings is the shape RetentionPolicy occupies inside
// tenants.settings, kept narrow so unrelated keys in that JSONB are ignored
// rather than fought over.
type tenantSettings struct {
	Retention *RetentionPolicy `json:"retention"`
}

// RetentionPolicyFromSettings extracts the policy from a tenant's settings
// blob. Absent, empty or malformed settings yield a disabled policy —
// failing closed here means "keep the data", which is the recoverable error.
func RetentionPolicyFromSettings(settings []byte) RetentionPolicy {
	if len(settings) == 0 {
		return RetentionPolicy{}
	}
	var s tenantSettings
	if err := json.Unmarshal(settings, &s); err != nil || s.Retention == nil {
		return RetentionPolicy{}
	}
	return *s.Retention
}

// ErasureReceipt is the proof that a removal happened, kept after the data is
// gone. It holds identifiers and counts, never subject content — documenting
// an erasure with the erased text would defeat it.
type ErasureReceipt struct {
	ID          string          `json:"id"`
	TenantID    string          `json:"tenantId"`
	Kind        string          `json:"kind"`
	Scope       json.RawMessage `json:"scope,omitempty"`
	Counts      json.RawMessage `json:"counts,omitempty"`
	Reason      string          `json:"reason,omitempty"`
	RequestedBy string          `json:"requestedBy,omitempty"`
	Executed    bool            `json:"executed"`
	CreatedAt   time.Time       `json:"createdAt"`
}
