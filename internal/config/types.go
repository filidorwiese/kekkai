package config

type Config struct {
	Image        ImageConfig       `yaml:"image"`
	Mounts       []Mount           `yaml:"mounts"`
	Env          map[string]string `yaml:"env"`
	Firewall     FirewallConfig    `yaml:"firewall"`
	Claude       ClaudeConfig      `yaml:"claude"`
	DockerAccess bool              `yaml:"docker_access"`
}

type ImageConfig struct {
	Base              string   `yaml:"base"`
	AptPackages       []string `yaml:"apt_packages"`
	ClaudeCodeVersion string   `yaml:"claude_code_version"`
	DockerCliVersion  string   `yaml:"docker_cli_version"`
}

type Mount struct {
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	Readonly bool   `yaml:"readonly,omitempty"`
	Optional bool   `yaml:"optional,omitempty"`

	Skip bool `yaml:"-"`
}

type FirewallConfig struct {
	AllowHostLan   bool     `yaml:"allow_host_lan"`
	AllowedDomains []string `yaml:"allowed_domains"`
}

type ClaudeConfig struct {
	Args string `yaml:"args"`
}

type layer struct {
	Image        *imageLayer       `yaml:"image,omitempty"`
	Mounts       []Mount           `yaml:"mounts,omitempty"`
	Env          map[string]string `yaml:"env,omitempty"`
	Firewall     *firewallLayer    `yaml:"firewall,omitempty"`
	Claude       *claudeLayer      `yaml:"claude,omitempty"`
	DockerAccess *bool             `yaml:"docker_access,omitempty"`
}

type imageLayer struct {
	Base              *string  `yaml:"base,omitempty"`
	AptPackages       []string `yaml:"apt_packages,omitempty"`
	ClaudeCodeVersion *string  `yaml:"claude_code_version,omitempty"`
	DockerCliVersion  *string  `yaml:"docker_cli_version,omitempty"`
}

type firewallLayer struct {
	AllowHostLan   *bool    `yaml:"allow_host_lan,omitempty"`
	AllowedDomains []string `yaml:"allowed_domains,omitempty"`
}

type claudeLayer struct {
	Args *string `yaml:"args,omitempty"`
}
