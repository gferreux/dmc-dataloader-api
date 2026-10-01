package sftpgo

import (
	"context"
	"slices"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

type accountPlan struct {
	req     model.SFTPAccountRequest
	bucket  string
	subs    []string
	folders []plannedFolder
	user    map[string]any
	action  string
	added   int
}

func (s *Service) prepare(ctx context.Context, req model.SFTPAccountRequest) (*accountPlan, error) {
	if err := s.configuredErr(); err != nil {
		return nil, err
	}

	req = req.Normalized()
	if issues := req.Issues(); len(issues) > 0 {
		return nil, &model.ValidationError{Issues: issues}
	}

	subs, _ := model.SFTPSubfolders(req.ClientType)

	bucket := s.bucketFor(req.ClientType)

	folders, err := s.resolveFolders(ctx, req, bucket, subs)
	if err != nil {
		return nil, err
	}

	user, exists, err := s.client.GetUser(ctx, req.User)
	if err != nil {
		return nil, err
	}

	if exists {
		if err := checkBucket("user", req.User, gcsBucket(user), bucket); err != nil {
			return nil, err
		}
	}

	action, added := decideAction(user, exists, folders)

	return &accountPlan{
		req:     req,
		bucket:  bucket,
		subs:    subs,
		folders: folders,
		user:    user,
		action:  action,
		added:   added,
	}, nil
}

func (s *Service) resolveFolders(
	ctx context.Context,
	req model.SFTPAccountRequest,
	bucket string,
	subs []string,
) ([]plannedFolder, error) {
	folders := make([]plannedFolder, 0, len(subs))
	for _, sub := range subs {
		name := req.User + "/" + req.Base + "/" + sub

		virtualPath := "/" + req.Base + "/" + sub

		existing, found, err := s.client.GetFolder(ctx, name)
		if err != nil {
			return nil, err
		}

		action := model.SFTPActionCreate

		if found {
			if err := checkBucket("folder", name, gcsBucket(existing), bucket); err != nil {
				return nil, err
			}

			action = model.SFTPFolderExists
		}

		folders = append(folders, plannedFolder{
			name:        name,
			virtualPath: virtualPath,
			sub:         sub,
			action:      action,
		})
	}

	return folders, nil
}

func checkBucket(subject, name, actual, expected string) error {
	if actual != "" && actual != expected {
		return &model.BucketMismatchError{
			Subject:  subject,
			Name:     name,
			Actual:   actual,
			Expected: expected,
		}
	}

	return nil
}

func decideAction(user map[string]any, exists bool, folders []plannedFolder) (string, int) {
	if !exists {
		return model.SFTPActionCreate, len(folders)
	}

	missing := countMissing(virtualPaths(user), folders)
	if missing == 0 && permissionsEqual(mergedPermissions(user, folders), permissionMap(user)) {
		return model.SFTPActionUnchanged, 0
	}

	return model.SFTPActionUpdate, missing
}

func countMissing(paths map[string]struct{}, folders []plannedFolder) int {
	missing := 0

	for _, folder := range folders {
		if _, ok := paths[folder.virtualPath]; !ok {
			missing++
		}
	}

	return missing
}

func (p *accountPlan) paths() []string {
	paths := make([]string, 0, len(p.folders))
	for _, folder := range p.folders {
		paths = append(paths, folder.virtualPath)
	}

	return paths
}

func (p *accountPlan) view() model.SFTPPlan {
	folders := make([]model.SFTPPlannedFolder, 0, len(p.folders))
	for _, folder := range p.folders {
		folders = append(folders, model.SFTPPlannedFolder{
			Name:        folder.name,
			VirtualPath: folder.virtualPath,
			Action:      folder.action,
		})
	}

	return model.SFTPPlan{
		User:       p.req.User,
		Base:       p.req.Base,
		ClientType: p.req.ClientType,
		Bucket:     p.bucket,
		Subfolders: stringList(p.subs),
		Folders:    folders,
		UserAction: p.action,
		Warnings:   p.warnings(),
	}
}

func (p *accountPlan) warnings() []string {
	if p.action == model.SFTPActionCreate {
		return []string{}
	}

	warnings := []string{warningPasswordKept}
	if len(p.req.PublicKeys) > 0 {
		warnings = append(warnings, warningKeysIgnored)
	}

	return warnings
}

func (p *accountPlan) applyResult() model.SFTPApplyResult {
	created := make([]string, 0)
	existing := make([]string, 0)

	for _, folder := range p.folders {
		if folder.action == model.SFTPFolderExists {
			existing = append(existing, folder.name)

			continue
		}

		created = append(created, folder.name)
	}

	return model.SFTPApplyResult{
		User:                p.req.User,
		Base:                p.req.Base,
		ClientType:          p.req.ClientType,
		Bucket:              p.bucket,
		Subfolders:          stringList(p.subs),
		FoldersCreated:      created,
		FoldersExisting:     existing,
		UserAction:          pastTense(p.action),
		AddedVirtualFolders: p.added,
	}
}

func pastTense(action string) string {
	switch action {
	case model.SFTPActionCreate:
		return model.SFTPActionCreated
	case model.SFTPActionUpdate:
		return model.SFTPActionUpdated
	default:
		return model.SFTPActionUnchanged
	}
}

func stringList(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}

	return slices.Clone(items)
}
