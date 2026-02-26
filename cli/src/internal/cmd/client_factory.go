package cmd

import (
	"time"

	"github.com/gravitrone/nebula-core/cli/internal/api"
)

// newDefaultClient builds API clients for command flows.
//
// Tests override this variable to route command calls into local test servers
// without requiring the default localhost port to be free.
var newDefaultClient = func(apiKey string, timeout ...time.Duration) *api.Client {
	return api.NewDefaultClient(apiKey, timeout...)
}
