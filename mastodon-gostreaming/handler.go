package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/juju/pubsub"
	"github.com/lib/pq"
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
	stmtSelectIDByToken *sql.Stmt
	stmtSelectMutedID   *sql.Stmt
	hub                 *pubsub.SimpleHub
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

func newHandler(db *sql.DB) (handler, error) {
	var err error
	h := handler{hub: pubsub.NewSimpleHub(nil)}

	h.stmtSelectIDByToken, err = db.Prepare("SELECT users.account_id FROM oauth_access_tokens INNER JOIN users ON oauth_access_tokens.resource_owner_id = users.id WHERE oauth_access_tokens.token = $1")
	if err != nil {
		return handler{}, err
	}

	h.stmtSelectMutedID, err = db.Prepare("(SELECT target_account_id FROM blocks WHERE account_id = $1 AND target_account_id = ANY($2) UNION SELECT target_account_id FROM mutes WHERE account_id = $1 AND target_account_id = ANY($2)) LIMIT 1")
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
	var unsubscribe func()

	done := context.Done()
	header := writer.Header()

	if websocket.IsWebSocketUpgrade(request) {
		id := uuid.New().String()
		header.Set("X-Request-ID", id)

		account, authorizeErr := h.authorize(request)
		if (authorizeErr != httpError{}) {
			serveErrorHTTP(writer, request, id, authorizeErr)
			return
		}

		channel, filtering := getQuery(request, request.FormValue("stream"), account)
		if channel == "" {
			http.NotFound(writer, request)
			return
		}

		conn, connErr := (&websocket.Upgrader{}).Upgrade(writer, request,
			http.Header{"X-Request-ID": []string{id}})
		if connErr != nil {
			log.Print(id, " failed to upgrade to WebSocket: ", connErr)
			return
		}

		subscription := make(chan interface{})
		unsubscribe = h.subscribe(channel, filtering, id, func(hubData data) {
			subscription <- hubData
		})

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

			case received := <-subscription:
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

		var channel string
		var filtering string
		if strings.HasPrefix(path, prefix) {
			channel, filtering = getQuery(request, path[len(prefix):], account)
		}

		if channel == "" {
			http.NotFound(writer, request)
			return
		}

		header.Set("Content-Type", "text/event-stream")
		header.Set("Transfer-Encoding", "identity")

		subscription := make(chan []byte)
		unsubscribe = h.subscribe(channel, filtering, id, func(hubData data) {
			buffer := bytes.NewBuffer(make([]byte, 0, 32))
			var writeErr error

			defer func() {
				if recovered := recover(); recovered != nil {
					recoveredErr, ok := recovered.(error)
					if !ok {
						panic(recovered)
					}

					log.Print(id, " failed to prepare buffer: ", recoveredErr)
				}
			}()

			_, writeErr = buffer.WriteString("event: ")
			if writeErr != nil {
				panic(writeErr)
			}

			_, writeErr = buffer.WriteString(hubData.Event)
			if writeErr != nil {
				panic(writeErr)
			}

			_, writeErr = buffer.WriteString("\ndata: ")
			if writeErr != nil {
				panic(writeErr)
			}

			if hubData.Payload.unmarshalled == "" {
				_, writeErr = buffer.Write(hubData.Payload.marshalled)
			} else {
				_, writeErr = buffer.WriteString(hubData.Payload.unmarshalled)
			}
			if writeErr != nil {
				panic(writeErr)
			}

			_, writeErr = buffer.Write([]byte{'\n', '\n'})
			if writeErr != nil {
				panic(writeErr)
			}

			subscription <- buffer.Bytes()
		})

		notification := writer.(http.CloseNotifier).CloseNotify()
		ticker = time.NewTicker(interval)

		log.Print(id, " starting stream for ", account)

	eventStreaming:
		for {
			select {
			case <-done:
			case <-notification:
				break eventStreaming

			case received := <-subscription:
				_, writeErr := writer.Write(received)
				if writeErr != nil {
					log.Print(id, " failed to write: ", writeErr)

					if sentToClosed(writeErr) {
						break eventStreaming
					}
				}

			case <-ticker.C:
				_, writeErr := writer.Write([]byte(":thump\n"))
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
	ticker.Stop()
}

func (h handler) authorize(request *http.Request) (string, httpError) {
	var token string
	if authorization := request.Header.Get("Authorization"); authorization != "" {
		token = strings.TrimPrefix(authorization, "Bearer ")
	} else {
		token = request.FormValue("access_token")
	}

	var id string
	switch scanErr := h.stmtSelectIDByToken.QueryRow(token).Scan(&id); scanErr {
	case nil:
		return id, httpError{}

	case sql.ErrNoRows:
		return "", httpError{
			code:    http.StatusUnauthorized,
			message: "Invalid access token",
		}

	default:
		log.Print(id, " failed to authorize: ", scanErr)
		return "", httpError{
			code:    http.StatusInternalServerError,
			message: "Internal server error",
		}
	}
}

func (h handler) subscribe(channel, filteringAccount, id string, handle func(data)) func() {
	var hubHandler func(string, interface{})

	if filteringAccount == "" {
		hubHandler = func(hubChannel string, hubData interface{}) {
			handle(hubData.(data))
		}
	} else {
		hubHandler = func(hubChannel string, hubData interface{}) {
			assertedData := hubData.(data)

			if assertedData.Event == "update" {
				var hubPayload payload
				if err := json.Unmarshal([]byte(assertedData.Payload.unmarshalled), &hubPayload); err != nil {
					log.Print(id, " failed to unmarshal payload: ", err)
					return
				}

				accounts := append(make([]int64, 0, len(hubPayload.Mentions)+2), hubPayload.Account.ID)

				for _, mention := range hubPayload.Mentions {
					accounts = append(accounts, mention.ID)
				}

				if hubPayload.Reblog.Account.ID != 0 {
					accounts = append(accounts, hubPayload.Reblog.Account.ID)
				}

				var account int64
				switch err := h.stmtSelectMutedID.QueryRow(filteringAccount, pq.Int64Array(accounts)).Scan(&account); err {
				case nil:
					return

				case sql.ErrNoRows:

				default:
					log.Print(id, " failed to query muted accounts: ", err)
					return
				}
			}

			handle(assertedData)
		}
	}

	return h.hub.Subscribe(channel, hubHandler)
}
