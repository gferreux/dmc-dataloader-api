package sftpgo

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/port"
)

// Config selects the SFTPGo server and the raw buckets for each client type.
type Config struct {
	URL              string
	APIKey           string
	HomeRoot         string
	PublisherBucket  string
	AdvertiserBucket string
	HTTPClient       *http.Client
}

// Service creates and extends SFTPGo accounts.
type Service struct {
	client           *Client
	homeRoot         string
	publisherBucket  string
	advertiserBucket string
	configured       bool
	logger           *slog.Logger
}

// NewService builds the account service. A missing URL or API key leaves it unconfigured.
func NewService(cfg Config, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	home := strings.TrimSpace(cfg.HomeRoot)
	if home == "" {
		home = model.DefaultSFTPHomeRoot
	}

	url := strings.TrimSpace(cfg.URL)
	apiKey := strings.TrimSpace(cfg.APIKey)
	configured := url != "" && apiKey != ""

	var client *Client
	if configured {
		client = NewClient(url, apiKey, cfg.HTTPClient)
	}

	return &Service{
		client:           client,
		homeRoot:         home,
		publisherBucket:  cfg.PublisherBucket,
		advertiserBucket: cfg.AdvertiserBucket,
		configured:       configured,
		logger:           logger,
	}
}

// ProvideService wires the service from process configuration.
// Raw buckets are the derive raw buckets for each client type.
func ProvideService(cfg model.AppConfig, logger *slog.Logger) *Service {
	return NewService(Config{
		URL:              cfg.SFTPGo.URL,
		APIKey:           cfg.SFTPGo.APIKey,
		HomeRoot:         cfg.SFTPGo.HomeRoot,
		PublisherBucket:  cfg.Derive.Publisher.RawBucket,
		AdvertiserBucket: cfg.Derive.Advertiser.RawBucket,
	}, logger)
}

// Config returns the form metadata. It succeeds when SFTPGo is not configured.
func (s *Service) Config() model.SFTPConfigView {
	publisher, _ := model.SFTPSubfolders(model.PartnerPublisher)
	advertiser, _ := model.SFTPSubfolders(model.PartnerAdvertiser)

	return model.SFTPConfigView{
		Configured: s.configured,
		ClientTypes: model.SFTPClientTypes{
			Publisher:  publisher,
			Advertiser: advertiser,
		},
		Buckets: model.SFTPBuckets{
			Publisher:  s.publisherBucket,
			Advertiser: s.advertiserBucket,
		},
	}
}

// Get reads one SFTPGo user. A missing user is exists=false, not an error.
func (s *Service) Get(ctx context.Context, username string) (model.SFTPAccountView, error) {
	view := emptyAccountView(username)
	if err := s.configuredErr(); err != nil {
		return view, err
	}

	user, found, err := s.client.GetUser(ctx, username)
	if err != nil {
		return view, err
	}

	if !found {
		return view, nil
	}

	view.Exists = true
	view.Bucket = gcsBucket(user)
	view.VirtualFolders = viewFolders(user)
	view.Bases = basesOf(view.VirtualFolders)

	return view, nil
}

// Preview plans the account change and does not write.
func (s *Service) Preview(ctx context.Context, req model.SFTPAccountRequest) (model.SFTPPlan, error) {
	plan, err := s.prepare(ctx, req)
	if err != nil {
		return model.SFTPPlan{}, err
	}

	return plan.view(), nil
}

// Apply creates or extends the account, then re-reads the user to verify the folders.
func (s *Service) Apply(ctx context.Context, req model.SFTPAccountRequest) (model.SFTPApplyResult, error) {
	plan, err := s.prepare(ctx, req)
	if err != nil {
		return model.SFTPApplyResult{}, err
	}

	result, err := s.execute(ctx, plan)
	if err != nil {
		return model.SFTPApplyResult{}, err
	}

	if err := s.verify(ctx, plan.paths(), &result); err != nil {
		return model.SFTPApplyResult{}, err
	}

	s.audit(ctx, result)

	return result, nil
}

func (s *Service) configuredErr() error {
	if !s.configured {
		return model.ErrSFTPGoUnconfigured
	}

	return nil
}

func (s *Service) bucketFor(clientType string) string {
	if clientType == model.PartnerPublisher {
		return s.publisherBucket
	}

	return s.advertiserBucket
}

func (s *Service) execute(ctx context.Context, plan *accountPlan) (model.SFTPApplyResult, error) {
	result := plan.applyResult()

	for _, folder := range plan.folders {
		if folder.action != model.SFTPActionCreate {
			continue
		}

		payload := folderPayload(plan.req.User, plan.req.Base, plan.bucket, folder)
		if err := s.client.CreateFolder(ctx, payload); err != nil {
			return model.SFTPApplyResult{}, err
		}
	}

	switch plan.action {
	case model.SFTPActionCreate:
		password, err := s.createUser(ctx, plan)
		if err != nil {
			return model.SFTPApplyResult{}, err
		}

		result.GeneratedPassword = password
	case model.SFTPActionUpdate:
		if err := s.updateUser(ctx, plan); err != nil {
			return model.SFTPApplyResult{}, err
		}
	}

	return result, nil
}

func (s *Service) createUser(ctx context.Context, plan *accountPlan) (string, error) {
	var password string

	if plan.req.PasswordMode == model.SFTPPasswordGenerate {
		generated, err := generatePassword()
		if err != nil {
			return "", err
		}

		password = generated
	}

	body := newUserPayload(s.homeDir(plan.req.User), plan, password)
	if err := s.client.CreateUser(ctx, body); err != nil {
		return "", err
	}

	return password, nil
}

func (s *Service) updateUser(ctx context.Context, plan *accountPlan) error {
	perms := mergedPermissions(plan.user, plan.folders)
	body := updatePayload(plan.user, plan.folders, perms)

	return s.client.UpdateUser(ctx, plan.req.User, body)
}

func (s *Service) verify(ctx context.Context, paths []string, result *model.SFTPApplyResult) error {
	user, found, err := s.client.GetUser(ctx, result.User)
	if err != nil {
		return err
	}

	got := map[string]struct{}{}
	if found {
		got = virtualPaths(user)
	}

	missing := make([]string, 0)

	for _, path := range paths {
		if _, ok := got[path]; !ok {
			missing = append(missing, path)
		}
	}

	result.Verified = len(missing) == 0
	if len(missing) > 0 {
		result.Missing = missing
	}

	return nil
}

// audit records who changed an account. The generated password is not a log field.
func (s *Service) audit(ctx context.Context, result model.SFTPApplyResult) {
	s.logger.Info(
		"sftp account "+result.UserAction,
		"action", result.UserAction,
		"user", result.User,
		"base", result.Base,
		"clientType", result.ClientType,
		"email", port.PrincipalFrom(ctx).Email,
		"verified", result.Verified,
	)
}

func emptyAccountView(username string) model.SFTPAccountView {
	return model.SFTPAccountView{
		Username:       username,
		VirtualFolders: []model.SFTPVirtualFolder{},
		Bases:          []string{},
	}
}

func viewFolders(user map[string]any) []model.SFTPVirtualFolder {
	raw := virtualFolderMaps(user)

	folders := make([]model.SFTPVirtualFolder, 0, len(raw))
	for _, folder := range raw {
		folders = append(folders, model.SFTPVirtualFolder{
			Name:        stringField(folder, "name"),
			VirtualPath: stringField(folder, "virtual_path"),
		})
	}

	return folders
}

func basesOf(folders []model.SFTPVirtualFolder) []string {
	seen := make(map[string]struct{})

	bases := make([]string, 0)

	for _, folder := range folders {
		base := baseFromVirtualPath(folder.VirtualPath)
		if base == "" {
			continue
		}

		if _, ok := seen[base]; ok {
			continue
		}

		seen[base] = struct{}{}

		bases = append(bases, base)
	}

	return bases
}

func baseFromVirtualPath(virtualPath string) string {
	parts := strings.Split(strings.Trim(virtualPath, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}

	return parts[0]
}
