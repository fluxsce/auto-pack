package repository

import "strings"

// HasDuplicateProjectNamesInWorkspace reports whether multiple projects share the same name.
func (r *ProjectRepository) HasDuplicateProjectNamesInWorkspace(workspaceRecID string) (bool, error) {
	workspaceRecID = strings.TrimSpace(workspaceRecID)
	if workspaceRecID == "" {
		return false, nil
	}
	var n int
	err := r.db.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT 1 FROM AP_PACK_PROJECT
			WHERE workspace_rec_id = ?
			GROUP BY project_name
			HAVING COUNT(*) > 1
		)`, workspaceRecID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// UpdateVersionByWorkspace sets project_version for all projects in the workspace.
func (r *ProjectRepository) UpdateVersionByWorkspace(workspaceRecID, version string) (int64, error) {
	workspaceRecID = strings.TrimSpace(workspaceRecID)
	version = strings.TrimSpace(version)
	if workspaceRecID == "" {
		return 0, nil
	}
	res, err := r.db.Exec(
		`UPDATE AP_PACK_PROJECT SET project_version = ?, updated_at = ? WHERE workspace_rec_id = ?`,
		version, nowTS(), workspaceRecID,
	)
	if err != nil {
		return 0, mapProjectUniqueErr(err)
	}
	return res.RowsAffected()
}

// UpdateRepoBranchByWorkspace sets repo_branch for all projects in the workspace.
func (r *ProjectRepository) UpdateRepoBranchByWorkspace(workspaceRecID, repoBranch string) (int64, error) {
	workspaceRecID = strings.TrimSpace(workspaceRecID)
	repoBranch = strings.TrimSpace(repoBranch)
	if workspaceRecID == "" {
		return 0, nil
	}
	res, err := r.db.Exec(
		`UPDATE AP_PACK_PROJECT SET repo_branch = ?, updated_at = ? WHERE workspace_rec_id = ?`,
		repoBranch, nowTS(), workspaceRecID,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
