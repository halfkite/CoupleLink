package main

import (
	"context"
	"net"
	"couplelink/server/internal/api"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	// Standalone Windows/Linux releases must work without a Go installation or
	// an operating-system timezone database.
	_ "time/tzdata"
)

var version = "1.0-r63"

func portOf(addr string) string {
	if index := strings.LastIndex(addr, ":"); index >= 0 && index+1 < len(addr) {
		return addr[index+1:]
	}
	return addr
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	database := os.Getenv("DATABASE_URL")
	if database == "" {
		log.Fatal("DATABASE_URL is required")
	}
	config, err := pgxpool.ParseConfig(database)
	if err != nil {
		log.Fatal("invalid database configuration")
	}
	config.MaxConns = 8
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatal("database initialization failed")
	}
	defer db.Close()
	boot, stop := context.WithTimeout(ctx, 30*time.Second)
	defer stop()
	if err = api.Migrate(boot, db); err != nil {
		log.Fatal("database migration failed")
	}
	// The management page is protected by its own account. On the first start the
	// password is generated here and printed once, so it can only be read from the
	// console (or from the configured override) and must be changed after login.
	adminFile := os.Getenv("COUPLELINK_ADMIN_FILE")
	if adminFile == "" {
		adminFile = "admin-credentials.txt"
	}
	admin, err := api.EnsureAdmin(boot, db, os.Getenv("COUPLELINK_ADMIN_PASSWORD"), adminFile)
	if err != nil {
		log.Fatal("admin account initialization failed")
	}
	handler, err := api.New(db, api.Config{Version: version,
		RealtimeURL: os.Getenv("COUPLELINK_REALTIME_URL"), RealtimeAPIKey: os.Getenv("COUPLELINK_REALTIME_API_KEY"), RealtimeSecret: os.Getenv("COUPLELINK_REALTIME_SECRET"),
		AllowedOrigins: strings.Fields(os.Getenv("COUPLELINK_ALLOWED_ORIGINS")), AdminFile: adminFile, DownloadDir: os.Getenv("COUPLELINK_DOWNLOAD_DIR"), DataDir: os.Getenv("COUPLELINK_DATA_DIR"), PublicURL: strings.TrimRight(os.Getenv("COUPLELINK_PUBLIC_URL"), "/")})
	if err != nil {
		log.Fatal("API initialization failed")
	}
	go handler.RunRealtimeOutbox(ctx)
	go handler.RunAnniversaryReminders(ctx)
	addr := os.Getenv("COUPLELINK_LISTEN_ADDR")
	if addr == "" {
		addr = "0.0.0.0:30710"
	}
	server := &http.Server{Addr: addr, Handler: handler.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	stopped := make(chan struct{})
	go func() {
		<-ctx.Done()
		shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		server.Shutdown(shutdown)
		close(stopped)
	}()
	adminURL := "http://127.0.0.1:" + portOf(addr) + "/admin"
	log.Printf("CoupleLink %s listening at %s", version, addr)
	log.Printf("Web app:  http://127.0.0.1:%s/", portOf(addr))
	log.Printf("Admin:    %s", adminURL)
	if lan := lanURLs(portOf(addr)); len(lan) > 0 {
		log.Printf("局域网:   %s", strings.Join(lan, "  "))
	}
	log.Printf("==============================================================")
	log.Printf("管理页账号: %s", admin.Username)
	log.Printf("管理页密码: %s", admin.Password)
	log.Printf("admin-password: %s", admin.Password)
	log.Printf("密码文件: %s", adminFile)
	if admin.Generated {
		log.Printf("密码已生成或重置，首次登录后必须修改。")
	}
	log.Printf("每次启动都会显示当前密码；网页改密码后这里和密码文件同步更新。")
	log.Printf("==============================================================")
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("HTTP server failed")
	}
	cancel()
	<-stopped
}

// lanURLs lists the IPv4 addresses a phone on the same network can use.
func lanURLs(port string) []string {
	var out []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, value := range addrs {
			ipnet, ok := value.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil || ipnet.IP.IsLinkLocalUnicast() {
				continue
			}
			out = append(out, "http://"+ipnet.IP.String()+":"+port+"/")
		}
	}
	return out
}
