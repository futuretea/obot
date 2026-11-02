package client

import (
	"testing"

	"github.com/obot-platform/obot/pkg/gateway/types"
)

func TestLogMCPAuditEntryDisabledDoesNotBuffer(t *testing.T) {
	c := &Client{
		auditBuffer:      make([]types.MCPAuditLog, 0, 4),
		kickAuditPersist: make(chan struct{}),
	}
	if !c.MCPAuditLogEnabled() {
		t.Fatal("MCPAuditLogEnabled() = false by default, want true")
	}

	c.DisableMCPAuditLog()
	if c.MCPAuditLogEnabled() {
		t.Fatal("MCPAuditLogEnabled() = true after DisableMCPAuditLog, want false")
	}

	c.LogMCPAuditEntry(types.MCPAuditLog{})
	if len(c.auditBuffer) != 0 {
		t.Fatalf("audit buffer length = %d, want 0", len(c.auditBuffer))
	}
}
