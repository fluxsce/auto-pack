package projectcfg

import (
	"encoding/json"
	"strings"
)

// RemapBuildDependsOn rewrites build_triggers.depends_on project rec_ids using idMap.
// IDs not present in idMap are dropped (e.g. external workspace dependencies).
func RemapBuildDependsOn(configJSON string, idMap map[string]string) (string, error) {
	if len(idMap) == 0 {
		return NormalizeConfigJSON(configJSON)
	}
	cfg, err := ParseConfigJSON(configJSON)
	if err != nil {
		return "", err
	}
	if len(cfg.BuildTriggers.DependsOn) == 0 {
		return NormalizeConfigJSON(configJSON)
	}
	remapped := make([]string, 0, len(cfg.BuildTriggers.DependsOn))
	seen := map[string]bool{}
	for _, id := range cfg.BuildTriggers.DependsOn {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		newID, ok := idMap[id]
		if !ok {
			continue
		}
		newID = strings.TrimSpace(newID)
		if newID == "" || seen[newID] {
			continue
		}
		seen[newID] = true
		remapped = append(remapped, newID)
	}
	cfg.BuildTriggers.DependsOn = remapped
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
