package intake

import (
	"context"
	"errors"
	"strings"

	"auto-pack/server/internal/batchlock"
	"auto-pack/server/internal/build"
	"auto-pack/server/internal/build/record"
)

type workspaceBuilder struct {
	svc *build.Service
}

// BindWorkspaceBuild 把工作空间自动构建接到通知处理。报文未给工程名时走这条。
func BindWorkspaceBuild(svc *build.Service) WorkspaceBuild {
	if svc == nil {
		return nil
	}
	return workspaceBuilder{svc: svc}
}

func (b workspaceBuilder) StartWorkspace(ctx context.Context, workspaceID string) (string, error) {
	res, err := b.svc.StartSystem(ctx, build.StartRequest{
		WorkspaceRecID:  workspaceID,
		Scope:           "workspace",
		IncludeNonMaven: true,
	})
	if errors.Is(err, build.ErrBuildInProgress) || errors.Is(err, batchlock.ErrBusy) {
		return "", errBuildBusy
	}
	if err != nil {
		return "", err
	}
	return res.BuildRecID, nil
}

func (b workspaceBuilder) WaitWorkspace(ctx context.Context, buildID string) (BuildOutcome, error) {
	rec, err := b.svc.Wait(ctx, buildID)
	if err != nil {
		return BuildOutcome{}, err
	}
	out := BuildOutcome{
		Status:    rec.Status,
		Succeeded: rec.Succeeded,
		Skipped:   rec.Skipped,
		Failed:    rec.Failed,
		Message:   rec.Message,
	}
	for _, row := range b.svc.ProjectResults(rec.WorkspaceRecID, buildID) {
		out.Projects = append(out.Projects, BuildProject{Name: row.ProjectName, Status: row.Status})
	}
	return out, nil
}

// AppendLog 把回执三步写进这次工作空间构建日志的末尾。
func (b workspaceBuilder) AppendLog(ctx context.Context, buildID, text string) error {
	if b.svc == nil || strings.TrimSpace(text) == "" {
		return nil
	}
	rec, err := b.svc.Wait(ctx, buildID)
	if err != nil {
		return err
	}
	lines := []string{"", "—— 回执 ——"}
	lines = append(lines, strings.Split(text, "\n")...)
	return record.AppendLog(rec.LogPath, lines...)
}
