package sftpgo

const (
	permList   = "list"
	permUpload = "upload"
	rootPath   = "/"
)

func stringField(doc map[string]any, key string) string {
	if doc == nil {
		return ""
	}

	value, _ := doc[key].(string)

	return value
}

func gcsBucket(doc map[string]any) string {
	filesystem, _ := doc["filesystem"].(map[string]any)
	if filesystem == nil {
		return ""
	}

	gcs, _ := filesystem["gcsconfig"].(map[string]any)
	if gcs == nil {
		return ""
	}

	return stringField(gcs, "bucket")
}

func virtualFolderMaps(user map[string]any) []map[string]any {
	raw, _ := user["virtual_folders"].([]any)

	folders := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		folder, ok := item.(map[string]any)
		if !ok {
			continue
		}

		folders = append(folders, folder)
	}

	return folders
}

func slimVirtualFolders(user map[string]any) []map[string]any {
	folders := virtualFolderMaps(user)

	slim := make([]map[string]any, 0, len(folders))
	for _, folder := range folders {
		slim = append(slim, slimVirtualFolder(folder))
	}

	return slim
}

func slimVirtualFolder(folder map[string]any) map[string]any {
	slim := make(map[string]any, 4)

	for _, key := range []string{"name", "virtual_path", "quota_size", "quota_files"} {
		value, ok := folder[key]
		if ok {
			slim[key] = value
		}
	}

	return slim
}

func virtualPaths(user map[string]any) map[string]struct{} {
	paths := make(map[string]struct{})

	for _, folder := range virtualFolderMaps(user) {
		path := stringField(folder, "virtual_path")
		if path != "" {
			paths[path] = struct{}{}
		}
	}

	return paths
}

func permissionMap(user map[string]any) map[string][]string {
	raw, _ := user["permissions"].(map[string]any)

	perms := make(map[string][]string, len(raw))
	for path, value := range raw {
		perms[path] = stringSlice(value)
	}

	return perms
}

func stringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string{}, typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if ok {
				out = append(out, text)
			}
		}

		return out
	default:
		return []string{}
	}
}

func mergedPermissions(user map[string]any, folders []plannedFolder) map[string][]string {
	perms := permissionMap(user)
	for _, folder := range folders {
		if _, exists := perms[folder.virtualPath]; exists {
			continue
		}

		perms[folder.virtualPath] = uploadPermissions()
	}

	return perms
}

func permissionsEqual(left, right map[string][]string) bool {
	if len(left) != len(right) {
		return false
	}

	for path, values := range left {
		other, ok := right[path]
		if !ok || !stringSlicesEqual(values, other) {
			return false
		}
	}

	return true
}

func stringSlicesEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}

	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}

	return true
}

func uploadPermissions() []string {
	return []string{permList, permUpload}
}

func listPermissions() []string {
	return []string{permList}
}
