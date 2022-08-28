package pkg

type Config struct {
	Output   string
	GitRange GitRange
	Headers  []Header
}

type GitRange struct {
	From string
	To   string
}

type Header struct {
	Name     string
	Prefixes []string
}

func DefaultConfig() *Config {
	return &Config{
		GitRange: GitRange{
			To: "HEAD",
		},
		Headers: []Header{
			{
				Name:     ":sparkles: New Features",
				Prefixes: []string{":sparkles:", "✨"},
			},
			{
				Name:     ":lock: Fix security issues",
				Prefixes: []string{":lock:", "🔒"},
			},
			{
				Name:     ":bug: Bug Fixes",
				Prefixes: []string{":bug:", "🐛"},
			},
			{
				Name:     ":art: Improve structure / format of the code",
				Prefixes: []string{":art:", "🎨"},
			},
			{
				Name:     ":recycle: Refactor code",
				Prefixes: []string{":recycle:", "♻️"},
			},
			{
				Name:     ":memo: Documentation",
				Prefixes: []string{":memo:", "📝"},
			},
			{
				Name:     ":warning: Breaking Changes",
				Prefixes: []string{":warning:", "⚠️"},
			},
			{
				Name:     ":wrench: Add or update configuration files",
				Prefixes: []string{":wrench:", "🔧"},
			},
			{
				Name:     ":seedling: Others",
				Prefixes: []string{":seedling:", "🌱"},
			},
		},
	}
}
