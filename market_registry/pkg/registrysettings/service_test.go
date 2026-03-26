package registrysettings

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"auralogic/market_registry/pkg/storage"
)

func TestGetMasksSecretsAndUpdatePreservesRenamedProfileCredentials(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	service := NewService(store, ArtifactStorageProfile{
		ID:      CanonicalArtifactStorageProfileID,
		Name:    "Canonical Storage",
		Type:    "local",
		BaseDir: filepath.Join(root, "data"),
		BaseURL: "http://localhost:18080",
	})

	_, err = service.Update(ctx, Document{
		ArtifactStorage: ArtifactStorageSettings{
			DefaultProfileID: "s3-demo",
			Profiles: []ArtifactStorageProfile{
				{
					ID:                "s3-demo",
					Name:              "S3 Demo",
					Type:              "s3",
					S3Endpoint:        "https://s3.example.com",
					S3Region:          "us-east-1",
					S3Bucket:          "bucket-a",
					S3Prefix:          "registry",
					S3AccessKeyID:     "access-key",
					S3SecretAccessKey: "secret-v1",
					S3SessionToken:    "session-v1",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("initial Update returned error: %v", err)
	}

	got, err := service.Get(ctx)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	profile := findProfile(t, got, "s3-demo")
	if profile.S3SecretAccessKey != "" {
		t.Fatalf("expected S3 secret to be masked, got %q", profile.S3SecretAccessKey)
	}
	if profile.S3SessionToken != "" {
		t.Fatalf("expected session token to be masked, got %q", profile.S3SessionToken)
	}
	if !profile.HasS3SecretAccessKey {
		t.Fatalf("expected HasS3SecretAccessKey to be true")
	}
	if !profile.HasS3SessionToken {
		t.Fatalf("expected HasS3SessionToken to be true")
	}

	updated, err := service.Update(ctx, Document{
		ArtifactStorage: ArtifactStorageSettings{
			DefaultProfileID: "s3-demo-renamed",
			Profiles: []ArtifactStorageProfile{
				{
					ID:                     "s3-demo-renamed",
					OriginalID:             "s3-demo",
					Name:                   "S3 Demo Renamed",
					Type:                   "s3",
					S3Endpoint:             "https://s3.example.com",
					S3Region:               "us-east-1",
					S3Bucket:               "bucket-b",
					S3Prefix:               "registry/v2",
					S3AccessKeyID:          "access-key-2",
					ClearS3SessionToken:    true,
					S3SecretAccessKey:      "",
					S3SessionToken:         "",
					HasS3SecretAccessKey:   true,
					HasS3SessionToken:      true,
					ClearS3SecretAccessKey: false,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("renamed Update returned error: %v", err)
	}
	renamed := findProfile(t, updated, "s3-demo-renamed")
	if !renamed.HasS3SecretAccessKey {
		t.Fatalf("expected renamed profile to keep secret")
	}
	if renamed.HasS3SessionToken {
		t.Fatalf("expected renamed profile session token to be cleared")
	}

	body, err := store.Read(ctx, SettingsPath)
	if err != nil {
		t.Fatalf("Read settings returned error: %v", err)
	}
	var persisted Document
	if err := json.Unmarshal(body, &persisted); err != nil {
		t.Fatalf("Unmarshal persisted settings returned error: %v", err)
	}
	persistedProfile := findProfile(t, persisted, "s3-demo-renamed")
	if persistedProfile.S3SecretAccessKey != "secret-v1" {
		t.Fatalf("expected secret to be preserved, got %q", persistedProfile.S3SecretAccessKey)
	}
	if persistedProfile.S3SessionToken != "" {
		t.Fatalf("expected session token to be cleared, got %q", persistedProfile.S3SessionToken)
	}
	if persistedProfile.OriginalID != "" {
		t.Fatalf("expected OriginalID to stay transient, got %q", persistedProfile.OriginalID)
	}
}

func TestGetMasksAndUpdatePreservesWebDAVAndFTPCredentials(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		initialID      string
		renamedID      string
		initialProfile ArtifactStorageProfile
		renameProfile  func() ArtifactStorageProfile
		findMasked     func(t *testing.T, profile ArtifactStorageProfile)
		assertStored   func(t *testing.T, profile ArtifactStorageProfile, want string)
		clearProfile   func(id string) ArtifactStorageProfile
		assertCleared  func(t *testing.T, profile ArtifactStorageProfile)
	}{
		{
			name:      "webdav",
			initialID: "webdav-demo",
			renamedID: "webdav-demo-renamed",
			initialProfile: ArtifactStorageProfile{
				ID:               "webdav-demo",
				Name:             "WebDAV Demo",
				Type:             "webdav",
				WebDAVEndpoint:   "https://dav.example.com/repository",
				WebDAVUsername:   "demo-user",
				WebDAVPassword:   "dav-secret-v1",
				WebDAVSkipVerify: true,
			},
			renameProfile: func() ArtifactStorageProfile {
				return ArtifactStorageProfile{
					ID:                  "webdav-demo-renamed",
					OriginalID:          "webdav-demo",
					Name:                "WebDAV Demo Renamed",
					Type:                "webdav",
					WebDAVEndpoint:      "https://dav.example.com/repository/v2",
					WebDAVUsername:      "demo-user-2",
					WebDAVPassword:      "",
					HasWebDAVPassword:   true,
					ClearWebDAVPassword: false,
				}
			},
			findMasked: func(t *testing.T, profile ArtifactStorageProfile) {
				t.Helper()
				if profile.WebDAVPassword != "" {
					t.Fatalf("expected WebDAV password to be masked, got %q", profile.WebDAVPassword)
				}
				if !profile.HasWebDAVPassword {
					t.Fatalf("expected HasWebDAVPassword to be true")
				}
			},
			assertStored: func(t *testing.T, profile ArtifactStorageProfile, want string) {
				t.Helper()
				if profile.WebDAVPassword != want {
					t.Fatalf("expected WebDAV password %q, got %q", want, profile.WebDAVPassword)
				}
			},
			clearProfile: func(id string) ArtifactStorageProfile {
				return ArtifactStorageProfile{
					ID:                  id,
					OriginalID:          id,
					Name:                "WebDAV Demo Renamed",
					Type:                "webdav",
					WebDAVEndpoint:      "https://dav.example.com/repository/v2",
					WebDAVUsername:      "demo-user-2",
					ClearWebDAVPassword: true,
				}
			},
			assertCleared: func(t *testing.T, profile ArtifactStorageProfile) {
				t.Helper()
				if profile.WebDAVPassword != "" {
					t.Fatalf("expected WebDAV password to be cleared, got %q", profile.WebDAVPassword)
				}
			},
		},
		{
			name:      "ftp",
			initialID: "ftp-demo",
			renamedID: "ftp-demo-renamed",
			initialProfile: ArtifactStorageProfile{
				ID:            "ftp-demo",
				Name:          "FTP Demo",
				Type:          "ftp",
				FTPAddress:    "ftp.example.com:21",
				FTPUsername:   "demo-user",
				FTPPassword:   "ftp-secret-v1",
				FTPRootDir:    "/packages",
				FTPSecurity:   "explicit_tls",
				FTPSkipVerify: true,
			},
			renameProfile: func() ArtifactStorageProfile {
				return ArtifactStorageProfile{
					ID:               "ftp-demo-renamed",
					OriginalID:       "ftp-demo",
					Name:             "FTP Demo Renamed",
					Type:             "ftp",
					FTPAddress:       "ftp.example.com:2121",
					FTPUsername:      "demo-user-2",
					FTPPassword:      "",
					FTPRootDir:       "/packages/v2",
					FTPSecurity:      "implicit_tls",
					HasFTPPassword:   true,
					ClearFTPPassword: false,
				}
			},
			findMasked: func(t *testing.T, profile ArtifactStorageProfile) {
				t.Helper()
				if profile.FTPPassword != "" {
					t.Fatalf("expected FTP password to be masked, got %q", profile.FTPPassword)
				}
				if !profile.HasFTPPassword {
					t.Fatalf("expected HasFTPPassword to be true")
				}
			},
			assertStored: func(t *testing.T, profile ArtifactStorageProfile, want string) {
				t.Helper()
				if profile.FTPPassword != want {
					t.Fatalf("expected FTP password %q, got %q", want, profile.FTPPassword)
				}
			},
			clearProfile: func(id string) ArtifactStorageProfile {
				return ArtifactStorageProfile{
					ID:               id,
					OriginalID:       id,
					Name:             "FTP Demo Renamed",
					Type:             "ftp",
					FTPAddress:       "ftp.example.com:2121",
					FTPUsername:      "demo-user-2",
					FTPRootDir:       "/packages/v2",
					FTPSecurity:      "implicit_tls",
					ClearFTPPassword: true,
				}
			},
			assertCleared: func(t *testing.T, profile ArtifactStorageProfile) {
				t.Helper()
				if profile.FTPPassword != "" {
					t.Fatalf("expected FTP password to be cleared, got %q", profile.FTPPassword)
				}
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			root := t.TempDir()

			store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
			if err != nil {
				t.Fatalf("NewLocalStorage returned error: %v", err)
			}
			service := NewService(store, ArtifactStorageProfile{
				ID:      CanonicalArtifactStorageProfileID,
				Name:    "Canonical Storage",
				Type:    "local",
				BaseDir: filepath.Join(root, "data"),
				BaseURL: "http://localhost:18080",
			})

			_, err = service.Update(ctx, Document{
				ArtifactStorage: ArtifactStorageSettings{
					DefaultProfileID: tc.initialID,
					Profiles:         []ArtifactStorageProfile{tc.initialProfile},
				},
			})
			if err != nil {
				t.Fatalf("initial Update returned error: %v", err)
			}

			got, err := service.Get(ctx)
			if err != nil {
				t.Fatalf("Get returned error: %v", err)
			}
			tc.findMasked(t, findProfile(t, got, tc.initialID))

			updated, err := service.Update(ctx, Document{
				ArtifactStorage: ArtifactStorageSettings{
					DefaultProfileID: tc.renamedID,
					Profiles:         []ArtifactStorageProfile{tc.renameProfile()},
				},
			})
			if err != nil {
				t.Fatalf("rename Update returned error: %v", err)
			}
			tc.findMasked(t, findProfile(t, updated, tc.renamedID))

			body, err := store.Read(ctx, SettingsPath)
			if err != nil {
				t.Fatalf("Read settings returned error: %v", err)
			}
			var persisted Document
			if err := json.Unmarshal(body, &persisted); err != nil {
				t.Fatalf("Unmarshal persisted settings returned error: %v", err)
			}
			tc.assertStored(t, findProfile(t, persisted, tc.renamedID), secretValue(tc.initialProfile))

			_, err = service.Update(ctx, Document{
				ArtifactStorage: ArtifactStorageSettings{
					DefaultProfileID: tc.renamedID,
					Profiles:         []ArtifactStorageProfile{tc.clearProfile(tc.renamedID)},
				},
			})
			if err != nil {
				t.Fatalf("clear Update returned error: %v", err)
			}

			body, err = store.Read(ctx, SettingsPath)
			if err != nil {
				t.Fatalf("Read settings after clear returned error: %v", err)
			}
			persisted = Document{}
			if err := json.Unmarshal(body, &persisted); err != nil {
				t.Fatalf("Unmarshal persisted settings after clear returned error: %v", err)
			}
			tc.assertCleared(t, findProfile(t, persisted, tc.renamedID))
		})
	}
}

func secretValue(profile ArtifactStorageProfile) string {
	if profile.WebDAVPassword != "" {
		return profile.WebDAVPassword
	}
	return profile.FTPPassword
}

func findProfile(t *testing.T, doc Document, id string) ArtifactStorageProfile {
	t.Helper()

	for _, profile := range doc.ArtifactStorage.Profiles {
		if profile.ID == id {
			return profile
		}
	}
	t.Fatalf("profile %q not found in %#v", id, doc.ArtifactStorage.Profiles)
	return ArtifactStorageProfile{}
}
