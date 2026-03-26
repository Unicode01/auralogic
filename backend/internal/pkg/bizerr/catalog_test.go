package bizerr

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

var bizErrorKeyPattern = regexp.MustCompile(`['"]([^'"\r\n]+)['"]\s*:`)

func TestFrontendBizErrorCatalog(t *testing.T) {
	repoRoot := mustRepoRoot(t)

	backendKeys := mustCollectBackendBizErrorKeys(t, filepath.Join(repoRoot, "backend"))
	zhKeys := mustCollectFrontendBizErrorKeys(t, filepath.Join(repoRoot, "frontend", "lib", "i18n", "zh.ts"))
	enKeys := mustCollectFrontendBizErrorKeys(t, filepath.Join(repoRoot, "frontend", "lib", "i18n", "en.ts"))

	t.Run("frontend locales stay in sync", func(t *testing.T) {
		missingInZh := diffSorted(enKeys, zhKeys)
		missingInEn := diffSorted(zhKeys, enKeys)

		if len(missingInZh) > 0 || len(missingInEn) > 0 {
			t.Fatalf("bizError locale mismatch\nmissing in zh: %v\nmissing in en: %v", missingInZh, missingInEn)
		}
	})

	t.Run("frontend translations cover backend static keys", func(t *testing.T) {
		missingInZh := diffSorted(backendKeys, zhKeys)
		missingInEn := diffSorted(backendKeys, enKeys)

		if len(missingInZh) > 0 || len(missingInEn) > 0 {
			t.Fatalf("missing frontend bizError translations for backend keys\nmissing in zh: %v\nmissing in en: %v", missingInZh, missingInEn)
		}
	})
}

func mustRepoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve current test file path")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}

func mustCollectBackendBizErrorKeys(t *testing.T, backendRoot string) map[string]struct{} {
	t.Helper()

	keys := make(map[string]struct{})
	fset := token.NewFileSet()

	err := filepath.WalkDir(backendRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "tmp", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}

		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			ident, ok := selector.X.(*ast.Ident)
			if !ok || ident.Name != "bizerr" {
				return true
			}

			if selector.Sel == nil || (selector.Sel.Name != "New" && selector.Sel.Name != "Newf") {
				return true
			}
			if len(call.Args) == 0 {
				return true
			}

			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}

			key, err := strconv.Unquote(literal.Value)
			if err != nil || strings.TrimSpace(key) == "" {
				return true
			}

			keys[key] = struct{}{}
			return true
		})

		return nil
	})
	if err != nil {
		t.Fatalf("collect backend bizerr keys: %v", err)
	}

	return keys
}

func mustCollectFrontendBizErrorKeys(t *testing.T, filePath string) map[string]struct{} {
	t.Helper()

	contentBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read frontend translation file %s: %v", filePath, err)
	}
	content := string(contentBytes)

	keys := make(map[string]struct{})
	searchFrom := 0
	for {
		relativeIndex := strings.Index(content[searchFrom:], "bizError")
		if relativeIndex < 0 {
			break
		}

		nameIndex := searchFrom + relativeIndex
		cursor := nameIndex + len("bizError")
		cursor = skipWhitespace(content, cursor)
		if cursor >= len(content) || content[cursor] != ':' {
			searchFrom = cursor
			continue
		}

		cursor++
		cursor = skipWhitespace(content, cursor)
		if cursor >= len(content) || content[cursor] != '{' {
			searchFrom = cursor
			continue
		}

		blockEnd, ok := findMatchingBrace(content, cursor)
		if !ok {
			t.Fatalf("failed to parse bizError block in %s", filePath)
		}

		block := content[cursor+1 : blockEnd]
		for _, match := range bizErrorKeyPattern.FindAllStringSubmatch(block, -1) {
			if len(match) < 2 {
				continue
			}
			key := strings.TrimSpace(match[1])
			if key != "" {
				keys[key] = struct{}{}
			}
		}

		searchFrom = blockEnd + 1
	}

	if len(keys) == 0 {
		t.Fatalf("no bizError keys found in %s", filePath)
	}

	return keys
}

func skipWhitespace(content string, index int) int {
	for index < len(content) {
		switch content[index] {
		case ' ', '\t', '\r', '\n':
			index++
		default:
			return index
		}
	}
	return index
}

func findMatchingBrace(content string, openIndex int) (int, bool) {
	depth := 0
	inSingleQuote := false
	inDoubleQuote := false
	inTemplate := false
	inLineComment := false
	inBlockComment := false
	escaped := false

	for i := openIndex; i < len(content); i++ {
		ch := content[i]

		if inLineComment {
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && i+1 < len(content) && content[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}

		if inSingleQuote || inDoubleQuote || inTemplate {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if inSingleQuote && ch == '\'' {
				inSingleQuote = false
				continue
			}
			if inDoubleQuote && ch == '"' {
				inDoubleQuote = false
				continue
			}
			if inTemplate && ch == '`' {
				inTemplate = false
			}
			continue
		}

		if ch == '/' && i+1 < len(content) {
			switch content[i+1] {
			case '/':
				inLineComment = true
				i++
				continue
			case '*':
				inBlockComment = true
				i++
				continue
			}
		}

		switch ch {
		case '\'':
			inSingleQuote = true
		case '"':
			inDoubleQuote = true
		case '`':
			inTemplate = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}

	return 0, false
}

func diffSorted(expected map[string]struct{}, actual map[string]struct{}) []string {
	missing := make([]string, 0)
	for key := range expected {
		if _, ok := actual[key]; ok {
			continue
		}
		missing = append(missing, key)
	}
	slices.Sort(missing)
	return missing
}
