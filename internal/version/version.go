package version

import (
	"fmt"
)

var (
	GitCommit = ""
	ver       *version
)

const (
	majorVersion uint32 = 0
	minorVersion uint32 = 1
	patchVersion uint32 = 0
)

type version struct {
	majorVersion  uint32
	minorVersion  uint32
	patchVersion  uint32
	versionString string
}

// Format version to "<majorVersion>.<minorVersion>.<patchVersion>[+<gitCommit>]",
// like "1.0.0", or "1.0.0+1a2b3c4d".
func (v version) String() string {
	if len(GitCommit) > 0 {
		return fmt.Sprintf("%s+%s", v.versionString, GitCommit)
	}
	return v.versionString
}

func GetVersion() string {
	if ver != nil {
		return ver.String()
	}
	ver = &version{
		majorVersion:  majorVersion,
		minorVersion:  minorVersion,
		patchVersion:  patchVersion,
		versionString: fmt.Sprintf("%d.%d.%d", majorVersion, minorVersion, patchVersion),
	}
	return ver.String()
}
