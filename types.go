package gogitcfg

import (
	"time"
)

const (
	DefaultTimeout = 30 * time.Second
)

const (
	// default system configuration file path.
	SystemConfigFile = "/etc/gitconfig"

	// default global configuration file name.
	GlobalConfigFile = ".gitconfig"

	// default local configuration file path.
	LocalConfigFile = ".git/config"

	// default worktree configuration file path.
	WorktreeConfigFile = ".git/config.worktree"

	// XDG configuration directory.
	XDGConfigDir = ".config/git/config"
)


const (
	CoreEditor                  = "core.editor"
	CoreAutoCRLF                = "core.autocrlf"
	CoreSafeCRLF                = "core.safecrlf"
	CoreFileMode                = "core.filemode"
	CoreSymlinks                = "core.symlinks"
	CoreIgnoreCase              = "core.ignorecase"
	CoreQuotePath               = "core.quotepath"
	CoreEOL                     = "core.eol"
	CorePager                   = "core.pager"
	CoreBare                    = "core.bare"
	CoreLogAllRefUpdates        = "core.logallrefupdates"
	CoreRepositoryFormatVersion = "core.repositoryformatversion"

	HTTPPostBuffer    = "http.postbuffer"
	HTTPSProxy        = "http.proxy"
	HTTPSLLVerify     = "http.sslverify"
	HTTPTimeout       = "http.timeout"
	HTTPLowSpeedLimit = "http.lowspeedlimit"
	HTTPLowSpeedTime  = "http.lowspeedtime"
)


type Remote struct {
	Name     string
	URL      string
	FetchURL string
	PushURL  string
	Fetch    []string
	Push     []string
}

type Branch struct {
	Name   string
	Remote string
	Merge  string
	Rebase string
}

type HTTPConfig struct {
	PostBuffer    int
	Proxy         string
	SSLVerify     bool
	Timeout       time.Duration
	LowSpeedLimit int
	LowSpeedTime  time.Duration
}

type CoreConfig struct {
	Editor                  string
	AutoCRLF                string
	SafeCRLF                string
	FileMode                bool
	Symlinks                bool
	IgnoreCase              bool
	PrecomposeUnicode       bool
	QuotePath               bool
	EOL                     string
	Pager                   string
	ExcludesFile            string
	AttributesFile          string
	HooksPath               string
	Worktree                string
	Bare                    bool
	LogAllRefUpdates        bool
	RepositoryFormatVersion int
}

