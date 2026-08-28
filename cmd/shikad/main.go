// Command shikad is the shikA control-plane daemon. One copy runs on every
// device. It detects this device, discovers peers on the LAN (and via seeds
// over Tailscale), computes a deterministic cluster plan shared by every node,
// supervises this node's prima.cpp process, and serves the control API plus the
// device-management dashboard.
//
// It never performs inference itself — that is prima.cpp's job (the data plane).
// Nothing launches inference unless the operator opts in with -autostart or the
// dashboard Start button.
package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/braymix/shika/internal/api"
	"github.com/braymix/shika/internal/config"
	"github.com/braymix/shika/internal/discovery"
	"github.com/braymix/shika/internal/node"
	"github.com/braymix/shika/internal/supervisor"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "0.1.0-dev"

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	log.SetPrefix("shikad: ")

	var (
		configPath  = flag.String("config", "", "path to a JSON config file (optional; sane defaults otherwise)")
		name        = flag.String("name", "", "override this device's friendly name")
		addr        = flag.String("addr", "", "override the control API listen address (host:port)")
		autostart   = flag.Bool("autostart", false, "launch prima.cpp automatically once a plan is reached")
		showVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVersion {
		log.Printf("shikA %s", version)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	// Command-line flags win over the config file.
	if *name != "" {
		cfg.NodeName = *name
	}
	if *addr != "" {
		cfg.APIAddr = *addr
	}
	if *autostart {
		cfg.AutoStart = true
	}

	// cfg.APIAddr may be a wildcard like "0.0.0.0:8977", which is useless to
	// advertise to peers. Advertise a concrete, routable host:port instead so
	// seed discovery and prima.cpp addressing work.
	control := node.OutboundIP()
	if port := node.PortOf(cfg.APIAddr); port > 0 {
		control = net.JoinHostPort(control, strconv.Itoa(port))
	}

	self := node.Detect(cfg.NodeName, control, cfg.LLMPort)
	log.Printf("this device: %s (%s/%s, %.1f GB RAM, %d cores, gpu=%v) id=%s",
		self.Name, self.OS, self.Arch, self.RAMGB(), self.Cores, self.HasGPU, self.ID)

	reg := discovery.NewRegistry(self, cfg.PeerTimeout.Std())
	sup := supervisor.New(cfg.PrimaDir, self.ID)
	srv := api.New(cfg, reg, sup)

	// Start ordering: the head holds until its workers' prima.cpp processes are
	// up, coordinated over the control API. Workers are never gated.
	sup.SetReadiness(srv.WorkersReady)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Discovery: LAN multicast beacons + optional seed polling.
	go discovery.RunMulticast(ctx, reg, cfg.MulticastAddr, cfg.BeaconEvery.Std())
	go discovery.RunSeeds(ctx, reg, cfg.Seeds, cfg.BeaconEvery.Std())

	// Reconcile loop: while autostart is on, keep this node's prima.cpp process
	// matched to the current plan. Because the plan is a pure function of the
	// (identical) membership set, every node converges to the same launch with
	// no central coordination. When prima.cpp isn't built, the supervisor stays
	// in dry-run and nothing is executed.
	go reconcile(ctx, srv, sup)

	httpSrv := &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		sup.Stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
	}()

	log.Printf("control API + dashboard on http://%s  (advertising %s)", cfg.APIAddr, control)
	if cfg.AutoStart {
		log.Printf("autostart ENABLED: prima.cpp will launch once peers are present")
	} else {
		log.Printf("autostart disabled: press Start in the dashboard or use -autostart")
	}
	log.Printf("note: the control API is unauthenticated — use only on trusted home networks / tailnets")

	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http server: %v", err)
	}
	log.Printf("stopped")
}

// reconcile periodically applies the current plan while autostart is enabled.
func reconcile(ctx context.Context, srv *api.Server, sup *supervisor.Supervisor) {
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if !srv.AutoStart() {
				continue
			}
			if plan, ok := srv.Plan(); ok {
				sup.Apply(ctx, plan)
			}
		}
	}
}
