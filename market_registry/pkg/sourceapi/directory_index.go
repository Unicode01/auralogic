package sourceapi

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

type directoryIndexPage struct {
	Title       string
	Path        string
	Description string
	ParentHref  string
	Entries     []directoryIndexEntry
}

type directoryIndexEntry struct {
	Name        string
	Href        string
	Type        string
	Description string
}

var directoryIndexTemplate = template.Must(template.New("directory-index").Parse(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width,initial-scale=1" />
    <title>{{.Title}}</title>
    <style>
      :root {
        color-scheme: light;
        --bg: #f4f7fb;
        --panel: #ffffff;
        --text: #0f172a;
        --muted: #64748b;
        --line: #dbe3ef;
        --accent: #0f766e;
        --accent-soft: rgba(15, 118, 110, 0.08);
      }

      * {
        box-sizing: border-box;
      }

      body {
        margin: 0;
        font-family: "Segoe UI", "PingFang SC", sans-serif;
        background: linear-gradient(180deg, #f8fbff 0%, var(--bg) 100%);
        color: var(--text);
      }

      main {
        max-width: 1100px;
        margin: 40px auto;
        padding: 0 24px;
      }

      .panel {
        background: var(--panel);
        border: 1px solid var(--line);
        border-radius: 20px;
        overflow: hidden;
        box-shadow: 0 20px 50px rgba(15, 23, 42, 0.08);
      }

      .header {
        padding: 28px 32px 20px;
        border-bottom: 1px solid var(--line);
        background: linear-gradient(135deg, #ffffff 0%, #f2fbfa 100%);
      }

      h1 {
        margin: 0;
        font-size: 28px;
        line-height: 1.2;
      }

      .path {
        margin-top: 8px;
        color: var(--muted);
        font-family: Consolas, "SFMono-Regular", monospace;
        font-size: 14px;
      }

      .description {
        margin-top: 12px;
        color: var(--muted);
        line-height: 1.6;
      }

      table {
        width: 100%;
        border-collapse: collapse;
      }

      th,
      td {
        padding: 14px 32px;
        border-bottom: 1px solid var(--line);
        text-align: left;
        vertical-align: top;
      }

      th {
        font-size: 13px;
        text-transform: uppercase;
        letter-spacing: 0.08em;
        color: var(--muted);
        background: #f8fafc;
      }

      tr:last-child td {
        border-bottom: none;
      }

      a {
        color: var(--accent);
        text-decoration: none;
      }

      a:hover {
        text-decoration: underline;
      }

      .entry-name {
        display: inline-flex;
        align-items: center;
        gap: 10px;
        font-weight: 600;
      }

      .entry-name::before {
        content: attr(data-icon);
        display: inline-flex;
        width: 24px;
        height: 24px;
        align-items: center;
        justify-content: center;
        border-radius: 999px;
        background: var(--accent-soft);
        font-size: 14px;
      }

      .empty {
        padding: 32px;
        color: var(--muted);
      }
    </style>
  </head>
  <body>
    <main>
      <section class="panel">
        <header class="header">
          <h1>{{.Title}}</h1>
          <div class="path">{{.Path}}</div>
          {{if .Description}}<div class="description">{{.Description}}</div>{{end}}
        </header>
        {{if .Entries}}
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Type</th>
              <th>Description</th>
            </tr>
          </thead>
          <tbody>
            {{if .ParentHref}}
            <tr>
              <td><a class="entry-name" data-icon="↩" href="{{.ParentHref}}">..</a></td>
              <td>parent</td>
              <td>Back to the previous directory</td>
            </tr>
            {{end}}
            {{range .Entries}}
            <tr>
              <td><a class="entry-name" data-icon="{{if eq .Type "directory"}}📁{{else}}📄{{end}}" href="{{.Href}}">{{.Name}}</a></td>
              <td>{{.Type}}</td>
              <td>{{.Description}}</td>
            </tr>
            {{end}}
          </tbody>
        </table>
        {{else}}
        <div class="empty">This directory is empty.</div>
        {{end}}
      </section>
    </main>
  </body>
</html>
`))

func writeDirectoryIndex(w http.ResponseWriter, status int, page directoryIndexPage) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = directoryIndexTemplate.Execute(w, page)
}

func ensureTrailingSlash(value string) string {
	if strings.HasSuffix(value, "/") {
		return value
	}
	return value + "/"
}

func formatBytes(value any) string {
	switch typed := value.(type) {
	case int:
		return formatByteCount(int64(typed))
	case int64:
		return formatByteCount(typed)
	case float64:
		return formatByteCount(int64(typed))
	default:
		return ""
	}
}

func formatByteCount(size int64) string {
	if size <= 0 {
		return ""
	}
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
}

func formatDigest(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 16 {
		return trimmed
	}
	return trimmed[:16] + "..."
}
