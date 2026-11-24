package licensetool

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v4"
)

// InstallCompose turns the repository's docker-compose.yml into the single
// file customers install from: no build sections, the Caddyfile inlined as
// a compose config, and images defaulting to the licence registry at a
// pinned version. Everything else (limits, health checks, networks,
// overrides through .env) is unchanged.
func InstallCompose(compose, caddyfile []byte, registry, version string) ([]byte, error) {
	if registry == "" || version == "" {
		return nil, errors.New("registry and version are required")
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(compose, &doc); err != nil {
		return nil, fmt.Errorf("compose: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, errors.New("compose: empty document")
	}
	root := doc.Content[0]
	doc.HeadComment = ""
	root.HeadComment = installHeader(registry)

	app := mapGet(root, "x-messora-app")
	services := mapGet(root, "services")
	if app == nil || services == nil {
		return nil, errors.New("compose: x-messora-app or services missing")
	}
	backend := fmt.Sprintf("${MESSORA_IMAGE:-%s/messora}:${MESSORA_VERSION:-%s}", registry, version)
	web := fmt.Sprintf("${MESSORA_WEB_IMAGE:-%s/messora-web}:${MESSORA_VERSION:-%s}", registry, version)
	if !mapSet(app, "image", backend) || !mapSet(app, "pull_policy", "${MESSORA_PULL:-missing}") {
		return nil, errors.New("compose: x-messora-app has no image or pull_policy")
	}
	clearComments(mapGetPair(app, "image"))
	clearComments(mapGetPair(app, "pull_policy"))

	for _, name := range []string{"messora-init", "messora-web"} {
		svc := mapGet(services, name)
		if svc == nil {
			return nil, fmt.Errorf("compose: service %s missing", name)
		}
		mapDelete(svc, "build")
		mapSet(svc, "pull_policy", "${MESSORA_PULL:-missing}")
	}
	mapSet(mapGet(services, "messora-web"), "image", web)

	// Caddy: the Caddyfile travels inside this file.
	caddy := mapGet(services, "caddy")
	if caddy == nil {
		return nil, errors.New("compose: service caddy missing")
	}
	vols := mapGet(caddy, "volumes")
	found := false
	if vols != nil {
		kept := vols.Content[:0]
		for _, v := range vols.Content {
			if v.Kind == yaml.ScalarNode && strings.Contains(v.Value, "Caddyfile") {
				found = true
				continue
			}
			kept = append(kept, v)
		}
		vols.Content = kept
	}
	if !found {
		return nil, errors.New("compose: caddy has no Caddyfile mount")
	}
	mapAdd(caddy, "configs", &yaml.Node{Kind: yaml.SequenceNode, Content: []*yaml.Node{
		{Kind: yaml.MappingNode, Content: []*yaml.Node{
			scalar("source"), scalar("caddyfile"),
			scalar("target"), scalar("/etc/caddy/Caddyfile"),
		}},
	}})
	// Compose interpolates "$" in config content; Caddy's own {$VAR}
	// placeholders must reach Caddy untouched.
	content := strings.ReplaceAll(string(caddyfile), "$", "$$")
	mapAdd(root, "configs", &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{
		scalar("caddyfile"),
		{Kind: yaml.MappingNode, Content: []*yaml.Node{
			scalar("content"), {Kind: yaml.ScalarNode, Style: yaml.LiteralStyle, Value: content},
		}},
	}})

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func installHeader(registry string) string {
	return strings.TrimSpace(fmt.Sprintf(`
Messora — install file. Generated for each release; do not edit, put your
settings in .env next to it.

  docker login %[1]s -u <any name>     # password: your licence key
  echo "PUBLIC_APP_URL=https://messora.example.com" >> .env
  echo "SITE_ADDRESS=messora.example.com" >> .env   # automatic HTTPS
  docker compose up -d

Then open PUBLIC_APP_URL: the setup link is in "docker compose logs
messora-api". Updating: download the new compose.yml, then
"docker compose up -d". Behind Traefik/Dokploy see the operations guide.`, registry))
}

func scalar(v string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Value: v} }

func mapGetPair(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i]
		}
	}
	return nil
}

func mapGet(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func mapSet(m *yaml.Node, key, value string) bool {
	v := mapGet(m, key)
	if v == nil || v.Kind != yaml.ScalarNode {
		return false
	}
	v.Value, v.Style, v.Tag = value, 0, ""
	v.LineComment = ""
	return true
}

func mapAdd(m *yaml.Node, key string, value *yaml.Node) {
	m.Content = append(m.Content, scalar(key), value)
}

func mapDelete(m *yaml.Node, key string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}

func clearComments(n *yaml.Node) {
	if n != nil {
		n.HeadComment, n.LineComment = "", ""
	}
}
