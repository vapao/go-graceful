package graceful

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
	"io/ioutil"
	"fmt"
)

var listener net.Listener

func ListenAndServe(server *http.Server, pidFile string) {
	err := savePidToFile(pidFile)
	if err != nil {
		log.Fatalf("Update pid file error: %v", err)
	}
	if os.Getenv("_GRACEFUL_RESTART") == "true" {
		f := os.NewFile(3, "")
		listener, err = net.FileListener(f)
		log.Printf("Starting server listening on %s ... (graceful mode)", server.Addr)
	} else {
		listener, err = net.Listen("tcp", server.Addr)
		log.Printf("Starting server listening on %s ...", server.Addr)
	}

	if err != nil {
		log.Fatalf("listener error: %v", err)
	}

	go func() {
		log.Println(server.Serve(listener))
	}()
	signalHandler(server)
	log.Println("graceful shutdown")
}

func ListenAndServeTLS(server *http.Server, certFile, keyFile, pidFile string) {
	err := savePidToFile(pidFile)
	if err != nil {
		log.Fatalf("Update pid file error: %v", err)
	}
	if os.Getenv("_GRACEFUL_RESTART") == "true" {
		f := os.NewFile(3, "")
		listener, err = net.FileListener(f)
		log.Printf("Starting server listening on %s ... (graceful mode)", server.Addr)
	} else {
		listener, err = net.Listen("tcp", server.Addr)
		log.Printf("Starting server listening on %s ...", server.Addr)
	}

	if err != nil {
		log.Fatalf("listener error: %v", err)
	}

	go func() {
		log.Println(server.ServeTLS(listener, certFile, keyFile))
	}()
	signalHandler(server)
	log.Println("graceful shutdown")
}

func reload() error {
	tl, ok := listener.(*net.TCPListener)
	if !ok {
		return errors.New("listener is not tcp listener")
	}

	f, err := tl.File()
	if err != nil {
		return err
	}
	os.Setenv("_GRACEFUL_RESTART", "true")
	cmd := exec.Command(os.Args[0])
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// put socket FD at the first entry
	cmd.ExtraFiles = []*os.File{f}
	return cmd.Start()
}

func signalHandler(server *http.Server) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.Signal(12))
	for {
		sig := <-ch
		// timeout context for shutdown
		ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			// stop
			signal.Stop(ch)
			server.Shutdown(ctx)
			return
		case syscall.Signal(12):
			// reload
			err := reload()
			if err != nil {
				log.Fatalf("graceful restart error: %v", err)
			}
			time.Sleep(2 * time.Second)
			server.Shutdown(ctx)
			return
		}
	}
}

func savePidToFile(pidFile string) error {
	pid := fmt.Sprintf("%d", os.Getpid())
	return ioutil.WriteFile(pidFile, []byte(pid), 0644)
}
