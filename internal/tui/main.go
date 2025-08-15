package tui

import (
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
)

func RunSSH(addr string, api Api) {
	srv, err := wish.NewServer(
		wish.WithAddress(addr),
		wish.WithHostKeyPath("ssh_host_key"),
		wish.WithMiddleware(
			bm.Middleware(func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
				return NewModel(api), []tea.ProgramOption{tea.WithAltScreen()}
			}),
			logging.Middleware(),
		),
	)
	if err != nil {
		log.Fatalf("server: %v", err)
	}

	// start
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()
	log.Printf("SSH TUI listening on %s (ssh -p %s 127.0.0.1)", addr, addr[1:])

	// graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
