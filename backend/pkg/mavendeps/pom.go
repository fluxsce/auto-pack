package mavendeps

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	reGroupID      = regexp.MustCompile(`(?s)<groupId>\s*([^<]+?)\s*</groupId>`)
	reArtifactID   = regexp.MustCompile(`(?s)<artifactId>\s*([^<]+?)\s*</artifactId>`)
	reVersion      = regexp.MustCompile(`(?s)<version>\s*([^<]+?)\s*</version>`)
	reProperties   = regexp.MustCompile(`(?s)<properties>(.*?)</properties>`)
	reXMLComment   = regexp.MustCompile(`(?s)<!--.*?-->`)
	reDepBlock     = regexp.MustCompile(`(?s)<dependency>(.*?)</dependency>`)
	reScope        = regexp.MustCompile(`(?s)<scope>\s*([^<]+?)\s*</scope>`)
	reOptional     = regexp.MustCompile(`(?s)<optional>\s*true\s*</optional>`)
	rePropEntry    = regexp.MustCompile(`<([a-zA-Z0-9_.-]+)>\s*([^<]+?)\s*</([a-zA-Z0-9_.-]+)>`)
	rePackaging    = regexp.MustCompile(`(?s)<packaging>\s*([^<]+?)\s*</packaging>`)
	reModulesBlock = regexp.MustCompile(`(?s)<modules>(.*?)</modules>`)
	reModuleTag    = regexp.MustCompile(`<module>\s*([^<]+?)\s*</module>`)
)

// Coordinates is a Maven GAV coordinate.
type Coordinates struct {
	GroupID    string `json:"group_id"`
	ArtifactID string `json:"artifact_id"`
	Version    string `json:"version,omitempty"`
}

// Key returns groupId:artifactId.
func (c Coordinates) Key() string {
	return c.GroupID + ":" + c.ArtifactID
}

// POM is parsed metadata from pom.xml.
type POM struct {
	GroupID      string
	ArtifactID   string
	Version      string
	Properties   map[string]string
	Dependencies []Coordinates
	rawContent   string
}

// ParsePOMFile reads and parses pom.xml at path.
func ParsePOMFile(path string) (*POM, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParsePOM(string(raw))
}

// ParsePOM extracts coordinates, properties, and compile-scope dependencies.
func ParsePOM(content string) (*POM, error) {
	if !strings.Contains(content, "<project") {
		return nil, fmt.Errorf("invalid pom.xml: missing project element")
	}
	p := &POM{Properties: map[string]string{}, rawContent: content}
	if m := reProperties.FindStringSubmatch(content); len(m) == 2 {
		for _, sm := range rePropEntry.FindAllStringSubmatch(m[1], -1) {
			if len(sm) == 4 && sm[1] == sm[3] {
				p.Properties[sm[1]] = strings.TrimSpace(sm[2])
			}
		}
	}
	g, a, v := parseProjectCoordinates(content)
	p.GroupID = g
	p.ArtifactID = a
	p.Version = v
	p.Dependencies = parseDirectDependencies(content)
	return p, nil
}

func stripXMLComments(content string) string {
	return reXMLComment.ReplaceAllString(content, "")
}

var (
	reDepSection      = regexp.MustCompile(`(?s)<dependencies>.*?</dependencies>`)
	reDepMgmtSection  = regexp.MustCompile(`(?s)<dependencyManagement>.*?</dependencyManagement>`)
	reBuildSection    = regexp.MustCompile(`(?s)<build>.*?</build>`)
	reProfilesBlock   = regexp.MustCompile(`(?s)<profiles>.*?</profiles>`)
	reReportingBlock  = regexp.MustCompile(`(?s)<reporting>.*?</reporting>`)
	reDistMgmtSection = regexp.MustCompile(`(?s)<distributionManagement>.*?</distributionManagement>`)
)

// projectModelSection is the project model with parent/dependency/build blocks removed.
func projectModelSection(content string) string {
	content = stripXMLComments(content)
	content = reParentBlock.ReplaceAllString(content, "")
	content = reDepMgmtSection.ReplaceAllString(content, "")
	content = reDepSection.ReplaceAllString(content, "")
	content = reBuildSection.ReplaceAllString(content, "")
	content = reProfilesBlock.ReplaceAllString(content, "")
	content = reReportingBlock.ReplaceAllString(content, "")
	content = reDistMgmtSection.ReplaceAllString(content, "")
	return content
}

func isMavenPluginCoordinate(groupID, artifactID string) bool {
	artifactID = strings.TrimSpace(strings.ToLower(artifactID))
	groupID = strings.TrimSpace(strings.ToLower(groupID))
	if strings.HasSuffix(artifactID, "-plugin") {
		return true
	}
	switch groupID {
	case "org.apache.maven.plugins", "org.codehaus.mojo":
		return true
	}
	return false
}

func parsePackaging(content string) string {
	return firstTag(projectModelSection(content), rePackaging)
}

func parseModules(content string) []string {
	m := reModulesBlock.FindStringSubmatch(stripXMLComments(content))
	if len(m) < 2 {
		return nil
	}
	var out []string
	for _, sm := range reModuleTag.FindAllStringSubmatch(m[1], -1) {
		if len(sm) == 2 {
			mod := strings.TrimSpace(sm[1])
			if mod != "" {
				out = append(out, mod)
			}
		}
	}
	return out
}

func isPublishablePackaging(packaging string) bool {
	switch strings.ToLower(strings.TrimSpace(packaging)) {
	case "", "jar", "war", "ear", "ejb", "bundle":
		return true
	default:
		return false
	}
}

func parseProjectCoordinates(content string) (groupID, artifactID, version string) {
	meta := projectModelSection(content)
	groupID = firstTag(meta, reGroupID)
	artifactID = firstTag(meta, reArtifactID)
	version = firstTag(meta, reVersion)
	pg, pa, pv := parseActiveProfileCoordinates(content)
	if groupID == "" {
		groupID = pg
	}
	if artifactID == "" {
		artifactID = pa
	}
	if version == "" {
		version = pv
	}
	return groupID, artifactID, version
}

func firstTag(section string, re *regexp.Regexp) string {
	if m := re.FindStringSubmatch(section); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func parseDependencyBlock(block string) Coordinates {
	var c Coordinates
	if m := reGroupID.FindStringSubmatch(block); len(m) == 2 {
		c.GroupID = strings.TrimSpace(m[1])
	}
	if m := reArtifactID.FindStringSubmatch(block); len(m) == 2 {
		c.ArtifactID = strings.TrimSpace(m[1])
	}
	if m := reVersion.FindStringSubmatch(block); len(m) == 2 {
		c.Version = strings.TrimSpace(m[1])
	}
	return c
}

func shouldSkipDependency(block string) bool {
	if reOptional.MatchString(block) {
		return true
	}
	if m := reScope.FindStringSubmatch(block); len(m) == 2 {
		switch strings.TrimSpace(strings.ToLower(m[1])) {
		case "test", "provided", "import":
			return true
		}
	}
	return false
}

// ResolveVersion substitutes ${property} and ${project.version} using pom + fallbacks.
func (p *POM) ResolveVersion(raw string, fallbacks map[string]string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return strings.TrimSpace(p.Version)
	}
	seen := map[string]bool{}
	for strings.HasPrefix(raw, "${") && strings.HasSuffix(raw, "}") {
		key := strings.TrimSpace(raw[2 : len(raw)-1])
		if seen[key] {
			break
		}
		seen[key] = true
		if v, ok := p.Properties[key]; ok && strings.TrimSpace(v) != "" {
			raw = strings.TrimSpace(v)
			continue
		}
		if key == "project.version" && strings.TrimSpace(p.Version) != "" {
			raw = strings.TrimSpace(p.Version)
			continue
		}
		if fallbacks != nil {
			if v, ok := fallbacks[key]; ok && strings.TrimSpace(v) != "" {
				raw = strings.TrimSpace(v)
				continue
			}
		}
		break
	}
	return raw
}
