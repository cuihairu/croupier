// AUD-082: SupportedDriver is the up-front executability signal used by the
// generic backup create gate (reject before writing a pending record when the
// driver cannot be dumped).
package backupexec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSupportedDriver(t *testing.T) {
	for _, driver := range []string{"mysql", "postgres", "pg", "sqlite", "MySQL", " PG ", "sqlite"} {
		assert.True(t, SupportedDriver(driver), "driver %q 必须可执行备份", driver)
	}
	for _, driver := range []string{"", "oracle", "mssql", "cockroachdb", "unknown"} {
		assert.False(t, SupportedDriver(driver), "driver %q 不可执行备份", driver)
	}
}
