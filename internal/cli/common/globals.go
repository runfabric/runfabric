package common

// GlobalOptions holds global CLI flags shared across all subpackages
type GlobalOptions struct {
	ConfigPath     string
	Stage          string
	JSONOutput     bool
	NonInteractive bool
	AssumeYes      bool
	AutoInstallExt bool
	AppService     AppService
}
