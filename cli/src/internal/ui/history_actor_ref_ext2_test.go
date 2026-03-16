package ui

import (
	"testing"

	"github.com/gravitrone/nebula-core/cli/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestFormatActorRefAdditionalBranches(t *testing.T) {
	actor := api.AuditActor{ActorType: "agent", ActorID: "raw-123456789"}
	assert.Equal(t, "agent:"+shortID("raw-123456789"), formatActorRef(actor))

	actor = api.AuditActor{ActorType: "agent", ActorID: "agent:   "}
	assert.Equal(t, "agent", formatActorRef(actor))
}

func TestInferActorTypeFromIDAdditionalBranches(t *testing.T) {
	assert.Equal(t, "", inferActorTypeFromID(":abc"))
	assert.Equal(t, "", inferActorTypeFromID(" :abc"))
}
