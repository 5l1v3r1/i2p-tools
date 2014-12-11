package reseed

import (
	"bytes"
	"crypto/tls"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/PuerkitoBio/throttled"
	"github.com/PuerkitoBio/throttled/store"
	"github.com/gorilla/handlers"
	"github.com/justinas/alice"
)

const (
	I2P_USER_AGENT = "Wget/1.11.4"
)

type Server struct {
	*http.Server
	Reseeder Reseeder
}

func NewServer() *Server {
	config := &tls.Config{MinVersion: tls.VersionTLS10}
	h := &http.Server{TLSConfig: config}
	server := Server{h, nil}

	th := throttled.RateLimit(throttled.PerHour(120), &throttled.VaryBy{RemoteAddr: true}, store.NewMemStore(10000))

	middlewareChain := alice.New(proxiedMiddleware, loggingMiddleware, verifyMiddleware, th.Throttle)

	mux := http.NewServeMux()
	mux.Handle("/i2pseeds.su3", middlewareChain.Then(http.HandlerFunc(server.reseedHandler)))
	server.Handler = mux

	return &server
}

func (s *Server) reseedHandler(w http.ResponseWriter, r *http.Request) {
	peer := s.Reseeder.Peer(r)

	su3, err := s.Reseeder.PeerSu3Bytes(peer)
	if nil != err {
		http.Error(w, "500 Unable to get SU3", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=i2pseeds.su3")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(su3)), 10))

	io.Copy(w, bytes.NewReader(su3))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return handlers.CombinedLoggingHandler(os.Stdout, next)
}

func verifyMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if I2P_USER_AGENT != r.UserAgent() {
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func proxiedMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if prior, ok := r.Header["X-Forwarded-For"]; ok {
			r.RemoteAddr = prior[0]
		}

		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
