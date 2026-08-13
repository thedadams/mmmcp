package stdio

import (
	"io"
	"log/slog"
	"os/exec"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/obot-platform/mmmcp/config"
)

type commandBuilder struct {
	lookupEnv         func(string) (string, bool)
	logger            *slog.Logger
	terminateDuration time.Duration
}

func (b commandBuilder) transport(server config.Server) *mcp.CommandTransport {
	cmd := exec.Command(server.Command, server.Args...)
	cmd.Dir = server.WorkingDirectory
	cmd.Env = Environment(b.lookupEnv, server.Env)
	cmd.Stderr = b.stderr(server.Name)
	return &mcp.CommandTransport{Command: cmd, TerminateDuration: b.terminateDuration}
}

func (b commandBuilder) stderr(component string) io.Writer {
	logger := b.logger
	if logger == nil {
		logger = slog.Default()
	}
	return slog.NewLogLogger(logger.With("component", component).Handler(), slog.LevelInfo).Writer()
}
