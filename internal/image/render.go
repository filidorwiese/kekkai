package image

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/filidorwiese/kekkai/internal/config"
	kekkaiembed "github.com/filidorwiese/kekkai/embed"
)

type dockerfileVars struct {
	Base               string
	AptPackages        []string
	GitDeltaVersion    string
	ZshInDockerVersion string
	TflintVersion      string
	ClaudeCodeVersion  string
}

// RenderDockerfile expands the embedded Dockerfile template with config values.
func RenderDockerfile(cfg *config.Config) (string, error) {
	tmpl, err := template.New("Dockerfile").Parse(kekkaiembed.DockerfileTemplate)
	if err != nil {
		return "", err
	}
	v := dockerfileVars{
		Base:               cfg.Image.Base,
		AptPackages:        dedupe(cfg.Image.AptPackages),
		GitDeltaVersion:    cfg.Image.GitDeltaVersion,
		ZshInDockerVersion: cfg.Image.ZshInDockerVersion,
		TflintVersion:      cfg.Image.TflintVersion,
		ClaudeCodeVersion:  cfg.Image.ClaudeCodeVersion,
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, v); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func dedupe(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
