package main

import (
	"database/sql"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"syscall"
	"time"
)

type httpError struct {
	code    int
	message string
}

type handler struct {
	stmt   *sql.Stmt
	pubSub *muxPubSub
}

func closeError(err error) bool {
	opErr, ok := err.(*net.OpError)
	return ok && (opErr.Err == syscall.EPIPE || opErr.Err == syscall.ECONNRESET)
}

func recvToClosed(err error) bool {
	return err == io.EOF || closeError(err)
}

func sentToClosed(err error) bool {
	return closeError(err)
}

func newHandler(db *sql.DB, pubSub *muxPubSub) (handler, error) {
	var err error
	h := handler{pubSub: pubSub}

	h.stmt, err = db.Prepare("SELECT users.account_id FROM oauth_access_tokens INNER JOIN users ON oauth_access_tokens.resource_owner_id = users.id WHERE oauth_access_tokens.token = $1")
	if err != nil {
		return handler{}, err
	}

	return h, nil
}

func serveErrorHTTP(writer http.ResponseWriter, request *http.Request, id string, err httpError) {
	log.Print(id, " ", err.message)

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(err.code)

	encoder := json.NewEncoder(writer)
	if encodeErr := encoder.Encode(err.message); encodeErr != nil {
		log.Print(id, " failed to encode error: ", err.message)
	}
}

func (h handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	const interval = 8589934592
	var ticker *time.Ticker
	var subscription <-chan data
	var unsubscribe func()

	done := request.Context().Done()
	header := writer.Header()

	if websocket.IsWebSocketUpgrade(request) {
		id := uuid.New().String()
		header.Set("X-Request-ID", id)

		account, authorizeErr := h.authorize(request)
		if (authorizeErr != httpError{}) {
			serveErrorHTTP(writer, request, id, authorizeErr)
			return
		}

		subscription, unsubscribe = h.pubSub.subscribe(request.FormValue("stream"), account, request)
		if subscription == nil || unsubscribe == nil {
			http.NotFound(writer, request)
			return
		}

		conn, connErr := (&websocket.Upgrader{}).Upgrade(writer, request,
			http.Header{"X-Request-ID": []string{id}})
		if connErr != nil {
			unsubscribe()
			log.Print(id, " failed to upgrade to WebSocket: ", connErr)
			return
		}

		notification := make(chan struct{})
		go func() {
			for {
				_, _, readErr := conn.NextReader()
				if readErr != nil {
					if _, ok := readErr.(*websocket.CloseError); ok || recvToClosed(readErr) || sentToClosed(readErr) {
						break
					}

					log.Print(id, " failed to read: ", readErr)
				}
			}

			close(notification)
		}()

		ticker = time.NewTicker(interval)

		log.Print(id, " starting stream for ", account)

	websocketStreaming:
		for {
			select {
			case <-done:
			case <-notification:
				break websocketStreaming

			case received, ok := <-subscription:
				if !ok {
					log.Print(id, " unsubscribed")
					break websocketStreaming
				}

				if err := conn.WriteJSON(received); err != nil {
					log.Print(id, " failed to write: ", err)

					if sentToClosed(err) {
						break websocketStreaming
					}
				}

			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(interval)); err != nil {
					log.Print(id, " failed to ping: ", err)

					if sentToClosed(err) {
						break websocketStreaming
					}
				}
			}
		}

		log.Print(id, " ending stream for ", account)
		closeErr := conn.Close()
		if closeErr != nil {
			log.Print(closeErr)
		}
	} else {
		id := uuid.New().String()
		header.Set("X-Request-ID", id)

		header.Set("Access-Control-Allow-Origin", "*")
		header.Set("Access-Control-Allow-Headers", "Authorization, Accept, Cache-Control")
		header.Set("Access-Control-Allow-Methods", "GET, OPTIONS")

		if request.Method == "OPTIONS" {
			return
		}

		account, authorizeErr := h.authorize(request)
		if (authorizeErr != httpError{}) {
			serveErrorHTTP(writer, request, id, authorizeErr)
			return
		}

		const prefix = "/api/v1/streaming/"
		path := strings.ToLower(request.URL.Path)

		if !strings.HasPrefix(path, prefix) {
			http.NotFound(writer, request)
			return
		}

		header.Set("Content-Type", "text/event-stream")
		header.Set("Transfer-Encoding", "identity")

		subscription, unsubscribe = h.pubSub.subscribe(path[len(prefix):], account, request)
		if subscription == nil || unsubscribe == nil {
			http.NotFound(writer, request)
			return
		}

		eventWriter := writer.(interface {
			http.Flusher
			http.ResponseWriter
		})

		ticker = time.NewTicker(interval)

		log.Print(id, " starting stream for ", account)

	eventStreaming:
		for {
			select {
			case <-done:
				break eventStreaming

			case received, ok := <-subscription:
				if !ok {
					break eventStreaming
				}

				log.Print(received)
				payload := []byte(received.Payload.unmarshalled)
				if len(payload) <= 0 {
					payload = received.Payload.marshalled
				}

				for _, bytes := range [...][]byte{
					[]byte("event: "),
					[]byte(received.Event),
					[]byte("\ndata: "),
					payload,
					[]byte{'\n', '\n'},
				} {
					_, writeErr := eventWriter.Write(bytes)
					if writeErr != nil {
						log.Print(id, " failed to write: ", writeErr)

						if sentToClosed(writeErr) {
							break eventStreaming
						}
					}
				}

				eventWriter.Flush()

			case <-ticker.C:
				_, writeErr := eventWriter.Write([]byte(":thump\n"))
				if writeErr != nil {
					log.Print(id, " failed to thump: ", writeErr)

					if sentToClosed(writeErr) {
						break eventStreaming
					}
				}
			}
		}

		log.Print(id, " ending stream for ", account)
	}

	unsubscribe()
	for range subscription {
	}
	ticker.Stop()
}

func (h handler) authorize(request *http.Request) (int64, httpError) {
	var token string
	if authorization := request.Header.Get("Authorization"); authorization != "" {
		token = strings.TrimPrefix(authorization, "Bearer ")
	} else {
		token = request.FormValue("access_token")
	}

	var id int64
	switch scanErr := h.stmt.QueryRow(token).Scan(&id); scanErr {
	case nil:
		return id, httpError{}

	case sql.ErrNoRows:
		return 0, httpError{
			code:    http.StatusUnauthorized,
			message: "Invalid access token",
		}

	default:
		log.Print(id, " failed to authorize: ", scanErr)
		return 0, httpError{
			code:    http.StatusInternalServerError,
			message: "Internal server error",
		}
	}
}

func (h handler) close() error {
	return h.stmt.Close()
}
