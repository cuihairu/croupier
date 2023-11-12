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

func createVersion() *version {
	ver = &version{
		majorVersion:  majorVersion,
		minorVersion:  minorVersion,
		patchVersion:  patchVersion,
		versionString: fmt.Sprintf("%d.%d.%d", majorVersion, minorVersion, patchVersion),
	}
	if len(GitCommit) > 0 {
		ver.versionString = fmt.Sprintf("%s+%s", ver.versionString, GitCommit)
	}
	return ver
}

// Format version to "<majorVersion>.<minorVersion>.<patchVersion>[+<gitCommit>]",
// like "1.0.0", or "1.0.0+1a2b3c4d".
func (v version) String() string {
	return v.versionString
}

func GetVersion() string {
	if ver != nil {
		return ver.String()
	}
	ver = createVersion()
	return ver.String()
}
