package sftpgo

import (
	"crypto/rand"
	"math/big"
	"strings"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

const (
	passwordAlphabet      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.!@#%+="
	passwordLength        = 24
	filesystemProviderGCS = 2
	automaticCredentials  = 1
	userStatusActive      = 1
	quotaIncluded         = -1

	warningPasswordKept = "existing password is kept"
	warningKeysIgnored  = "public keys are only applied when the user is created"
)

type plannedFolder struct {
	name        string
	virtualPath string
	sub         string
	action      string
}

func gcsFilesystem(bucket, prefix string) map[string]any {
	return map[string]any{
		"provider": filesystemProviderGCS,
		"gcsconfig": map[string]any{
			"bucket":                bucket,
			"key_prefix":            prefix,
			"automatic_credentials": automaticCredentials,
		},
	}
}

func newVirtualFolder(folder plannedFolder) map[string]any {
	return map[string]any{
		"name":         folder.name,
		"virtual_path": folder.virtualPath,
		"quota_size":   quotaIncluded,
		"quota_files":  quotaIncluded,
	}
}

func folderPayload(user, base, bucket string, folder plannedFolder) map[string]any {
	return map[string]any{
		"name":        folder.name,
		"description": user + " - " + base + " - " + folder.sub,
		"filesystem":  gcsFilesystem(bucket, folder.name+"/"),
	}
}

func (s *Service) homeDir(user string) string {
	root := strings.TrimRight(strings.TrimSpace(s.homeRoot), "/")
	if root == "" {
		root = model.DefaultSFTPHomeRoot
	}

	return root + "/" + user
}

func newUserPayload(home string, plan *accountPlan, password string) map[string]any {
	perms := map[string][]string{rootPath: listPermissions()}

	folders := make([]map[string]any, 0, len(plan.folders))
	for _, folder := range plan.folders {
		perms[folder.virtualPath] = uploadPermissions()
		folders = append(folders, newVirtualFolder(folder))
	}

	body := map[string]any{
		"username":        plan.req.User,
		"status":          userStatusActive,
		"description":     plan.req.User,
		"home_dir":        home,
		"filesystem":      gcsFilesystem(plan.bucket, plan.req.User+"/"),
		"permissions":     perms,
		"virtual_folders": folders,
	}
	if len(plan.req.PublicKeys) > 0 {
		body["public_keys"] = plan.req.PublicKeys
	}

	if password != "" {
		body["password"] = password
	}

	return body
}

func updatePayload(user map[string]any, folders []plannedFolder, perms map[string][]string) map[string]any {
	delete(user, "password")

	existing := slimVirtualFolders(user)

	present := make(map[string]struct{}, len(existing))
	for _, folder := range existing {
		present[stringField(folder, "virtual_path")] = struct{}{}
	}

	for _, folder := range folders {
		if _, ok := present[folder.virtualPath]; ok {
			continue
		}

		existing = append(existing, newVirtualFolder(folder))
	}

	user["virtual_folders"] = existing
	user["permissions"] = perms

	return user
}

func generatePassword() (string, error) {
	var builder strings.Builder

	builder.Grow(passwordLength)

	maxN := big.NewInt(int64(len(passwordAlphabet)))
	for range passwordLength {
		n, err := rand.Int(rand.Reader, maxN)
		if err != nil {
			return "", err
		}

		builder.WriteByte(passwordAlphabet[n.Int64()])
	}

	return builder.String(), nil
}
