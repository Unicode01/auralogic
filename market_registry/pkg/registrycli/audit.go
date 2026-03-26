package registrycli

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"auralogic/market_registry/pkg/runtimeconfig"
)

type auditMatch struct {
	Token string   `json:"token"`
	Count int      `json:"count"`
	Files []string `json:"files,omitempty"`
}

type auditReport struct {
	RootDir                 string          `json:"root_dir"`
	ImplementationModule    string          `json:"implementation_module"`
	ShellModule             string          `json:"shell_module"`
	ShellDirPresent         bool            `json:"shell_dir_present"`
	ShellSubmodulePresent   bool            `json:"shell_submodule_present"`
	FilesScanned            int             `json:"files_scanned"`
	LegacyCommandRefs       []auditMatch    `json:"legacy_command_refs"`
	AllowedLegacyRefs       []auditMatch    `json:"allowed_legacy_refs"`
	PendingLegacyRefs       []auditMatch    `json:"pending_legacy_refs"`
	CanonicalCommandRefs    []auditMatch    `json:"canonical_command_refs"`
	CanonicalFilesystemRefs []auditMatch    `json:"canonical_filesystem_refs"`
	ModuleReadiness         moduleReadiness `json:"module_readiness"`
}

type moduleReadiness struct {
	ReadyForModuleRename                  bool     `json:"ready_for_module_rename"`
	EntrypointsUsingParentInternal        int      `json:"entrypoints_using_parent_internal"`
	ParentInternalPackages                []string `json:"parent_internal_packages"`
	BootstrapFilesUsingParentInternal     int      `json:"bootstrap_files_using_parent_internal"`
	BootstrapParentInternalPackages       []string `json:"bootstrap_parent_internal_packages"`
	RuntimeFacadeFilesUsingParentInternal int      `json:"runtime_facade_files_using_parent_internal"`
	RuntimeFacadeParentInternalPackages   []string `json:"runtime_facade_parent_internal_packages"`
	ServiceLayerFilesUsingParentInternal  int      `json:"service_layer_files_using_parent_internal"`
	ServiceLayerParentInternalPackages    []string `json:"service_layer_parent_internal_packages"`
	Blockers                              []string `json:"blockers,omitempty"`
}

func (app App) handleAudit(args []string) int {
	report, err := buildAuditReport()
	if err != nil {
		fmt.Fprintf(app.Stderr, "Error building audit report: %v\n", err)
		return 1
	}
	if hasFlag(args, "--strict") && totalAuditCount(report.PendingLegacyRefs) > 0 {
		fmt.Fprintln(app.Stderr, "Error: pending legacy references remain")
		app.printAuditSection("Pending legacy references", report.PendingLegacyRefs)
		return 1
	}
	if hasFlag(args, "--json") {
		encoder := json.NewEncoder(app.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintf(app.Stderr, "Error encoding audit report: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintf(app.Stdout, "Implementation root: %s\n", report.RootDir)
	fmt.Fprintf(app.Stdout, "Implementation module: %s\n", report.ImplementationModule)
	fmt.Fprintf(app.Stdout, "Shell module: %s\n", report.ShellModule)
	fmt.Fprintf(app.Stdout, "Shell dir present: %t\n", report.ShellDirPresent)
	fmt.Fprintf(app.Stdout, "Shell submodule present: %t\n", report.ShellSubmodulePresent)
	fmt.Fprintf(app.Stdout, "Files scanned: %d\n", report.FilesScanned)
	fmt.Fprintf(app.Stdout, "Ready for true module rename: %t\n", report.ModuleReadiness.ReadyForModuleRename)
	if report.ModuleReadiness.EntrypointsUsingParentInternal > 0 {
		fmt.Fprintf(app.Stdout, "Entrypoints using parent internal packages: %d\n", report.ModuleReadiness.EntrypointsUsingParentInternal)
	}
	if report.ModuleReadiness.BootstrapFilesUsingParentInternal > 0 {
		fmt.Fprintf(app.Stdout, "Bootstrap files using parent internal packages: %d\n", report.ModuleReadiness.BootstrapFilesUsingParentInternal)
	}
	if report.ModuleReadiness.RuntimeFacadeFilesUsingParentInternal > 0 {
		fmt.Fprintf(app.Stdout, "Runtime facade files using parent internal packages: %d\n", report.ModuleReadiness.RuntimeFacadeFilesUsingParentInternal)
	}
	if report.ModuleReadiness.ServiceLayerFilesUsingParentInternal > 0 {
		fmt.Fprintf(app.Stdout, "Service layer files using parent internal packages: %d\n", report.ModuleReadiness.ServiceLayerFilesUsingParentInternal)
	}
	if len(report.ModuleReadiness.Blockers) > 0 {
		fmt.Fprintln(app.Stdout, "Module rename blockers:")
		for _, blocker := range report.ModuleReadiness.Blockers {
			fmt.Fprintf(app.Stdout, "  %s\n", blocker)
		}
	}
	fmt.Fprintln(app.Stdout, "")
	app.printAuditSection("Legacy command references", report.LegacyCommandRefs)
	app.printAuditSection("Allowed legacy references", report.AllowedLegacyRefs)
	app.printAuditSection("Pending legacy references", report.PendingLegacyRefs)
	app.printAuditSection("Canonical command references", report.CanonicalCommandRefs)
	app.printAuditSection("Canonical filesystem references", report.CanonicalFilesystemRefs)
	return 0
}

func (app App) printAuditSection(title string, matches []auditMatch) {
	fmt.Fprintf(app.Stdout, "%s:\n", title)
	for _, match := range matches {
		fmt.Fprintf(app.Stdout, "  %s -> %d\n", match.Token, match.Count)
		for _, file := range match.Files {
			fmt.Fprintf(app.Stdout, "    %s\n", file)
		}
	}
	fmt.Fprintln(app.Stdout)
}

func buildAuditReport() (auditReport, error) {
	rootDir, err := findImplementationRoot()
	if err != nil {
		return auditReport{}, err
	}

	report := auditReport{
		RootDir:               rootDir,
		ImplementationModule:  runtimeconfig.ModulePath,
		ShellModule:           shellModulePath,
		ShellDirPresent:       fileExists(rootDir),
		ShellSubmodulePresent: fileExists(filepath.Join(rootDir, "go.mod")),
	}

	legacyTokens := []string{
		"cmd/source-api",
		"cmd/source-cli",
	}
	canonicalCommandTokens := []string{
		"cmd/market-registry-api",
		"cmd/market-registry-cli",
	}
	canonicalFilesystemTokens := []string{
		"./cmd/market-registry-api",
		"./cmd/market-registry-cli",
	}

	legacyIndex := newAuditIndex(legacyTokens)
	canonicalCommandIndex := newAuditIndex(canonicalCommandTokens)
	canonicalFilesystemIndex := newAuditIndex(canonicalFilesystemTokens)

	err = filepath.WalkDir(rootDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if shouldSkipDir(rootDir, path) {
				return filepath.SkipDir
			}
			return nil
		}
		if shouldSkipFile(path) {
			return nil
		}
		payload, readErr := os.ReadFile(path)
		if readErr != nil || looksBinary(payload) {
			return nil
		}
		report.FilesScanned++
		relative := filepath.ToSlash(strings.TrimPrefix(path, rootDir))
		relative = strings.TrimPrefix(relative, "/")
		content := string(payload)
		recordAuditMatches(legacyIndex, content, relative)
		recordAuditMatches(canonicalCommandIndex, content, relative)
		recordAuditMatches(canonicalFilesystemIndex, content, relative)
		return nil
	})
	if err != nil {
		return auditReport{}, err
	}

	report.LegacyCommandRefs = flattenAuditIndex(legacyIndex)
	report.AllowedLegacyRefs, report.PendingLegacyRefs = splitLegacyRefs(report.LegacyCommandRefs)
	report.CanonicalCommandRefs = flattenAuditIndex(canonicalCommandIndex)
	report.CanonicalFilesystemRefs = flattenAuditIndex(canonicalFilesystemIndex)
	report.ModuleReadiness, err = buildModuleReadiness(rootDir)
	if err != nil {
		return auditReport{}, err
	}
	return report, nil
}

func findImplementationRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	current := cwd
	for {
		goModPath := filepath.Join(current, "go.mod")
		payload, readErr := os.ReadFile(goModPath)
		if readErr == nil && hasModuleDeclaration(string(payload), runtimeconfig.ModulePath) {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", errors.New("implementation root not found")
}

func shouldSkipDir(root string, path string) bool {
	relative := filepath.ToSlash(strings.TrimPrefix(path, root))
	relative = strings.TrimPrefix(relative, "/")
	if relative == "" {
		return false
	}
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".gocache") {
		return true
	}
	switch relative {
	case "admin/node_modules", "admin/build":
		return true
	}
	switch base {
	case ".git", ".next", "build", "dist", "coverage", "node_modules":
		return true
	}
	return false
}

func shouldSkipFile(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	return strings.HasSuffix(base, ".exe")
}

func looksBinary(payload []byte) bool {
	for _, b := range payload {
		if b == 0 {
			return true
		}
	}
	return false
}

func newAuditIndex(tokens []string) map[string]*auditMatch {
	out := make(map[string]*auditMatch, len(tokens))
	for _, token := range tokens {
		out[token] = &auditMatch{Token: token, Files: []string{}}
	}
	return out
}

func recordAuditMatches(index map[string]*auditMatch, content string, relative string) {
	for token, item := range index {
		count := strings.Count(content, token)
		if count == 0 {
			continue
		}
		item.Count += count
		item.Files = append(item.Files, relative)
	}
}

func flattenAuditIndex(index map[string]*auditMatch) []auditMatch {
	out := make([]auditMatch, 0, len(index))
	for _, item := range index {
		sort.Strings(item.Files)
		out = append(out, *item)
	}
	sort.Slice(out, func(i int, j int) bool {
		return out[i].Token < out[j].Token
	})
	return out
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func splitLegacyRefs(matches []auditMatch) ([]auditMatch, []auditMatch) {
	allowed := make([]auditMatch, 0, len(matches))
	pending := make([]auditMatch, 0, len(matches))
	for _, match := range matches {
		allowedFiles := make([]string, 0, len(match.Files))
		pendingFiles := make([]string, 0, len(match.Files))
		for _, file := range match.Files {
			if isAllowedLegacyReferenceFile(file) {
				allowedFiles = append(allowedFiles, file)
			} else {
				pendingFiles = append(pendingFiles, file)
			}
		}
		allowed = append(allowed, auditMatch{
			Token: match.Token,
			Count: len(allowedFiles),
			Files: allowedFiles,
		})
		pending = append(pending, auditMatch{
			Token: match.Token,
			Count: len(pendingFiles),
			Files: pendingFiles,
		})
	}
	return allowed, pending
}

func isAllowedLegacyReferenceFile(path string) bool {
	switch path {
	case "pkg/registrycli/audit.go":
		return true
	default:
		return false
	}
}

func totalAuditCount(matches []auditMatch) int {
	total := 0
	for _, match := range matches {
		total += match.Count
	}
	return total
}

func buildModuleReadiness(rootDir string) (moduleReadiness, error) {
	entrypointFiles := []string{
		filepath.Join(rootDir, "cmd", "market-registry-api", "main.go"),
		filepath.Join(rootDir, "cmd", "market-registry-cli", "main.go"),
	}
	bootstrapFiles, err := listGoFiles(
		filepath.Join(rootDir, "pkg", "registryapi"),
		filepath.Join(rootDir, "pkg", "registrycli"),
		filepath.Join(rootDir, "pkg", "runtimeconfig"),
	)
	if err != nil {
		return moduleReadiness{}, err
	}
	runtimeFacadeFiles, err := listGoFiles(
		filepath.Join(rootDir, "pkg", "registryruntime"),
	)
	if err != nil {
		return moduleReadiness{}, err
	}
	serviceLayerFiles, err := listGoFiles(
		filepath.Join(rootDir, "pkg", "storage"),
		filepath.Join(rootDir, "pkg", "signing"),
		filepath.Join(rootDir, "pkg", "publish"),
		filepath.Join(rootDir, "pkg", "auth"),
		filepath.Join(rootDir, "pkg", "analytics"),
		filepath.Join(rootDir, "pkg", "catalog"),
		filepath.Join(rootDir, "pkg", "sourceapi"),
		filepath.Join(rootDir, "pkg", "adminapi"),
	)
	if err != nil {
		return moduleReadiness{}, err
	}

	entrypointPackages, entrypointsUsingParentInternal, err := collectInternalImports(entrypointFiles)
	if err != nil {
		return moduleReadiness{}, err
	}
	bootstrapPackages, bootstrapFilesUsingParentInternal, err := collectInternalImports(bootstrapFiles)
	if err != nil {
		return moduleReadiness{}, err
	}
	runtimeFacadePackages, runtimeFacadeFilesUsingParentInternal, err := collectInternalImports(runtimeFacadeFiles)
	if err != nil {
		return moduleReadiness{}, err
	}
	serviceLayerPackages, serviceLayerFilesUsingParentInternal, err := collectInternalImports(serviceLayerFiles)
	if err != nil {
		return moduleReadiness{}, err
	}

	readiness := moduleReadiness{
		ReadyForModuleRename:                  len(entrypointPackages) == 0 && len(bootstrapPackages) == 0 && len(runtimeFacadePackages) == 0 && len(serviceLayerPackages) == 0,
		EntrypointsUsingParentInternal:        entrypointsUsingParentInternal,
		ParentInternalPackages:                entrypointPackages,
		BootstrapFilesUsingParentInternal:     bootstrapFilesUsingParentInternal,
		BootstrapParentInternalPackages:       bootstrapPackages,
		RuntimeFacadeFilesUsingParentInternal: runtimeFacadeFilesUsingParentInternal,
		RuntimeFacadeParentInternalPackages:   runtimeFacadePackages,
		ServiceLayerFilesUsingParentInternal:  serviceLayerFilesUsingParentInternal,
		ServiceLayerParentInternalPackages:    serviceLayerPackages,
		Blockers:                              []string{},
	}
	if len(entrypointPackages) > 0 {
		readiness.Blockers = append(readiness.Blockers, "entrypoint wrappers still import parent module internal packages")
	}
	if len(bootstrapPackages) > 0 {
		readiness.Blockers = append(readiness.Blockers,
			"public bootstrap packages still depend on parent module internal services",
		)
	}
	if len(runtimeFacadePackages) > 0 {
		readiness.Blockers = append(readiness.Blockers,
			"public runtime facade packages still depend on parent module internal services",
		)
	}
	if len(serviceLayerPackages) > 0 {
		readiness.Blockers = append(readiness.Blockers,
			"public service wrapper packages still depend on parent module internal services",
			"true module rename still requires promoting internal service implementations behind stable public service packages",
		)
	}
	return readiness, nil
}

func listGoFiles(roots ...string) ([]string, error) {
	out := make([]string, 0)
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			continue
		}
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			out = append(out, path)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

func collectInternalImports(files []string) ([]string, int, error) {
	packageSet := map[string]struct{}{}
	filesUsingParentInternal := 0
	for _, file := range files {
		payload, err := os.ReadFile(file)
		if err != nil {
			return nil, 0, err
		}
		matches := extractInternalImports(file, payload)
		if len(matches) == 0 {
			continue
		}
		filesUsingParentInternal++
		for _, match := range matches {
			packageSet[match] = struct{}{}
		}
	}

	packages := make([]string, 0, len(packageSet))
	for pkg := range packageSet {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	return packages, filesUsingParentInternal, nil
}

func extractInternalImports(filename string, payload []byte) []string {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, filename, payload, parser.ImportsOnly)
	if err != nil {
		return []string{}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for _, spec := range parsed.Imports {
		if spec == nil || spec.Path == nil {
			continue
		}
		value := strings.Trim(spec.Path.Value, "\"")
		if !strings.HasPrefix(value, "auralogic/market_registry/internal/") {
			continue
		}
		if isNonBlockingInternalImport(value) {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func isNonBlockingInternalImport(value string) bool {
	switch value {
	case "auralogic/market_registry/internal/adminui":
		return true
	default:
		return false
	}
}

func hasModuleDeclaration(goMod string, modulePath string) bool {
	for _, line := range strings.Split(goMod, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if !strings.HasPrefix(line, "module ") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, "module ")) == modulePath
	}
	return false
}
