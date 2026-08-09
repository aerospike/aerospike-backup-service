package audit

import (
	"context"

	"github.com/aerospike/aerospike-backup-service/v3/internal/server"
)

type auditServer struct {
	underlying server.Server
	auditor    Auditor
}

var _ server.Server = (*auditServer)(nil)

// NewAuditServer wraps a server.Server with audit logging.
func NewAuditServer(underlying server.Server, auditor Auditor) server.Server {
	return &auditServer{
		underlying: underlying,
		auditor:    auditor,
	}
}

func (a *auditServer) Start() error {
	return a.underlying.Start()
}

func (a *auditServer) Shutdown() error {
	err := a.underlying.Shutdown()
	a.auditor.WriteEvent(context.Background(), "ServerShutdown", EventStatusFromError(err))
	return err
}
