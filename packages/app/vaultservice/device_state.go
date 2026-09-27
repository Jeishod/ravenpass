package vaultservice

import "github.com/dortanes/ravenpass/packages/app/unlock"

// deviceState caches the device records of the unlocked session; writes refresh it and locking drops it.
type deviceState struct {
	usage        usageLog
	usageLoaded  bool
	export       *exportRecord
	exportLoaded bool
	// policy is read while locked, so it is keyed by policyVault.
	policy      unlock.Policy
	policyVault string
}

func (d *deviceState) rememberPolicy(vaultID string, policy unlock.Policy) {
	d.policy = policy
	d.policyVault = vaultID
}

func (d *deviceState) knownPolicy(vaultID string) (unlock.Policy, bool) {
	if d.policyVault == "" || d.policyVault != vaultID {
		return unlock.Policy{}, false
	}
	return d.policy, true
}

func (d *deviceState) rememberUsage(log usageLog) {
	d.usage = log
	d.usageLoaded = true
}

func (d *deviceState) rememberExport(record *exportRecord) {
	d.export = record
	d.exportLoaded = true
}

func (d *deviceState) forget() {
	*d = deviceState{}
}
