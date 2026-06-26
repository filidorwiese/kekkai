package config

type Config struct {
	Image    ImageConfig       `yaml:"image"`
	Mounts   []Mount           `yaml:"mounts"`
	Env      map[string]string `yaml:"env"`
	Firewall FirewallConfig    `yaml:"firewall"`
	Caps     []string          `yaml:"caps"`
	Claude   ClaudeConfig      `yaml:"claude"`
}

type ImageConfig struct {
	Base               string   `yaml:"base"`
	AptPackages        []string `yaml:"apt_packages"`
	GitDeltaVersion    string   `yaml:"git_delta_version"`
	ZshInDockerVersion string   `yaml:"zsh_in_docker_version"`
	TflintVersion      string   `yaml:"tflint_version"`
	ClaudeCodeVersion  string   `yaml:"claude_code_version"`
}

type Mount struct {
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	Readonly bool   `yaml:"readonly,omitempty"`
	Optional bool   `yaml:"optional,omitempty"`

	Skip bool `yaml:"-"`
}

type FirewallConfig struct {
	AllowGithubMeta bool     `yaml:"allow_github_meta"`
	AllowHostLan    bool     `yaml:"allow_host_lan"`
	AllowedDomains  []string `yaml:"allowed_domains"`
}

type ClaudeConfig struct {
	Args string `yaml:"args"`
}

type layer struct {
	Image    *imageLayer    `yaml:"image,omitempty"`
	Mounts   []Mount        `yaml:"mounts,omitempty"`
	Env      map[string]string `yaml:"env,omitempty"`
	Firewall *firewallLayer `yaml:"firewall,omitempty"`
	Caps     []string       `yaml:"caps,omitempty"`
	Claude   *claudeLayer   `yaml:"claude,omitempty"`
}

type imageLayer struct {
	Base               *string  `yaml:"base,omitempty"`
	AptPackages        []string `yaml:"apt_packages,omitempty"`
	GitDeltaVersion    *string  `yaml:"git_delta_version,omitempty"`
	ZshInDockerVersion *string  `yaml:"zsh_in_docker_version,omitempty"`
	TflintVersion      *string  `yaml:"tflint_version,omitempty"`
	ClaudeCodeVersion  *string  `yaml:"claude_code_version,omitempty"`
}

type firewallLayer struct {
	AllowGithubMeta *bool    `yaml:"allow_github_meta,omitempty"`
	AllowHostLan    *bool    `yaml:"allow_host_lan,omitempty"`
	AllowedDomains  []string `yaml:"allowed_domains,omitempty"`
}

type claudeLayer struct {
	Args *string `yaml:"args,omitempty"`
}
