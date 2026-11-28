// Command mailpreview renders every transactional e-mail in every locale with
// sample data into a directory (<kind>.<locale>.html and .txt) plus an
// index.html gallery, for reviewing the templates in a browser.
//
//	go run ./cmd/mailpreview -out tmp/mail-preview
//	make mail-preview
package main

import (
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/furkanmeclis/messora-app-go/internal/platform/mailer/templates"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mailpreview:", err)
		os.Exit(1)
	}
}

type entry struct {
	Kind, Locale, Subject, Preheader, HTML, Text string
	Size                                         int
}

func run() error {
	out := flag.String("out", "tmp/mail-preview", "output directory")
	assets := flag.String("asset-base-url", templates.DefaultAssetBaseURL, "https base URL serving email/logo-light@2x.png and email/logo-dark@2x.png")
	flag.Parse()

	brand := templates.DefaultBrand()
	brand.AssetBaseURL = *assets
	r, err := templates.New(brand)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*out, 0o750); err != nil {
		return err
	}
	var entries []entry
	for _, k := range templates.Kinds() {
		for _, loc := range templates.Locales {
			e, err := r.Email(k, loc, templates.Sample(k, loc))
			if err != nil {
				return err
			}
			base := string(k) + "." + loc
			if err := os.WriteFile(filepath.Join(*out, base+".html"), []byte(e.HTML), 0o600); err != nil {
				return err
			}
			txt := "Subject: " + e.Subject + "\nPreheader: " + e.Preheader + "\n\n" + e.Text
			if err := os.WriteFile(filepath.Join(*out, base+".txt"), []byte(txt), 0o600); err != nil {
				return err
			}
			entries = append(entries, entry{string(k), loc, e.Subject, e.Preheader, base + ".html", base + ".txt", len(e.HTML)})
		}
	}
	f, err := os.Create(filepath.Join(*out, "index.html"))
	if err != nil {
		return err
	}
	if err := gallery.Execute(f, entries); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	abs, _ := filepath.Abs(filepath.Join(*out, "index.html"))
	fmt.Printf("%d e-mails written; open %s\n", len(entries), abs)
	return nil
}

var gallery = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Messora e-mail preview</title>
<style>
  :root { color-scheme: light dark; --bg:#F4F6FB; --card:#fff; --text:#0B1220; --muted:#4A5468; --line:#E3E7F0; --accent:#3B4CCA; }
  @media (prefers-color-scheme: dark) { :root { --bg:#0B1220; --card:#141C2E; --text:#E8ECF5; --muted:#A9B2C7; --line:#26304A; --accent:#A5B0FF; } }
  * { box-sizing: border-box; }
  body { margin:0; padding:24px 16px; background:var(--bg); color:var(--text); font:15px/1.5 Inter, -apple-system, "Segoe UI", Roboto, Arial, sans-serif; }
  h1 { font:800 24px/1.2 "Plus Jakarta Sans", Inter, sans-serif; letter-spacing:-0.03em; margin:0 0 4px; }
  p.lead { color:var(--muted); margin:0 0 24px; }
  .grid { display:grid; grid-template-columns:repeat(auto-fill, minmax(min(100%, 420px), 1fr)); gap:20px; }
  .card { background:var(--card); border:1px solid var(--line); border-radius:14px; overflow:hidden; }
  .meta { padding:12px 16px; border-bottom:1px solid var(--line); }
  .meta b { display:block; }
  .meta small { color:var(--muted); display:block; }
  .meta a { color:var(--accent); margin-right:12px; font-size:13px; }
  .tag { display:inline-block; font-size:11px; font-weight:700; text-transform:uppercase; letter-spacing:.05em; background:#2DD4A7; color:#0B1220; border-radius:6px; padding:1px 6px; margin-right:6px; }
  iframe { display:block; width:100%; height:560px; border:0; background:#F4F6FB; }
</style>
</head>
<body>
<h1>Messora e-mail preview</h1>
<p class="lead">{{len .}} renders (every kind in Turkish and English) with sample data. Each card links the HTML and plain-text parts.</p>
<div class="grid">
{{range .}}<section class="card">
  <div class="meta">
    <b><span class="tag">{{.Locale}}</span>{{.Kind}}</b>
    <small>{{.Subject}}</small>
    <small>{{.Preheader}}</small>
    <a href="{{.HTML}}" target="_blank">HTML ({{.Size}} bytes)</a><a href="{{.Text}}" target="_blank">Text</a>
  </div>
  <iframe src="{{.HTML}}" title="{{.Kind}} {{.Locale}}" loading="lazy"></iframe>
</section>
{{end}}</div>
</body>
</html>
`))
