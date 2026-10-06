package model

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// SFTPPasswordGenerate asks the API to create a one-time random password.
	SFTPPasswordGenerate = "generate"
	// SFTPPasswordNone creates the account with public keys and no password.
	SFTPPasswordNone = "none"

	// SFTPActionCreate is a preview of a user that does not exist yet.
	SFTPActionCreate = "create"
	// SFTPActionUpdate is a preview of an existing user that needs new folders.
	SFTPActionUpdate = "update"
	// SFTPActionUnchanged means the requested base is already configured.
	SFTPActionUnchanged = "unchanged"
	// SFTPActionCreated is the apply result for a new user.
	SFTPActionCreated = "created"
	// SFTPActionUpdated is the apply result for an extended user.
	SFTPActionUpdated = "updated"

	// SFTPFolderExists means the virtual folder is already on the expected bucket.
	SFTPFolderExists = "exists"

	// DefaultSFTPHomeRoot is the SFTPGo data directory used when none is configured.
	DefaultSFTPHomeRoot = "/srv/sftpgo/data"

	sftpSubfolderStop = "stop"
)

// sftpNamePattern matches the script's user and base names.
var sftpNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// SFTPGoConfig is the SFTPGo connection. URL and APIKey come from SFTPGO_URL and
// SFTPGO_API_KEY. The API key is a Secret Manager value mounted as an env var.
// HomeRoot is the prefix for a new user's home_dir.
type SFTPGoConfig struct {
	URL      string `mapstructure:"url"`
	APIKey   string `mapstructure:"api_key"`
	HomeRoot string `mapstructure:"home_root"`
}

// Configured reports whether both the URL and the API key are set.
func (c SFTPGoConfig) Configured() bool {
	return strings.TrimSpace(c.URL) != "" && strings.TrimSpace(c.APIKey) != ""
}

// SFTPAccountRequest is the body of the preview and create routes.
type SFTPAccountRequest struct {
	User         string   `json:"user"`
	Base         string   `json:"base"`
	ClientType   string   `json:"clientType"`
	PasswordMode string   `json:"passwordMode,omitempty"`
	PublicKeys   []string `json:"publicKeys,omitempty"`
}

// SFTPConfigView is the body of GET /api/v1/sftp-accounts/config.
type SFTPConfigView struct {
	Configured  bool            `json:"configured"`
	ClientTypes SFTPClientTypes `json:"clientTypes"`
	Buckets     SFTPBuckets     `json:"buckets"`
}

// SFTPClientTypes lists the sub-folders created for each client type.
type SFTPClientTypes struct {
	Publisher  []string `json:"publisher"`
	Advertiser []string `json:"advertiser"`
}

// SFTPBuckets are the raw GCS buckets used for each client type.
type SFTPBuckets struct {
	Publisher  string `json:"publisher"`
	Advertiser string `json:"advertiser"`
}

// SFTPAccountView is the body of GET /api/v1/sftp-accounts/{username}.
type SFTPAccountView struct {
	Username       string              `json:"username"`
	Exists         bool                `json:"exists"`
	Bucket         string              `json:"bucket,omitempty"`
	VirtualFolders []SFTPVirtualFolder `json:"virtualFolders"`
	Bases          []string            `json:"bases"`
}

// SFTPVirtualFolder is one folder mounted on a user.
type SFTPVirtualFolder struct {
	Name        string `json:"name"`
	VirtualPath string `json:"virtualPath"`
}

// SFTPPlannedFolder is one folder in a dry-run plan.
type SFTPPlannedFolder struct {
	Name        string `json:"name"`
	VirtualPath string `json:"virtualPath"`
	Action      string `json:"action"`
}

// SFTPPlan is the body of POST /api/v1/sftp-accounts/preview.
type SFTPPlan struct {
	User       string              `json:"user"`
	Base       string              `json:"base"`
	ClientType string              `json:"clientType"`
	Bucket     string              `json:"bucket"`
	Subfolders []string            `json:"subfolders"`
	Folders    []SFTPPlannedFolder `json:"folders"`
	UserAction string              `json:"userAction"`
	Warnings   []string            `json:"warnings"`
}

// SFTPApplyResult is the body of POST /api/v1/sftp-accounts.
// GeneratedPassword is returned once and must not be logged or stored.
type SFTPApplyResult struct {
	User                string   `json:"user"`
	Base                string   `json:"base"`
	ClientType          string   `json:"clientType"`
	Bucket              string   `json:"bucket"`
	Subfolders          []string `json:"subfolders"`
	FoldersCreated      []string `json:"foldersCreated"`
	FoldersExisting     []string `json:"foldersExisting"`
	UserAction          string   `json:"userAction"`
	AddedVirtualFolders int      `json:"addedVirtualFolders"`
	GeneratedPassword   string   `json:"generatedPassword,omitempty"`
	Verified            bool     `json:"verified"`
	Missing             []string `json:"missing,omitempty"`
}

// SFTPSubfolders returns the virtual sub-folders for a client type.
// The bool is false for anything other than publisher or advertiser.
// Referential is intentionally unsupported.
func SFTPSubfolders(clientType string) ([]string, bool) {
	switch clientType {
	case PartnerPublisher:
		return []string{ImportOptin, ImportOptout, sftpSubfolderStop}, true
	case PartnerAdvertiser:
		return []string{ImportBlacklists, ImportCustomers, ImportStores, ImportSales}, true
	default:
		return nil, false
	}
}

// Normalized trims the request and applies the default password mode.
func (r SFTPAccountRequest) Normalized() SFTPAccountRequest {
	r.User = strings.TrimSpace(r.User)
	r.Base = strings.TrimSpace(r.Base)
	r.ClientType = strings.ToLower(strings.TrimSpace(r.ClientType))
	r.PasswordMode = strings.ToLower(strings.TrimSpace(r.PasswordMode))

	if r.PasswordMode == "" {
		r.PasswordMode = SFTPPasswordGenerate
	}

	keys := make([]string, 0, len(r.PublicKeys))
	for _, key := range r.PublicKeys {
		trimmed := strings.TrimSpace(key)
		if trimmed != "" {
			keys = append(keys, trimmed)
		}
	}

	r.PublicKeys = keys

	return r
}

// Issues reports request problems. An empty slice means the request can be planned.
func (r SFTPAccountRequest) Issues() []FieldIssue {
	r = r.Normalized()

	issues := make([]FieldIssue, 0)
	if !sftpNamePattern.MatchString(r.User) {
		issues = append(issues, FieldIssue{
			Field:   "user",
			Message: invalidSFTPName("user", r.User),
		})
	}

	if !sftpNamePattern.MatchString(r.Base) {
		issues = append(issues, FieldIssue{
			Field:   "base",
			Message: invalidSFTPName("base", r.Base),
		})
	}

	if _, known := SFTPSubfolders(r.ClientType); !known {
		issues = append(issues, FieldIssue{
			Field:   "clientType",
			Message: "clientType must be " + PartnerPublisher + " or " + PartnerAdvertiser,
		})
	}

	if r.PasswordMode != SFTPPasswordGenerate && r.PasswordMode != SFTPPasswordNone {
		issues = append(issues, FieldIssue{
			Field:   "passwordMode",
			Message: "passwordMode must be " + SFTPPasswordGenerate + " or " + SFTPPasswordNone,
		})
	}

	if r.PasswordMode == SFTPPasswordNone && len(r.PublicKeys) == 0 {
		issues = append(issues, FieldIssue{
			Field:   "passwordMode",
			Message: "passwordMode none requires at least one public key",
		})
	}

	return issues
}

func invalidSFTPName(label, value string) string {
	return fmt.Sprintf("invalid %s %q (lowercase letters, digits, '-', '_')", label, value)
}

// BucketMismatchError means a folder or user already lives on another GCS bucket.
type BucketMismatchError struct {
	Subject  string
	Name     string
	Actual   string
	Expected string
}

func (e *BucketMismatchError) Error() string {
	if e.Subject == "folder" {
		return fmt.Sprintf("folder %s exists on bucket %s, expected %s", e.Name, e.Actual, e.Expected)
	}

	return fmt.Sprintf("user %s is on bucket %s, expected %s", e.Name, e.Actual, e.Expected)
}

// SFTPGoError is an unexpected response or transport failure from SFTPGo.
// Message includes the trimmed upstream body and must not contain the API key.
type SFTPGoError struct {
	Status  int
	Message string
}

func (e *SFTPGoError) Error() string {
	return e.Message
}
