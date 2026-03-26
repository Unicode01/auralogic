package storage

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/textproto"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
)

const (
	ftpSecurityPlain       = "plain"
	ftpSecurityExplicitTLS = "explicit_tls"
	ftpSecurityImplicitTLS = "implicit_tls"
)

type FTPStorage struct {
	address     string
	username    string
	password    string
	rootDir     string
	security    string
	skipVerify  bool
	baseURL     string
	dialTimeout time.Duration
}

func NewFTPStorage(cfg Config) (*FTPStorage, error) {
	address := strings.TrimSpace(cfg.FTPAddress)
	if address == "" {
		return nil, fmt.Errorf("ftp address is required")
	}
	security := normalizeFTPSecurity(cfg.FTPSecurity)
	if security == "" {
		return nil, fmt.Errorf("ftp security mode %q is invalid", strings.TrimSpace(cfg.FTPSecurity))
	}
	rootDir, err := normalizeStoragePath(cfg.FTPRootDir, true)
	if err != nil {
		return nil, fmt.Errorf("ftp root dir is invalid: %w", err)
	}
	return &FTPStorage{
		address:     address,
		username:    strings.TrimSpace(cfg.FTPUsername),
		password:    cfg.FTPPassword,
		rootDir:     rootDir,
		security:    security,
		skipVerify:  cfg.FTPSkipVerify,
		baseURL:     strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		dialTimeout: 30 * time.Second,
	}, nil
}

func (s *FTPStorage) Read(ctx context.Context, itemPath string) ([]byte, error) {
	resolved, err := s.resolvePath(itemPath, false)
	if err != nil {
		return nil, err
	}
	conn, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer s.close(conn)

	reader, err := conn.Retr(resolved)
	if err != nil {
		return nil, translateFTPError(err)
	}
	defer reader.Close()

	payload, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func (s *FTPStorage) Write(ctx context.Context, itemPath string, data []byte) error {
	resolved, err := s.resolvePath(itemPath, false)
	if err != nil {
		return err
	}
	conn, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer s.close(conn)

	if err := s.ensureDir(conn, path.Dir(resolved)); err != nil {
		return err
	}
	if err := conn.Stor(resolved, bytes.NewReader(data)); err != nil {
		return translateFTPError(err)
	}
	return nil
}

func (s *FTPStorage) Exists(ctx context.Context, itemPath string) (bool, error) {
	resolved, err := s.resolvePath(itemPath, false)
	if err != nil {
		return false, err
	}
	conn, err := s.connect(ctx)
	if err != nil {
		return false, err
	}
	defer s.close(conn)

	exists, _, _, err := s.stat(conn, resolved)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s *FTPStorage) List(ctx context.Context, prefix string) ([]string, error) {
	normalized, err := normalizeStoragePath(prefix, true)
	if err != nil {
		return nil, err
	}
	resolved, err := s.resolvePath(prefix, true)
	if err != nil {
		return nil, err
	}
	conn, err := s.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer s.close(conn)

	if normalized != "" {
		exists, isDir, _, statErr := s.stat(conn, resolved)
		if statErr != nil {
			return nil, statErr
		}
		if !exists {
			return []string{}, nil
		}
		if !isDir {
			return []string{normalized}, nil
		}
	}

	walker := conn.Walk(resolved)
	files := make([]string, 0, 8)
	for walker.Next() {
		entry := walker.Stat()
		if entry == nil || entry.Type == ftp.EntryTypeFolder {
			continue
		}
		relative := s.relativePath(walker.Path())
		if relative == "" {
			continue
		}
		files = append(files, relative)
	}
	if err := walker.Err(); err != nil {
		if isFTPNotFound(err) {
			return []string{}, nil
		}
		return nil, translateFTPError(err)
	}
	sort.Strings(files)
	return files, nil
}

func (s *FTPStorage) Delete(ctx context.Context, itemPath string) error {
	resolved, err := s.resolvePath(itemPath, false)
	if err != nil {
		return err
	}
	conn, err := s.connect(ctx)
	if err != nil {
		return err
	}
	defer s.close(conn)

	if err := conn.Delete(resolved); err == nil {
		return nil
	} else if !isFTPNotFound(err) {
		exists, isDir, _, statErr := s.stat(conn, resolved)
		if statErr != nil {
			return statErr
		}
		if !exists {
			return translateFTPError(err)
		}
		if isDir {
			if removeErr := conn.RemoveDirRecur(resolved); removeErr != nil {
				return translateFTPError(removeErr)
			}
			return nil
		}
		return translateFTPError(err)
	}
	return translateFTPError(err)
}

func (s *FTPStorage) PublicURL(itemPath string) string {
	if s.baseURL != "" {
		trimmed := strings.TrimLeft(strings.TrimSpace(itemPath), "/")
		if trimmed == "" {
			return s.baseURL
		}
		return s.baseURL + "/" + trimmed
	}
	root := strings.Trim(strings.TrimSpace(s.rootDir), "/")
	trimmed := strings.TrimLeft(strings.TrimSpace(itemPath), "/")
	parts := []string{"ftp://" + strings.TrimSpace(s.address)}
	if root != "" {
		parts = append(parts, root)
	}
	if trimmed != "" {
		parts = append(parts, trimmed)
	}
	return strings.Join(parts, "/")
}

func (s *FTPStorage) Usage(ctx context.Context) (UsageSummary, error) {
	conn, err := s.connect(ctx)
	if err != nil {
		return UsageSummary{}, err
	}
	defer s.close(conn)

	root := "."
	if s.rootDir != "" {
		root = s.rootDir
	}
	summary := UsageSummary{
		Backend:     "ftp",
		DisplayName: "FTP Storage",
		Location:    s.PublicURL(""),
	}
	walker := conn.Walk(root)
	for walker.Next() {
		entry := walker.Stat()
		if entry == nil || entry.Type == ftp.EntryTypeFolder {
			continue
		}
		summary.FileCount++
		summary.TotalBytes += int64(entry.Size)
	}
	if err := walker.Err(); err != nil {
		if isFTPNotFound(err) {
			return summary, nil
		}
		return UsageSummary{}, translateFTPError(err)
	}
	return summary, nil
}

func (s *FTPStorage) connect(ctx context.Context) (*ftp.ServerConn, error) {
	options := []ftp.DialOption{
		ftp.DialWithContext(ctx),
		ftp.DialWithTimeout(s.dialTimeout),
	}
	if s.security == ftpSecurityExplicitTLS || s.security == ftpSecurityImplicitTLS {
		tlsConfig := &tls.Config{InsecureSkipVerify: s.skipVerify}
		if s.security == ftpSecurityExplicitTLS {
			options = append(options, ftp.DialWithExplicitTLS(tlsConfig))
		} else {
			options = append(options, ftp.DialWithTLS(tlsConfig))
		}
	}

	conn, err := ftp.Dial(s.address, options...)
	if err != nil {
		return nil, err
	}
	username := s.username
	password := s.password
	if username == "" {
		username = "anonymous"
		if password == "" {
			password = "anonymous@"
		}
	}
	if err := conn.Login(username, password); err != nil {
		_ = conn.Quit()
		return nil, err
	}
	_ = conn.Type(ftp.TransferTypeBinary)
	return conn, nil
}

func (s *FTPStorage) close(conn *ftp.ServerConn) {
	if conn != nil {
		_ = conn.Quit()
	}
}

func (s *FTPStorage) ensureDir(conn *ftp.ServerConn, dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" || dir == "." {
		return nil
	}
	parts := strings.Split(dir, "/")
	current := ""
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." {
			continue
		}
		if current == "" {
			current = part
		} else {
			current = current + "/" + part
		}
		if _, err := conn.List(current); err == nil {
			continue
		}
		if err := conn.MakeDir(current); err != nil {
			if _, listErr := conn.List(current); listErr == nil {
				continue
			}
			return translateFTPError(err)
		}
	}
	return nil
}

func (s *FTPStorage) stat(conn *ftp.ServerConn, target string) (bool, bool, int64, error) {
	target = strings.TrimSpace(target)
	if target == "" || target == "." {
		return true, true, 0, nil
	}
	size, err := conn.FileSize(target)
	if err == nil {
		return true, false, size, nil
	}
	if !isFTPNotFound(err) {
		entries, listErr := conn.List(target)
		if listErr != nil {
			return false, false, 0, translateFTPError(err)
		}
		if len(entries) == 0 {
			return true, true, 0, nil
		}
		base := path.Base(target)
		for _, entry := range entries {
			if entry == nil {
				continue
			}
			if entry.Name == base && entry.Type != ftp.EntryTypeFolder {
				return true, false, int64(entry.Size), nil
			}
		}
		return true, true, 0, nil
	}
	entries, listErr := conn.List(target)
	if listErr == nil {
		if len(entries) == 1 {
			base := path.Base(target)
			entry := entries[0]
			if entry != nil && entry.Name == base && entry.Type != ftp.EntryTypeFolder {
				return true, false, int64(entry.Size), nil
			}
		}
		return true, true, 0, nil
	}
	if isFTPNotFound(listErr) {
		return false, false, 0, nil
	}
	return false, false, 0, translateFTPError(listErr)
}

func (s *FTPStorage) resolvePath(value string, allowEmpty bool) (string, error) {
	normalized, err := normalizeStoragePath(value, allowEmpty)
	if err != nil {
		return "", err
	}
	switch {
	case normalized == "" && s.rootDir == "":
		if allowEmpty {
			return ".", nil
		}
		return "", fmt.Errorf("path is required")
	case normalized == "":
		return s.rootDir, nil
	case s.rootDir == "":
		return normalized, nil
	default:
		return path.Join(s.rootDir, normalized), nil
	}
}

func (s *FTPStorage) relativePath(value string) string {
	normalized := path.Clean(strings.TrimSpace(strings.ReplaceAll(value, "\\", "/")))
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimPrefix(normalized, "/")
	root := strings.Trim(strings.TrimSpace(s.rootDir), "/")
	if root != "" {
		rootPrefix := root + "/"
		if normalized == root {
			return ""
		}
		if strings.HasPrefix(normalized, rootPrefix) {
			normalized = strings.TrimPrefix(normalized, rootPrefix)
		}
	}
	if normalized == "" || normalized == "." {
		return ""
	}
	return normalized
}

func normalizeFTPSecurity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", ftpSecurityPlain:
		return ftpSecurityPlain
	case ftpSecurityExplicitTLS:
		return ftpSecurityExplicitTLS
	case ftpSecurityImplicitTLS:
		return ftpSecurityImplicitTLS
	default:
		return ""
	}
}

func isFTPNotFound(err error) bool {
	var protocolErr *textproto.Error
	if !errorAsTextproto(err, &protocolErr) {
		return false
	}
	return protocolErr.Code == ftp.StatusFileUnavailable
}

func translateFTPError(err error) error {
	if isFTPNotFound(err) {
		return fmt.Errorf("ftp object not found: %w", fs.ErrNotExist)
	}
	return err
}

func errorAsTextproto(err error, target **textproto.Error) bool {
	if err == nil || target == nil {
		return false
	}
	var protocolErr *textproto.Error
	if !errors.As(err, &protocolErr) {
		return false
	}
	*target = protocolErr
	return true
}
