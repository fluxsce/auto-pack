package projectrelease

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/git"
	"auto-pack/backend/pkg/projectfs"
)

// CollectWorkspaceRemoteTags merges origin tags from workspace projects (first successful repo wins per tag name).
func CollectWorkspaceRemoteTags(
	ctx context.Context,
	g *GitCommitter,
	ws *repository.PackWorkspace,
	projects []repository.PackProject,
	dataDir string,
) ([]string, bool) {
	if g == nil || ws == nil {
		return nil, false
	}
	seen := make(map[string]struct{})
	var merged []string
	ok := false
	for i := range projects {
		p := &projects[i]
		if strings.TrimSpace(p.RepoURL) == "" {
			continue
		}
		shellEnv, err := g.shellEnv(p)
		if err != nil {
			continue
		}
		workDir := projectfs.ResolveProjectAbsDir(p, ws, dataDir)
		var tags []string
		var listErr error
		if workDir != "" && git.IsRepository(ctx, workDir) {
			tags, listErr = git.ListRemoteTagsOnOrigin(ctx, workDir, shellEnv)
		} else {
			repoURL, urlErr := g.authenticatedRepoURL(p)
			if urlErr != nil {
				continue
			}
			tags, listErr = git.ListRemoteTagsOnURL(ctx, repoURL, shellEnv)
		}
		if listErr != nil {
			continue
		}
		ok = true
		for _, t := range tags {
			t = git.NormalizeTagName(t)
			if t == "" {
				continue
			}
			if _, exists := seen[t]; exists {
				continue
			}
			seen[t] = struct{}{}
			merged = append(merged, t)
		}
	}
	return merged, ok
}

func (g *GitCommitter) authenticatedRepoURL(project *repository.PackProject) (string, error) {
	repoURL := strings.TrimSpace(project.RepoURL)
	if repoURL == "" {
		return "", errNoRepoURL
	}
	if project.GitEnvRecID == "" || g.EnvRepo == nil {
		return repoURL, nil
	}
	item, err := g.EnvRepo.FindByRecID(project.GitEnvRecID)
	if err != nil {
		return "", err
	}
	var cfg envtypes.GitConfig
	if err := json.Unmarshal([]byte(item.ConfigJSON), &cfg); err != nil {
		return "", err
	}
	if strings.EqualFold(strings.TrimSpace(cfg.AuthType), envtypes.GitAuthHTTPS) {
		return embedHTTPSAuth(repoURL, cfg), nil
	}
	return repoURL, nil
}

var errNoRepoURL = errors.New("project has no repository URL configured")

func embedHTTPSAuth(rawURL string, cfg envtypes.GitConfig) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if strings.TrimSpace(cfg.AuthToken) != "" {
		u.User = url.UserPassword("oauth2", cfg.AuthToken)
	} else if u2 := strings.TrimSpace(cfg.AuthUsername); u2 != "" {
		u.User = url.UserPassword(u2, cfg.AuthPassword)
	}
	return u.String()
}
