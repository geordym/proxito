package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	gmp "github.com/lqqyt2423/go-mitmproxy/proxy"

	"proxito/api"
	"proxito/config"
	"proxito/jvm"
	"proxito/proxy"
)

//go:embed frontend
var frontendFS embed.FS

//go:embed http-proxy-agent-1.3.9-dist.jar
var agentJarData []byte

func main() {
	homeDir, _ := os.UserHomeDir()
	proxitoDir := filepath.Join(homeDir, ".proxito")

	proxyHost := flag.String("host", "127.0.0.1", "proxy host")
	proxyPort := flag.String("port", "8080", "proxy port")
	uiPort    := flag.String("ui-port", "8282", "UI port")
	flag.Parse()

	// 1. Load persisted settings
	cfg, err := config.Load(proxitoDir)
	if err != nil {
		log.Printf("aviso: no pude leer settings: %v", err)
		cfg = &config.Config{}
	}
	if cfg.ProxyHost != "" {
		*proxyHost = cfg.ProxyHost
	}
	if cfg.ProxyPort != "" {
		*proxyPort = cfg.ProxyPort
	}

	// 2. Detect available JDKs
	jdks := jvm.DetectJDKs()

	// 3. Auto-detect Java if none saved
	autoDetected := cfg.JavaBin == ""
	autoPath := ""
	if autoDetected {
		cfg.JavaBin = jvm.BestJava(jdks)
		autoPath = cfg.JavaBin
		fmt.Printf("Java auto-detectado: %s\n", cfg.JavaBin)
	}

	// 4. Extract embedded agent JAR
	agentJar, err := extractJAR(proxitoDir)
	if err != nil {
		log.Fatalf("No pude extraer el agente JAR: %v", err)
	}

	// 5. Build components
	store     := proxy.NewStore(1000)
	intercept := proxy.NewInterceptHub()
	jvmMgr   := jvm.New(agentJar, cfg.JavaBin)
	jvmMgr.SetAutoAttach(cfg.AutoAttach, cfg.AutoAttachPatterns)

	// 6. Create the MITM proxy (go-mitmproxy manages CA cert in proxitoDir)
	proxyAddr := *proxyHost + ":" + *proxyPort
	gmpProxy, err := gmp.NewProxy(&gmp.Options{
		Addr:              proxyAddr,
		CaRootPath:        proxitoDir,
		StreamLargeBodies: 1 << 20, // buffer bodies up to 1 MB
	})
	if err != nil {
		log.Fatalf("No pude crear el proxy: %v", err)
	}

	// CA cert path go-mitmproxy stores for the user to trust
	certPath := filepath.Join(proxitoDir, "mitmproxy-ca-cert.pem")

	// 7. Build UI server
	srv := api.NewServer(
		store, intercept, jvmMgr,
		certPath, *proxyHost, *proxyPort, proxitoDir,
		autoDetected, autoPath,
		frontendFS,
	)

	srv.StartAutoAttach(context.Background())

	// 8. Wire the addon (broadcast → WebSocket hub)
	gmpProxy.AddAddon(proxy.NewAddon(store, intercept, func(msg any) {
		srv.Hub().Broadcast(msg)
	}))

	// 9. Start MITM proxy
	go func() {
		fmt.Printf("Proxy MitM en %s\n", proxyAddr)
		if err := gmpProxy.Start(); err != nil {
			log.Fatalf("proxy: %v", err)
		}
	}()

	// 10. Start UI server
	uiAddr := "localhost:" + *uiPort
	fmt.Printf("Proxito UI  → http://%s\n", uiAddr)
	fmt.Printf("CA cert     → %s\n", certPath)

	go openBrowser("http://" + uiAddr)

	if err := http.ListenAndServe(uiAddr, srv.Handler()); err != nil {
		log.Fatalf("ui: %v", err)
	}
}

func extractJAR(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	dest := filepath.Join(dir, "http-proxy-agent.jar")
	want := sha256.Sum256(agentJarData)
	if data, err := os.ReadFile(dest); err == nil {
		if got := sha256.Sum256(data); bytes.Equal(want[:], got[:]) {
			return dest, nil
		}
	}
	if err := os.WriteFile(dest, agentJarData, 0600); err != nil {
		return "", err
	}
	return dest, nil
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd, args = "cmd", []string{"/c", "start", url}
	case "darwin":
		cmd, args = "open", []string{url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	exec.Command(cmd, args...).Start()
}
