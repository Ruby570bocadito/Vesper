package agent

import "os"

// persistenceAllowed reports whether this agent may install
// persistence mechanisms on the current host. The check is fail-closed:
//
//   - VESPER_LAB_ONLY set (to anything other than "0") → never;
//     lab runs must not touch cron/systemd/registry on the host.
//   - noPersistence (from config safety.no_persistence) → never.
//
// Every install path (post-exploit fallbacks, watchdog resilience)
// must consult this before writing anything outside the process.
func persistenceAllowed(noPersistence bool) bool {
	if v := os.Getenv("VESPER_LAB_ONLY"); v != "" && v != "0" {
		return false
	}
	return !noPersistence
}
