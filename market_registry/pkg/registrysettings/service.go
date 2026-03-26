package registrysettings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
	"sync"

	"auralogic/market_registry/pkg/storage"
)

const (
	SettingsPath                      = "admin/settings.json"
	CanonicalArtifactStorageProfileID = "canonical"
)

var artifactStorageProfileIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type Document struct {
	Version         int                     `json:"version"`
	ArtifactStorage ArtifactStorageSettings `json:"artifact_storage"`
}

type ArtifactStorageSettings struct {
	DefaultProfileID string                   `json:"default_profile_id"`
	Profiles         []ArtifactStorageProfile `json:"profiles"`
}

type ArtifactStorageProfile struct {
	ID                     string `json:"id"`
	OriginalID             string `json:"original_id,omitempty"`
	Name                   string `json:"name"`
	Type                   string `json:"type"`
	Description            string `json:"description,omitempty"`
	BaseDir                string `json:"base_dir,omitempty"`
	BaseURL                string `json:"base_url,omitempty"`
	S3Endpoint             string `json:"s3_endpoint,omitempty"`
	S3Region               string `json:"s3_region,omitempty"`
	S3Bucket               string `json:"s3_bucket,omitempty"`
	S3Prefix               string `json:"s3_prefix,omitempty"`
	S3AccessKeyID          string `json:"s3_access_key_id,omitempty"`
	S3SecretAccessKey      string `json:"s3_secret_access_key,omitempty"`
	S3SessionToken         string `json:"s3_session_token,omitempty"`
	HasS3SecretAccessKey   bool   `json:"has_s3_secret_access_key,omitempty"`
	HasS3SessionToken      bool   `json:"has_s3_session_token,omitempty"`
	ClearS3SecretAccessKey bool   `json:"clear_s3_secret_access_key,omitempty"`
	ClearS3SessionToken    bool   `json:"clear_s3_session_token,omitempty"`
	S3UsePathStyle         bool   `json:"s3_use_path_style,omitempty"`
	WebDAVEndpoint         string `json:"webdav_endpoint,omitempty"`
	WebDAVUsername         string `json:"webdav_username,omitempty"`
	WebDAVPassword         string `json:"webdav_password,omitempty"`
	HasWebDAVPassword      bool   `json:"has_webdav_password,omitempty"`
	ClearWebDAVPassword    bool   `json:"clear_webdav_password,omitempty"`
	WebDAVSkipVerify       bool   `json:"webdav_skip_verify,omitempty"`
	FTPAddress             string `json:"ftp_address,omitempty"`
	FTPUsername            string `json:"ftp_username,omitempty"`
	FTPPassword            string `json:"ftp_password,omitempty"`
	HasFTPPassword         bool   `json:"has_ftp_password,omitempty"`
	ClearFTPPassword       bool   `json:"clear_ftp_password,omitempty"`
	FTPRootDir             string `json:"ftp_root_dir,omitempty"`
	FTPSecurity            string `json:"ftp_security,omitempty"`
	FTPSkipVerify          bool   `json:"ftp_skip_verify,omitempty"`
	Builtin                bool   `json:"builtin,omitempty"`
	ReadOnly               bool   `json:"read_only,omitempty"`
}

type ResolvedArtifactStorage struct {
	Profile ArtifactStorageProfile
	Store   storage.Storage
}

type Service struct {
	store            storage.Storage
	canonicalProfile ArtifactStorageProfile

	mu    sync.RWMutex
	cache map[string]cachedStorage
}

type cachedStorage struct {
	fingerprint string
	store       storage.Storage
}

func NewService(store storage.Storage, canonicalProfile ArtifactStorageProfile) *Service {
	if store == nil {
		panic("registrysettings: storage is required")
	}
	canonicalProfile = normalizeProfile(canonicalProfile)
	canonicalProfile.ID = CanonicalArtifactStorageProfileID
	canonicalProfile.Name = firstNonEmpty(canonicalProfile.Name, "Canonical Storage")
	canonicalProfile.Type = firstNonEmpty(canonicalProfile.Type, "local")
	canonicalProfile.Builtin = true
	canonicalProfile.ReadOnly = true
	return &Service{
		store:            store,
		canonicalProfile: canonicalProfile,
		cache:            map[string]cachedStorage{},
	}
}

func (s *Service) Get(ctx context.Context) (Document, error) {
	stored, err := s.readStoredDocument(ctx)
	if err != nil {
		return Document{}, err
	}
	return s.decorateDocument(stored), nil
}

func (s *Service) Update(ctx context.Context, next Document) (Document, error) {
	existing, err := s.readStoredDocument(ctx)
	if err != nil {
		return Document{}, err
	}
	stored, err := s.mergeUpdateDocument(next, existing)
	if err != nil {
		return Document{}, err
	}

	payload, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return Document{}, fmt.Errorf("marshal settings: %w", err)
	}
	if err := s.store.Write(ctx, SettingsPath, payload); err != nil {
		return Document{}, fmt.Errorf("write settings: %w", err)
	}

	s.resetCache()
	return s.decorateDocument(stored), nil
}

func (s *Service) ResolveArtifactStorage(ctx context.Context, profileID string) (ResolvedArtifactStorage, error) {
	stored, err := s.readStoredDocument(ctx)
	if err != nil {
		return ResolvedArtifactStorage{}, err
	}

	resolvedID := strings.TrimSpace(profileID)
	if resolvedID == "" {
		resolvedID = strings.TrimSpace(stored.ArtifactStorage.DefaultProfileID)
	}
	if resolvedID == "" || strings.EqualFold(resolvedID, CanonicalArtifactStorageProfileID) {
		return ResolvedArtifactStorage{
			Profile: s.canonicalProfile,
			Store:   s.store,
		}, nil
	}

	for _, profile := range stored.ArtifactStorage.Profiles {
		if !strings.EqualFold(profile.ID, resolvedID) {
			continue
		}
		instance, err := s.resolveCustomStorage(profile)
		if err != nil {
			return ResolvedArtifactStorage{}, err
		}
		return ResolvedArtifactStorage{
			Profile: profile,
			Store:   instance,
		}, nil
	}

	return ResolvedArtifactStorage{}, fmt.Errorf("artifact storage profile %q was not found", resolvedID)
}

func (s *Service) DefaultArtifactStorageProfileID(ctx context.Context) (string, error) {
	stored, err := s.readStoredDocument(ctx)
	if err != nil {
		return "", err
	}
	return firstNonEmpty(stored.ArtifactStorage.DefaultProfileID, CanonicalArtifactStorageProfileID), nil
}

func (s *Service) readStoredDocument(ctx context.Context) (Document, error) {
	payload, err := s.store.Read(ctx, SettingsPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return s.normalizeStoredDocument(Document{})
		}
		return Document{}, fmt.Errorf("read settings: %w", err)
	}

	var raw Document
	if err := json.Unmarshal(payload, &raw); err != nil {
		return Document{}, fmt.Errorf("decode settings: %w", err)
	}
	return s.normalizeStoredDocument(raw)
}

func (s *Service) normalizeStoredDocument(raw Document) (Document, error) {
	out := Document{
		Version: 1,
		ArtifactStorage: ArtifactStorageSettings{
			DefaultProfileID: CanonicalArtifactStorageProfileID,
			Profiles:         []ArtifactStorageProfile{},
		},
	}

	defaultProfileID := strings.TrimSpace(raw.ArtifactStorage.DefaultProfileID)
	profiles := make([]ArtifactStorageProfile, 0, len(raw.ArtifactStorage.Profiles))
	seen := map[string]struct{}{}

	for _, candidate := range raw.ArtifactStorage.Profiles {
		profile := normalizeProfile(candidate)
		if profile.Builtin || strings.EqualFold(profile.ID, CanonicalArtifactStorageProfileID) {
			continue
		}
		if profile.ID == "" {
			return Document{}, fmt.Errorf("artifact storage profile id is required")
		}
		if !artifactStorageProfileIDPattern.MatchString(profile.ID) {
			return Document{}, fmt.Errorf("artifact storage profile id %q is invalid", profile.ID)
		}
		lookupKey := strings.ToLower(profile.ID)
		if _, exists := seen[lookupKey]; exists {
			return Document{}, fmt.Errorf("artifact storage profile id %q is duplicated", profile.ID)
		}
		seen[lookupKey] = struct{}{}

		if profile.Name == "" {
			profile.Name = profile.ID
		}
		switch profile.Type {
		case "local":
			clearS3Fields(&profile)
			clearWebDAVFields(&profile)
			clearFTPFields(&profile)
			if profile.BaseDir == "" {
				return Document{}, fmt.Errorf("artifact storage profile %q requires base_dir", profile.ID)
			}
		case "s3":
			clearWebDAVFields(&profile)
			clearFTPFields(&profile)
			if profile.S3Endpoint == "" {
				return Document{}, fmt.Errorf("artifact storage profile %q requires s3_endpoint", profile.ID)
			}
			if profile.S3Bucket == "" {
				return Document{}, fmt.Errorf("artifact storage profile %q requires s3_bucket", profile.ID)
			}
			if profile.S3AccessKeyID == "" {
				return Document{}, fmt.Errorf("artifact storage profile %q requires s3_access_key_id", profile.ID)
			}
			if profile.S3SecretAccessKey == "" {
				return Document{}, fmt.Errorf("artifact storage profile %q requires s3_secret_access_key", profile.ID)
			}
		case "webdav":
			clearS3Fields(&profile)
			clearFTPFields(&profile)
			if profile.WebDAVEndpoint == "" {
				return Document{}, fmt.Errorf("artifact storage profile %q requires webdav_endpoint", profile.ID)
			}
		case "ftp":
			clearS3Fields(&profile)
			clearWebDAVFields(&profile)
			if profile.FTPAddress == "" {
				return Document{}, fmt.Errorf("artifact storage profile %q requires ftp_address", profile.ID)
			}
			if profile.FTPSecurity == "" {
				profile.FTPSecurity = "plain"
			}
			switch profile.FTPSecurity {
			case "plain", "explicit_tls", "implicit_tls":
			default:
				return Document{}, fmt.Errorf("artifact storage profile %q uses unsupported ftp_security %q", profile.ID, profile.FTPSecurity)
			}
		default:
			return Document{}, fmt.Errorf("artifact storage profile %q uses unsupported type %q", profile.ID, profile.Type)
		}
		profile.OriginalID = ""
		profile.HasS3SecretAccessKey = false
		profile.HasS3SessionToken = false
		profile.ClearS3SecretAccessKey = false
		profile.ClearS3SessionToken = false
		profile.HasWebDAVPassword = false
		profile.ClearWebDAVPassword = false
		profile.HasFTPPassword = false
		profile.ClearFTPPassword = false
		profile.Builtin = false
		profile.ReadOnly = false
		profiles = append(profiles, profile)
	}

	if strings.EqualFold(defaultProfileID, CanonicalArtifactStorageProfileID) || defaultProfileID == "" {
		out.ArtifactStorage.DefaultProfileID = CanonicalArtifactStorageProfileID
	} else {
		if _, exists := seen[strings.ToLower(defaultProfileID)]; !exists {
			return Document{}, fmt.Errorf("default artifact storage profile %q was not found", defaultProfileID)
		}
		out.ArtifactStorage.DefaultProfileID = defaultProfileID
	}
	out.ArtifactStorage.Profiles = profiles
	return out, nil
}

func (s *Service) decorateDocument(stored Document) Document {
	normalized, err := s.normalizeStoredDocument(stored)
	if err != nil {
		return Document{
			Version: 1,
			ArtifactStorage: ArtifactStorageSettings{
				DefaultProfileID: CanonicalArtifactStorageProfileID,
				Profiles:         []ArtifactStorageProfile{s.canonicalProfile},
			},
		}
	}

	profiles := make([]ArtifactStorageProfile, 0, len(normalized.ArtifactStorage.Profiles)+1)
	profiles = append(profiles, sanitizeProfileForOutput(s.canonicalProfile, false))
	for _, profile := range normalized.ArtifactStorage.Profiles {
		profiles = append(profiles, sanitizeProfileForOutput(profile, true))
	}
	normalized.ArtifactStorage.Profiles = profiles
	return normalized
}

func (s *Service) mergeUpdateDocument(next Document, existing Document) (Document, error) {
	existingProfiles := make(map[string]ArtifactStorageProfile, len(existing.ArtifactStorage.Profiles))
	for _, profile := range existing.ArtifactStorage.Profiles {
		if profile.ID == "" {
			continue
		}
		existingProfiles[strings.ToLower(profile.ID)] = profile
	}

	mergedProfiles := make([]ArtifactStorageProfile, 0, len(next.ArtifactStorage.Profiles))
	for _, candidate := range next.ArtifactStorage.Profiles {
		profile := normalizeProfile(candidate)
		lookupID := firstNonEmpty(profile.OriginalID, profile.ID)
		current := existingProfiles[strings.ToLower(lookupID)]

		switch profile.Type {
		case "s3":
			if profile.ClearS3SecretAccessKey {
				profile.S3SecretAccessKey = ""
			} else if profile.S3SecretAccessKey == "" {
				profile.S3SecretAccessKey = current.S3SecretAccessKey
			}
			if profile.ClearS3SessionToken {
				profile.S3SessionToken = ""
			} else if profile.S3SessionToken == "" {
				profile.S3SessionToken = current.S3SessionToken
			}
			clearWebDAVFields(&profile)
			clearFTPFields(&profile)
		case "webdav":
			if profile.ClearWebDAVPassword {
				profile.WebDAVPassword = ""
			} else if profile.WebDAVPassword == "" {
				profile.WebDAVPassword = current.WebDAVPassword
			}
			clearS3Fields(&profile)
			clearFTPFields(&profile)
		case "ftp":
			if profile.ClearFTPPassword {
				profile.FTPPassword = ""
			} else if profile.FTPPassword == "" {
				profile.FTPPassword = current.FTPPassword
			}
			clearS3Fields(&profile)
			clearWebDAVFields(&profile)
		case "local":
			clearS3Fields(&profile)
			clearWebDAVFields(&profile)
			clearFTPFields(&profile)
		}

		profile.OriginalID = ""
		profile.HasS3SecretAccessKey = false
		profile.HasS3SessionToken = false
		profile.ClearS3SecretAccessKey = false
		profile.ClearS3SessionToken = false
		profile.HasWebDAVPassword = false
		profile.ClearWebDAVPassword = false
		profile.HasFTPPassword = false
		profile.ClearFTPPassword = false
		mergedProfiles = append(mergedProfiles, profile)
	}

	return s.normalizeStoredDocument(Document{
		Version: next.Version,
		ArtifactStorage: ArtifactStorageSettings{
			DefaultProfileID: next.ArtifactStorage.DefaultProfileID,
			Profiles:         mergedProfiles,
		},
	})
}

func (s *Service) resolveCustomStorage(profile ArtifactStorageProfile) (storage.Storage, error) {
	fingerprint, err := fingerprintProfile(profile)
	if err != nil {
		return nil, fmt.Errorf("fingerprint artifact storage profile %q: %w", profile.ID, err)
	}

	s.mu.RLock()
	cached, ok := s.cache[profile.ID]
	s.mu.RUnlock()
	if ok && cached.fingerprint == fingerprint && cached.store != nil {
		return cached.store, nil
	}

	instance, err := storage.New(storage.Config{
		Type:              profile.Type,
		BaseDir:           profile.BaseDir,
		BaseURL:           profile.BaseURL,
		S3Endpoint:        profile.S3Endpoint,
		S3Region:          profile.S3Region,
		S3Bucket:          profile.S3Bucket,
		S3Prefix:          profile.S3Prefix,
		S3AccessKeyID:     profile.S3AccessKeyID,
		S3SecretAccessKey: profile.S3SecretAccessKey,
		S3SessionToken:    profile.S3SessionToken,
		S3UsePathStyle:    profile.S3UsePathStyle,
		WebDAVEndpoint:    profile.WebDAVEndpoint,
		WebDAVUsername:    profile.WebDAVUsername,
		WebDAVPassword:    profile.WebDAVPassword,
		WebDAVSkipVerify:  profile.WebDAVSkipVerify,
		FTPAddress:        profile.FTPAddress,
		FTPUsername:       profile.FTPUsername,
		FTPPassword:       profile.FTPPassword,
		FTPRootDir:        profile.FTPRootDir,
		FTPSecurity:       profile.FTPSecurity,
		FTPSkipVerify:     profile.FTPSkipVerify,
	})
	if err != nil {
		return nil, fmt.Errorf("build artifact storage profile %q: %w", profile.ID, err)
	}

	s.mu.Lock()
	s.cache[profile.ID] = cachedStorage{
		fingerprint: fingerprint,
		store:       instance,
	}
	s.mu.Unlock()
	return instance, nil
}

func (s *Service) resetCache() {
	s.mu.Lock()
	s.cache = map[string]cachedStorage{}
	s.mu.Unlock()
}

func fingerprintProfile(profile ArtifactStorageProfile) (string, error) {
	normalized := normalizeProfile(profile)
	normalized.Builtin = false
	normalized.ReadOnly = false
	payload, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func normalizeProfile(profile ArtifactStorageProfile) ArtifactStorageProfile {
	profile.ID = strings.TrimSpace(profile.ID)
	profile.OriginalID = strings.TrimSpace(profile.OriginalID)
	profile.Name = strings.TrimSpace(profile.Name)
	profile.Type = strings.ToLower(strings.TrimSpace(profile.Type))
	profile.Description = strings.TrimSpace(profile.Description)
	profile.BaseDir = strings.TrimSpace(profile.BaseDir)
	profile.BaseURL = strings.TrimSpace(profile.BaseURL)
	profile.S3Endpoint = strings.TrimSpace(profile.S3Endpoint)
	profile.S3Region = strings.TrimSpace(profile.S3Region)
	profile.S3Bucket = strings.TrimSpace(profile.S3Bucket)
	profile.S3Prefix = strings.TrimSpace(profile.S3Prefix)
	profile.S3AccessKeyID = strings.TrimSpace(profile.S3AccessKeyID)
	profile.S3SecretAccessKey = strings.TrimSpace(profile.S3SecretAccessKey)
	profile.S3SessionToken = strings.TrimSpace(profile.S3SessionToken)
	profile.WebDAVEndpoint = strings.TrimSpace(profile.WebDAVEndpoint)
	profile.WebDAVUsername = strings.TrimSpace(profile.WebDAVUsername)
	profile.WebDAVPassword = strings.TrimSpace(profile.WebDAVPassword)
	profile.FTPAddress = strings.TrimSpace(profile.FTPAddress)
	profile.FTPUsername = strings.TrimSpace(profile.FTPUsername)
	profile.FTPPassword = strings.TrimSpace(profile.FTPPassword)
	profile.FTPRootDir = strings.TrimSpace(profile.FTPRootDir)
	profile.FTPSecurity = strings.ToLower(strings.TrimSpace(profile.FTPSecurity))
	return profile
}

func sanitizeProfileForOutput(profile ArtifactStorageProfile, editable bool) ArtifactStorageProfile {
	profile = normalizeProfile(profile)
	profile.OriginalID = ""
	if editable {
		profile.OriginalID = profile.ID
	}
	profile.HasS3SecretAccessKey = profile.S3SecretAccessKey != ""
	profile.HasS3SessionToken = profile.S3SessionToken != ""
	profile.S3SecretAccessKey = ""
	profile.S3SessionToken = ""
	profile.ClearS3SecretAccessKey = false
	profile.ClearS3SessionToken = false
	profile.HasWebDAVPassword = profile.WebDAVPassword != ""
	profile.WebDAVPassword = ""
	profile.ClearWebDAVPassword = false
	profile.HasFTPPassword = profile.FTPPassword != ""
	profile.FTPPassword = ""
	profile.ClearFTPPassword = false
	return profile
}

func clearS3Fields(profile *ArtifactStorageProfile) {
	if profile == nil {
		return
	}
	profile.S3Endpoint = ""
	profile.S3Region = ""
	profile.S3Bucket = ""
	profile.S3Prefix = ""
	profile.S3AccessKeyID = ""
	profile.S3SecretAccessKey = ""
	profile.S3SessionToken = ""
	profile.HasS3SecretAccessKey = false
	profile.HasS3SessionToken = false
	profile.ClearS3SecretAccessKey = false
	profile.ClearS3SessionToken = false
	profile.S3UsePathStyle = false
}

func clearWebDAVFields(profile *ArtifactStorageProfile) {
	if profile == nil {
		return
	}
	profile.WebDAVEndpoint = ""
	profile.WebDAVUsername = ""
	profile.WebDAVPassword = ""
	profile.HasWebDAVPassword = false
	profile.ClearWebDAVPassword = false
	profile.WebDAVSkipVerify = false
}

func clearFTPFields(profile *ArtifactStorageProfile) {
	if profile == nil {
		return
	}
	profile.FTPAddress = ""
	profile.FTPUsername = ""
	profile.FTPPassword = ""
	profile.HasFTPPassword = false
	profile.ClearFTPPassword = false
	profile.FTPRootDir = ""
	profile.FTPSecurity = ""
	profile.FTPSkipVerify = false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
