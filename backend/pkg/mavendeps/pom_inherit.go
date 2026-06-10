package mavendeps

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reParentBlock     = regexp.MustCompile(`(?s)<parent>(.*?)</parent>`)
	reDepMgmt         = regexp.MustCompile(`(?s)<dependencyManagement>.*?</dependencyManagement>`)
	reProfilesSection = regexp.MustCompile(`(?s)<profiles>(.*?)</profiles>`)
	reProfileBlock    = regexp.MustCompile(`(?s)<profile>(.*?)</profile>`)
	reActiveByDefault = regexp.MustCompile(`(?s)<activeByDefault>\s*true\s*</activeByDefault>`)
)

// ParentRef is the Maven parent coordinates from pom.xml.
type ParentRef struct {
	GroupID      string
	ArtifactID   string
	Version      string
	RelativePath string
}

type pomLookup struct {
	byGAV map[string]string
	byGA  map[string]string
}

func parseParentRef(content string) (ParentRef, bool) {
	m := reParentBlock.FindStringSubmatch(content)
	if len(m) < 2 {
		return ParentRef{}, false
	}
	block := m[1]
	ref := ParentRef{}
	if v := reGroupID.FindStringSubmatch(block); len(v) == 2 {
		ref.GroupID = strings.TrimSpace(v[1])
	}
	if v := reArtifactID.FindStringSubmatch(block); len(v) == 2 {
		ref.ArtifactID = strings.TrimSpace(v[1])
	}
	if v := reVersion.FindStringSubmatch(block); len(v) == 2 {
		ref.Version = strings.TrimSpace(v[1])
	}
	if v := regexp.MustCompile(`(?s)<relativePath>\s*([^<]*?)\s*</relativePath>`).FindStringSubmatch(block); len(v) == 2 {
		ref.RelativePath = strings.TrimSpace(v[1])
	}
	return ref, ref.GroupID != "" && ref.ArtifactID != ""
}

// parseActiveProfileCoordinates reads GAV overrides from active-by-default profiles.
func parseActiveProfileCoordinates(content string) (groupID, artifactID, version string) {
	m := reProfilesSection.FindStringSubmatch(content)
	if len(m) < 2 {
		return "", "", ""
	}
	for _, pm := range reProfileBlock.FindAllStringSubmatch(m[1], -1) {
		if len(pm) < 2 || !reActiveByDefault.MatchString(pm[1]) {
			continue
		}
		block := stripXMLComments(pm[1])
		block = reDepMgmt.ReplaceAllString(block, "")
		block = reDepSection.ReplaceAllString(block, "")
		block = reBuildSection.ReplaceAllString(block, "")
		if g := firstTag(block, reGroupID); g != "" {
			groupID = g
		}
		if a := firstTag(block, reArtifactID); a != "" && !isMavenPluginCoordinate("", a) {
			artifactID = a
		}
		if v := firstTag(block, reVersion); v != "" {
			version = v
		}
	}
	return groupID, artifactID, version
}

func parseProfileDependencies(content string) []Coordinates {
	m := reProfilesSection.FindStringSubmatch(content)
	if len(m) < 2 {
		return nil
	}
	var deps []Coordinates
	for _, pm := range reProfileBlock.FindAllStringSubmatch(m[1], -1) {
		if len(pm) < 2 || !reActiveByDefault.MatchString(pm[1]) {
			continue
		}
		block := stripXMLComments(pm[1])
		block = reDepMgmt.ReplaceAllString(block, "")
		for _, depBlock := range reDepBlock.FindAllStringSubmatch(block, -1) {
			if len(depBlock) < 2 {
				continue
			}
			dep := parseDependencyBlock(depBlock[1])
			if dep.GroupID == "" || dep.ArtifactID == "" || shouldSkipDependency(depBlock[1]) {
				continue
			}
			deps = append(deps, dep)
		}
	}
	return deps
}

func parseDirectDependencies(content string) []Coordinates {
	content = stripXMLComments(content)
	content = reDepMgmt.ReplaceAllString(content, "")
	var deps []Coordinates
	for _, block := range reDepBlock.FindAllStringSubmatch(content, -1) {
		if len(block) < 2 {
			continue
		}
		dep := parseDependencyBlock(block[1])
		if dep.GroupID == "" || dep.ArtifactID == "" {
			continue
		}
		if shouldSkipDependency(block[1]) {
			continue
		}
		deps = append(deps, dep)
	}
	return deps
}

func parseDependencyManagement(content string) map[string]string {
	m := reDepMgmt.FindStringSubmatch(content)
	if len(m) < 2 {
		return nil
	}
	out := map[string]string{}
	for _, block := range reDepBlock.FindAllStringSubmatch(m[1], -1) {
		if len(block) < 2 {
			continue
		}
		dep := parseDependencyBlock(block[1])
		if dep.GroupID == "" || dep.ArtifactID == "" {
			continue
		}
		if shouldSkipDependency(block[1]) {
			continue
		}
		out[dep.Key()] = dep.Version
	}
	return out
}

func mergeDependencyLists(base, child []Coordinates) []Coordinates {
	byKey := map[string]Coordinates{}
	order := make([]string, 0, len(base)+len(child))
	add := func(d Coordinates) {
		k := d.Key()
		if _, ok := byKey[k]; !ok {
			order = append(order, k)
		}
		byKey[k] = d
	}
	for _, d := range base {
		add(d)
	}
	for _, d := range child {
		add(d)
	}
	out := make([]Coordinates, 0, len(order))
	for _, k := range order {
		out = append(out, byKey[k])
	}
	return out
}

func mergeDMMaps(parent, child map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range parent {
		out[k] = v
	}
	for k, v := range child {
		out[k] = v
	}
	return out
}

func applyDependencyManagement(deps []Coordinates, dm map[string]string, pom *POM, fallbacks map[string]string) []Coordinates {
	if len(dm) == 0 {
		return deps
	}
	out := make([]Coordinates, len(deps))
	for i, d := range deps {
		if strings.TrimSpace(d.Version) == "" {
			if v, ok := dm[d.Key()]; ok {
				d.Version = pom.ResolveVersion(v, fallbacks)
			}
		}
		out[i] = d
	}
	return out
}

func resolveParentPomPath(pomDir string, parent ParentRef, localRepo string, lookup pomLookup) string {
	rel := strings.TrimSpace(parent.RelativePath)
	if rel == "" {
		rel = "../pom.xml"
	}
	if !strings.EqualFold(rel, "false") && !strings.EqualFold(rel, "none") {
		candidate := filepath.Clean(filepath.Join(pomDir, rel))
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	gav := parent.GroupID + ":" + parent.ArtifactID + ":" + parent.Version
	if p, ok := lookup.byGAV[gav]; ok {
		return p
	}
	if p, ok := lookup.byGA[parent.GroupID+":"+parent.ArtifactID]; ok {
		return p
	}
	if localRepo != "" && parent.Version != "" {
		segments := strings.Split(parent.GroupID, ".")
		segments = append(segments, parent.ArtifactID, parent.Version, parent.ArtifactID+"-"+parent.Version+".pom")
		candidate := filepath.Join(append([]string{localRepo}, segments...)...)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// ResolveModuleCoordinates resolves the GAV Maven uses to publish this module.
func ResolveModuleCoordinates(
	pomPath, localRepo string,
	lookup pomLookup,
	projectName, projectVersion string,
) (Coordinates, error) {
	return resolveModuleCoordinatesAt(pomPath, localRepo, lookup, projectName, projectVersion, map[string]bool{})
}

func resolveModuleCoordinatesAt(
	pomPath, localRepo string,
	lookup pomLookup,
	defaultArtifact, projectVersion string,
	seen map[string]bool,
) (Coordinates, error) {
	pomPath = filepath.Clean(pomPath)
	if seen[pomPath] {
		return Coordinates{}, fmt.Errorf("cyclic parent chain at %s", pomPath)
	}
	seen[pomPath] = true

	pom, err := ParsePOMFile(pomPath)
	if err != nil {
		return Coordinates{}, err
	}
	content := readPOMContent(pomPath)
	fallbacks := CollectEffectiveProperties(pomPath, localRepo, lookup, map[string]string{
		"project.version": projectVersion,
	})

	groupID := strings.TrimSpace(pom.GroupID)
	artifactID := strings.TrimSpace(pom.ArtifactID)
	if groupID != "" {
		groupID = strings.TrimSpace(pom.ResolveVersion(groupID, fallbacks))
		if isUnresolvedVersion(groupID) {
			groupID = ""
		}
	}
	if isMavenPluginCoordinate(groupID, artifactID) {
		groupID = ""
		artifactID = ""
	}
	if artifactID == "" {
		artifactID = strings.TrimSpace(defaultArtifact)
	}
	version := pom.ResolveVersion(pom.Version, fallbacks)
	if isMavenPluginCoordinate("", artifactID) {
		artifactID = strings.TrimSpace(defaultArtifact)
	}

	if parent, ok := parseParentRef(content); ok {
		parent.Version = pom.ResolveVersion(parent.Version, fallbacks)
		parentPath := resolveParentPomPath(filepath.Dir(pomPath), parent, localRepo, lookup)
		if parentPath != "" {
			parentCoord, err := resolveModuleCoordinatesAt(
				parentPath, localRepo, lookup, parent.ArtifactID, projectVersion, seen,
			)
			if err == nil {
				if groupID == "" {
					groupID = parentCoord.GroupID
				}
				if version == "" || isUnresolvedVersion(version) || isSnapshotVersion(version) {
					version = parentCoord.Version
				}
			}
		}
	}

	if groupID == "" {
		if parent, ok := parseParentRef(content); ok {
			groupID = strings.TrimSpace(parent.GroupID)
		}
	}
	if groupID == "" {
		return Coordinates{}, fmt.Errorf("groupId missing in %s", pomPath)
	}
	if artifactID == "" {
		return Coordinates{}, fmt.Errorf("artifactId missing in %s", pomPath)
	}
	version = pom.ResolveVersion(version, fallbacks)
	if version == "" || isUnresolvedVersion(version) || isSnapshotVersion(version) {
		if pv := strings.TrimSpace(projectVersion); pv != "" {
			version = pv
		}
	}
	coord := Coordinates{
		GroupID:    groupID,
		ArtifactID: artifactID,
		Version:    version,
	}
	if strings.EqualFold(parsePackaging(content), "pom") {
		if leaf, ok := resolveAggregatorLeafCoordinates(pomPath, localRepo, lookup, defaultArtifact, projectVersion, coord); ok {
			return leaf, nil
		}
	}
	return coord, nil
}

func resolveAggregatorLeafCoordinates(
	pomPath, localRepo string,
	lookup pomLookup,
	defaultArtifact, projectVersion string,
	parentCoord Coordinates,
) (Coordinates, bool) {
	pomDir := filepath.Dir(filepath.Clean(pomPath))
	modules := parseModules(readPOMContent(pomPath))
	if len(modules) == 0 {
		return Coordinates{}, false
	}
	ordered := prioritizeModules(modules, defaultArtifact)
	for _, mod := range ordered {
		childPOM := filepath.Join(pomDir, filepath.FromSlash(mod), "pom.xml")
		if _, err := os.Stat(childPOM); err != nil {
			continue
		}
		childContent := readPOMContent(childPOM)
		if !isPublishablePackaging(parsePackaging(childContent)) {
			continue
		}
		fallbacks := CollectEffectiveProperties(childPOM, localRepo, lookup, map[string]string{
			"project.version": projectVersion,
		})
		childPOMFile, err := ParsePOMFile(childPOM)
		if err != nil {
			continue
		}
		groupID := strings.TrimSpace(childPOMFile.GroupID)
		if groupID != "" {
			groupID = strings.TrimSpace(childPOMFile.ResolveVersion(groupID, fallbacks))
			if isUnresolvedVersion(groupID) {
				groupID = ""
			}
		}
		artifactID := strings.TrimSpace(childPOMFile.ArtifactID)
		if artifactID == "" {
			artifactID = filepath.Base(filepath.FromSlash(mod))
		}
		if defaultArtifact != "" && !artifactNamesMatch(artifactID, defaultArtifact) {
			continue
		}
		version := childPOMFile.ResolveVersion(childPOMFile.Version, fallbacks)
		if version == "" || isUnresolvedVersion(version) || isSnapshotVersion(version) {
			version = parentCoord.Version
		}
		if groupID == "" {
			groupID = parentCoord.GroupID
		}
		if groupID == "" || artifactID == "" || version == "" {
			continue
		}
		return Coordinates{
			GroupID:    groupID,
			ArtifactID: artifactID,
			Version:    version,
		}, true
	}
	return Coordinates{}, false
}

func prioritizeModules(modules []string, defaultArtifact string) []string {
	if defaultArtifact == "" {
		return modules
	}
	var front, rest []string
	for _, mod := range modules {
		base := filepath.Base(filepath.FromSlash(mod))
		if artifactNamesMatch(base, defaultArtifact) {
			front = append(front, mod)
		} else {
			rest = append(rest, mod)
		}
	}
	return append(front, rest...)
}

// CollectEffectiveProperties merges properties from the parent chain (ancestors first,
// module last) so ${datahub.core.version} and similar keys resolve like Maven.
func CollectEffectiveProperties(pomPath, localRepo string, lookup pomLookup, base map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	seen := map[string]bool{}
	var ordered []*POM
	var walk func(string)
	walk = func(path string) {
		path = filepath.Clean(path)
		if seen[path] {
			return
		}
		seen[path] = true
		pom, err := ParsePOMFile(path)
		if err != nil {
			return
		}
		content := readPOMContent(path)
		if parent, ok := parseParentRef(content); ok {
			parent.Version = pom.ResolveVersion(parent.Version, out)
			parentPath := resolveParentPomPath(filepath.Dir(path), parent, localRepo, lookup)
			if parentPath != "" {
				walk(parentPath)
			}
		}
		ordered = append(ordered, pom)
	}
	walk(pomPath)
	for _, pom := range ordered {
		for k, v := range pom.Properties {
			out[k] = v
		}
		if v := strings.TrimSpace(pom.Version); v != "" {
			out["project.version"] = v
		}
	}
	return out
}

func effectiveDependenciesAt(
	pomPath, localRepo string,
	lookup pomLookup,
	fallbacks map[string]string,
	seen map[string]bool,
) ([]Coordinates, map[string]string, error) {
	pomPath = filepath.Clean(pomPath)
	if seen[pomPath] {
		return nil, nil, nil
	}
	seen[pomPath] = true

	pom, err := ParsePOMFile(pomPath)
	if err != nil {
		return nil, nil, err
	}
	pomDir := filepath.Dir(pomPath)
	childFallbacks := CollectEffectiveProperties(pomPath, localRepo, lookup, fallbacks)

	content := readPOMContent(pomPath)

	var inherited []Coordinates
	var dm map[string]string
	if parent, ok := parseParentRef(content); ok {
		parent.Version = pom.ResolveVersion(parent.Version, childFallbacks)
		parentPath := resolveParentPomPath(pomDir, parent, localRepo, lookup)
		if parentPath != "" {
			parentDeps, parentDM, err := effectiveDependenciesAt(parentPath, localRepo, lookup, childFallbacks, seen)
			if err == nil {
				inherited = parentDeps
				dm = parentDM
			}
		}
	}

	childDM := parseDependencyManagement(content)
	dm = mergeDMMaps(dm, childDM)
	own := mergeDependencyLists(parseDirectDependencies(content), parseProfileDependencies(content))
	deps := mergeDependencyLists(inherited, own)
	deps = applyDependencyManagement(deps, dm, pom, childFallbacks)
	for i := range deps {
		deps[i].Version = pom.ResolveVersion(deps[i].Version, childFallbacks)
	}
	return deps, dm, nil
}

func readPOMContent(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(raw)
}

// EffectiveDependencies resolves inherited parent dependencies for a module pom.
func EffectiveDependencies(pomPath, localRepo string, lookup pomLookup, fallbacks map[string]string) ([]Coordinates, error) {
	deps, _, err := effectiveDependenciesAt(pomPath, localRepo, lookup, fallbacks, map[string]bool{})
	return deps, err
}

func buildPOMLookup(entries []ScanEntry) pomLookup {
	byGAV := map[string]string{}
	byGA := map[string]string{}

	for _, e := range entries {
		pomPath := filepath.Join(e.ProjectDir, "pom.xml")
		pom, err := ParsePOMFile(pomPath)
		if err != nil {
			continue
		}
		fb := map[string]string{"project.version": e.ProjectVersion}
		for k, v := range pom.Properties {
			fb[k] = v
		}
		ver := pom.ResolveVersion(pom.Version, fb)
		if ver == "" {
			ver = strings.TrimSpace(e.ProjectVersion)
		}
		gav := pom.GroupID + ":" + pom.ArtifactID + ":" + ver
		byGAV[gav] = pomPath
		ga := pom.GroupID + ":" + pom.ArtifactID
		if _, ok := byGA[ga]; !ok {
			byGA[ga] = pomPath
		}
	}
	return pomLookup{byGAV: byGAV, byGA: byGA}
}
