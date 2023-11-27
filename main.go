package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const limeric = `In the realm of requests and replies,
HTTP with its status denies.
With a 404 frown,
It turns users to clowns,
As they search for the page that belies.
`

func main() {
	httpaddr := "localhost:7080"
	flag.StringVar(&httpaddr, "http", httpaddr, "address to serve HTTP requests")

	logLevel := &slog.LevelVar{}
	flag.Func("log-level", "log level, default: "+logLevel.Level().String(), func(s string) error {
		return logLevel.UnmarshalText([]byte(s))
	})

	flag.Usage = func() {
		output := flag.CommandLine.Output()
		fmt.Fprintln(output,
			"badserv is a HTTP server that can be used to test HTTP clients.",
			"Client can force server to perform an action by passing 'action' query parameter.\n",
			"Available actions:\n"+
				"  - hang: server will hang on request until client closes connection\n"+
				"  - close: server will close connection without HTTP response\n"+
				"  - slow-write: server will write response slowly, byte by byte, 10 byte/s",
		)

		fmt.Fprintln(output, "\nFlags:")
		flag.PrintDefaults()
	}

	flag.Parse()

	logHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})
	logger := slog.New(&slogMeta{logHandler})
	slog.SetDefault(logger)

	srv := &service{}
	connIDs := new(atomic.Int64)
	server := &http.Server{
		Addr:              httpaddr,
		ReadHeaderTimeout: time.Hour,
		Handler:           srv,
		ErrorLog:          slog.NewLogLogger(logHandler.WithGroup("net/http"), slog.LevelDebug),
		ConnContext: func(ctx context.Context, _ net.Conn) context.Context {
			connID := connIDs.Add(1)

			return context.WithValue(ctx, connIDCtxKey{}, connID)
		},
	}

	slog.Info("Listening HTTP", "addr", httpaddr)

	errServe := server.ListenAndServe()

	switch {
	case errServe == nil,
		errors.Is(errServe, http.ErrServerClosed),
		errors.Is(errServe, context.Canceled):

		slog.Info("Bye!")
	default:
		panic("serving HTTP: " + errServe.Error())
	}
}

type service struct {
	counter atomic.Int64
}

func (srv *service) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	ctx := context.WithValue(req.Context(), requestIDKey{}, srv.counter.Add(1))

	dump, errInput := httputil.DumpRequest(req, true)
	if errInput != nil {
		slog.ErrorContext(ctx, "dumping request", errInput)
		http.Error(rw, "bad request: "+errInput.Error(), http.StatusBadRequest)
		return
	}

	msg := &strings.Builder{}

	writeStrs(msg,
		"---\n",
		string(dump), "\n",
		"---\n",
	)

	fmt.Println(msg)

	action := req.URL.Query().Get("action")
	slog.InfoContext(ctx, "handling", "action", action)

	switch action {
	case "":
		http.ServeContent(rw, req, "limeric.txt", time.Now(), strings.NewReader(limeric))
	case "hang":
		<-ctx.Done()
		return
	case "close":
		if err := closeConn(rw); err != nil {
			slog.ErrorContext(ctx, "closing connection", "error", err)
			http.Error(rw, "can't properly close connection", http.StatusInternalServerError)
		}
		return
	case "slow-write":
		if err := slowWrite(rw, req); err != nil {
			slog.ErrorContext(ctx, "writing response", "error", err)
			http.Error(rw, "can't properly write response", http.StatusInternalServerError)
		}
	default:
		http.Error(rw, "unknown action", http.StatusBadRequest)
	}
}

func slowWrite(rw http.ResponseWriter, req *http.Request) error {
	ctx := req.Context()
	controller := http.NewResponseController(rw)

	slog.InfoContext(ctx, "hijacking connection")

	conn, w, errHijack := controller.Hijack()
	if errHijack != nil {
		return fmt.Errorf("hijacking connection: %w", errHijack)
	}

	defer conn.Close()

	slog.InfoContext(ctx, "writing slow response")

	resp := &bytes.Buffer{}
	writeStrs(resp,
		"HTTP/1.1 200 OK\r\n",
		"Host: ", req.Host, "\r\n",
		"Content-Length: ", strconv.Itoa(len(limeric)), "\r\n",
		"Content-Type: text/plain\r\n\r\n",
	)
	resp.WriteString(limeric)

	for _, b := range resp.Bytes() {
		time.Sleep(100 * time.Millisecond)
		_, errWrite := w.Write([]byte{b})
		if errWrite != nil {
			return fmt.Errorf("writing response: %w", errWrite)
		}
		_ = w.Flush()
	}

	return nil
}

func writeStrs(b io.StringWriter, strs ...string) {
	for _, str := range strs {
		b.WriteString(str)
	}
}

func closeConn(rw http.ResponseWriter) error {
	controller := http.NewResponseController(rw)

	conn, _, errHijack := controller.Hijack()
	if errHijack != nil {
		return fmt.Errorf("hijacking connection: %w", errHijack)
	}

	_ = conn.Close()

	return nil
}
