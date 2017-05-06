package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"github.com/gorilla/websocket"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

type responseFlushWriter interface {
	http.Flusher
	http.ResponseWriter
}

type failingResponseWriter struct {
	responseFlushWriter
	err error
}

func (writer failingResponseWriter) Write(buf []byte) (int, error) {
	return 0, writer.err
}

// net: Checking errors for EPIPE and ECONNRESET requires syscall · Issue #8319 · golang/go
// https://github.com/golang/go/issues/8319
func TestCloseError(t *testing.T) {
	t.Parallel()

	for _, test := range [...]struct {
		description string
		err         error
		expected    bool
	}{
		{"misc error", nil, false},
		{"misc OpError", &net.OpError{Err: nil}, false},
		{"ECONNRESET", &net.OpError{Err: syscall.ECONNRESET}, true},
		{"EPIPE", &net.OpError{Err: syscall.EPIPE}, true},
	} {
		test := test

		t.Run(test.description, func(t *testing.T) {
			t.Parallel()

			result := closeError(test.err)
			if result != test.expected {
				t.Errorf("expected %v, got %v",
					test.expected, result)
			}
		})
	}
}

func TestRecvToClosed(t *testing.T) {
	t.Parallel()

	for _, test := range [...]struct {
		description string
		err         error
		expected    bool
	}{
		{"misc error", nil, false},
		{"EOF", io.EOF, true},
		{"EPIPE", &net.OpError{Err: syscall.EPIPE}, true},
	} {
		test := test

		t.Run(test.description, func(t *testing.T) {
			t.Parallel()

			result := recvToClosed(test.err)
			if result != test.expected {
				t.Errorf("expected %v, got %v",
					test.expected, result)
			}
		})
	}
}

func TestSentToClosed(t *testing.T) {
	t.Parallel()

	for _, test := range [...]struct {
		description string
		err         error
		expected    bool
	}{
		{"misc error", nil, false},
		{"EPIPE", &net.OpError{Err: syscall.EPIPE}, true},
	} {
		test := test

		t.Run(test.description, func(t *testing.T) {
			t.Parallel()

			result := sentToClosed(test.err)
			if result != test.expected {
				t.Errorf("expected %v, got %v",
					test.expected, result)
			}
		})
	}
}

func TestServeErrorHTTP(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	serveErrorHTTP(recorder, httptest.NewRequest("GET", "https://cb6e6126.ngrok.io/", nil), "I",
		httpError{http.StatusNotFound, "not found"})

	if contentType := recorder.HeaderMap.Get("Content-Type"); contentType != "application/json" {
		t.Error("expected application/json Content-Type, got ", contentType)
	}

	if recorder.Code != http.StatusNotFound {
		t.Errorf("expected %v, got %v",
			http.StatusNotFound, recorder.Code)
	}

	if body := recorder.Body.String(); body != "\"not found\"\n" {
		t.Error(`expected "not found", got `, body)
	}
}

func TestHandler(t *testing.T) {
	t.Parallel()

	db, id := openTestDB(t)
	defer closeTestDB(t, db)

	strID := strconv.FormatInt(id, 10)

	closedStmt, stmtErr := db.Prepare("SELECT 0")
	if stmtErr != nil {
		t.Fatal(stmtErr)
	}

	closeErr := closedStmt.Close()
	if closeErr != nil {
		t.Fatal(closeErr)
	}

	pubSub, pubSubErr := newMuxPubSub(db)
	if pubSubErr != nil {
		t.Fatal(pubSubErr)
	}

	defer func() {
		pubSubErr = pubSub.close()
		if pubSubErr != nil {
			t.Error(pubSubErr)
		}
	}()

	h, hErr := newHandler(db, &pubSub)
	if hErr != nil {
		t.Fatal(hErr)
	}

	t.Run("ServeHTTP", func(t *testing.T) {
		expectUnauthorized := func(t *testing.T, response *http.Response) {
			if response.StatusCode != http.StatusUnauthorized {
				t.Errorf("expected status %v, got %v",
					http.StatusUnauthorized, response.StatusCode)
			}

			if id := response.Header.Get("X-Request-ID"); id == "" {
				t.Error("X-Request-ID: expected to be set")
			}

			body := make([]byte, 32)
			read, err := response.Body.Read(body)
			if err != io.EOF && err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(body[:read], []byte("\"Invalid access token\"\n")) {
				t.Errorf(`expected body "\"Invalid access token\"\n", got %q`, body)
			}
		}

		expectNotFound := func(t *testing.T, response *http.Response) {
			if response.StatusCode != http.StatusNotFound {
				t.Errorf("expected status %v, got %v",
					http.StatusNotFound, response.StatusCode)
			}

			if id := response.Header.Get("X-Request-ID"); id == "" {
				t.Error("X-Request-ID: expected to be set")
			}

			body := make([]byte, 32)
			read, err := response.Body.Read(body)
			if err != io.EOF && err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(body[:read], []byte("404 page not found\n")) {
				t.Errorf(`expected body "404 page not found\n", got %q`, body)
			}
		}

		t.Run("event stream", func(t *testing.T) {
			t.Run("GET", func(t *testing.T) {
				// HTML Standard
				// 9.2.3 Processing model
				// https://html.spec.whatwg.org/multipage/comms.html#sse-processing-model
				// > HTTP 200 OK responses with a
				// > `Content-Type` header specifying the
				// > type `text/event-stream`, ignoring
				// > any MIME type parameters, must be
				// > processed line by line as described
				// > below.
				// > (snip)
				// > HTTP 200 OK responses that have a
				// > Content-Type specifying an
				// > unsupported type, or that have no
				// > Content-Type at all, must cause the
				// > user agent to fail the connection.
				expectSuccessfulHeader := func(t *testing.T, recorder *httptest.ResponseRecorder) {
					if recorder.Code != http.StatusOK {
						t.Errorf("expected status %v, got %v",
							http.StatusOK, recorder.Code)
					}

					if contentType := recorder.HeaderMap.Get("Content-Type"); contentType != "text/event-stream" {
						t.Errorf(`Content-Type: expected "text/event-stream", got %q`,
							contentType)
					}

					if id := recorder.HeaderMap.Get("X-Request-ID"); id == "" {
						t.Error("X-Request-ID: expected to be set")
					}

					// 9.2.6 Authoring notes
					// https://html.spec.whatwg.org/multipage/comms.html#authoring-notes
					// > Authors are also cautioned that
					// > HTTP chunking can have unexpected
					// > negative effects on the reliability
					// > of this protocol, in particular if
					// > the chunking is done by a different
					// > layer unaware of the timing
					// > requirements. If this is a problem,
					// > chunking can be disabled for
					// > serving event streams.
					if transferEncoding := recorder.HeaderMap.Get("Transfer-Encoding"); transferEncoding != "identity" {
						t.Errorf(`Transfer-Encoding: expected "identity", got %q`,
							transferEncoding)
					}
				}

				recordWriting := func(d data, recorder *httptest.ResponseRecorder, writer http.ResponseWriter) (<-chan struct{}, func()) {
					request := httptest.NewRequest("GET", "https://cb6e6126.ngrok.io/api/v1/streaming/user", nil)
					request.Header.Set("Authorization", "Bearer mastodon-gostreaming-test-token")

					c, f := context.WithCancel(request.Context())
					done := make(chan struct{})

					go func() {
						h.ServeHTTP(writer, request.WithContext(c))

						done <- struct{}{}
						close(done)
					}()

					// wait for the server to subscribe
					time.Sleep(33554432)
					expectSuccessfulHeader(t, recorder)

					pubSub.publish("timeline:"+strID, d)

					// wait for the server to write the body.
					time.Sleep(33554432)

					return done, f
				}

				t.Run("unauthorized", func(t *testing.T) {
					recorder := httptest.NewRecorder()

					h.ServeHTTP(recorder,
						httptest.NewRequest("GET", "https://cb6e6126.ngrok.io/api/v1/streaming/user", nil))

					expectUnauthorized(t, recorder.Result())
				})

				t.Run("no prefix", func(t *testing.T) {
					recorder := httptest.NewRecorder()

					request := httptest.NewRequest("GET", "https://cb6e6126.ngrok.io/api/v1/streaming", nil)
					request.Header.Set("Authorization", "Bearer mastodon-gostreaming-test-token")

					h.ServeHTTP(recorder, request)
					expectNotFound(t, recorder.Result())
				})

				t.Run("not found", func(t *testing.T) {
					recorder := httptest.NewRecorder()

					request := httptest.NewRequest("GET", "https://cb6e6126.ngrok.io/api/v1/streaming/", nil)
					request.Header.Set("Authorization", "Bearer mastodon-gostreaming-test-token")

					h.ServeHTTP(recorder, request)
					expectNotFound(t, recorder.Result())
				})

				t.Run("writing", func(t *testing.T) {
					recorder := httptest.NewRecorder()
					done, cancel := recordWriting(data{}, recorder, recorder)

					if !recorder.Flushed {
						t.Error("expected http.ResponseWriter to be flushed, but it is not")
					}

					// HTML Standard
					// 9.2.4 Parsing an event stream
					// https://html.spec.whatwg.org/multipage/comms.html#parsing-an-event-stream
					if body := recorder.Body.String(); body != "event: \ndata: \n\n" {
						t.Errorf(`expected body "event: \ndata: \n\n", got %q`, body)
					}

					cancel()
					<-done
				})

				t.Run("writing unmarshalled", func(t *testing.T) {
					recorder := httptest.NewRecorder()

					done, cancel := recordWriting(
						data{Payload: dataPayload{[]byte(`"{}"`), `{}`}},
						recorder, recorder)

					if !recorder.Flushed {
						t.Error("expected http.ResponseWriter to be flushed, but it is not")
					}

					// HTML Standard
					// 9.2.4 Parsing an event stream
					// https://html.spec.whatwg.org/multipage/comms.html#parsing-an-event-stream
					if body := recorder.Body.String(); body != "event: \ndata: {}\n\n" {
						t.Errorf(`expected body "event: \ndata: \n\n", got %q`, body)
					}

					cancel()
					<-done
				})

				t.Run("misc writing failure", func(t *testing.T) {
					recorder := httptest.NewRecorder()

					done, cancel := recordWriting(
						data{},
						recorder,
						failingResponseWriter{
							recorder,
							errors.New("tried to write response writer with generic failure"),
						})

					cancel()
					<-done
				})

				t.Run("writing to closed", func(t *testing.T) {
					recorder := httptest.NewRecorder()

					done, cancel := recordWriting(
						data{},
						recorder,
						failingResponseWriter{
							recorder,
							&net.OpError{Err: syscall.EPIPE},
						})

					cancel()
					<-done
				})
			})

			t.Run("OPTIONS", func(t *testing.T) {
				recorder := httptest.NewRecorder()

				h.ServeHTTP(recorder,
					httptest.NewRequest("OPTIONS", "https://cb6e6126.ngrok.io//api/v1/streaming", nil))

				if origin := recorder.HeaderMap.Get("Access-Control-Allow-Origin"); origin != "*" {
					t.Errorf(`Allow-Control-Allow-Origin: expected "*", got %q`, origin)
				}

				if headers := recorder.HeaderMap.Get("Access-Control-Allow-Headers"); headers != "Authorization, Accept, Cache-Control" {
					t.Errorf(`Access-Control-Allow-Headers: expected "Authorization, Accept, Cache-Control", got %q`, headers)
				}

				if methods := recorder.HeaderMap.Get("Access-Control-Allow-Methods"); methods != "GET, OPTIONS" {
					t.Errorf(`Access-Control-Allow-Methods: expected "GET, OPTIONS", got %q`, methods)
				}

				if recorder.Code != http.StatusOK {
					t.Errorf("expected status %v, got %v",
						http.StatusUnauthorized, recorder.Code)
				}
			})
		})

		// RFC 6455 - The WebSocket Protocol
		// https://tools.ietf.org/html/rfc6455
		t.Run("WebSocket", func(t *testing.T) {
			dialer := websocket.Dialer{}

			server := httptest.NewServer(h)
			defer server.Close()

			url := "ws://" + strings.TrimPrefix(server.URL, "http://")

			testWriting := func(t *testing.T, suffix string, header http.Header) {
				conn, _, dialErr := dialer.Dial(url+suffix, header)
				if dialErr != nil {
					t.Fatal(dialErr)
				}

				defer func() {
					closeErr := conn.WriteControl(websocket.CloseMessage, nil, time.Now().Add(33554432))
					if closeErr != nil {
						t.Error(closeErr)
					}
				}()

				published := data{"", dataPayload{[]byte{'1'}, ""}}
				pubSub.publish("timeline:"+strID, published)

				var read data
				readErr := conn.ReadJSON(&read)
				if readErr != nil {
					t.Error(readErr)
				}

				if !reflect.DeepEqual(read, published) {
					t.Errorf("expected %v, got %v",
						published, read)
				}
			}

			t.Run("unauthorized", func(t *testing.T) {
				_, response, err := dialer.Dial(url, nil)
				if err != websocket.ErrBadHandshake {
					t.Fatal(err)
				}

				// 4.2.1.  Reading the Client's Opening Handshake
				// https://tools.ietf.org/html/rfc6455#section-4.2.2
				// > 10.  Optionally, other header fields, such as
				// >      those used to send cookies or request
				// >      authentication to a server.  Unknown
				// >      header fields are ignored, as per
				// >      [RFC2616].
				expectUnauthorized(t, response)
			})

			t.Run("unknown query", func(t *testing.T) {
				_, response, err := dialer.Dial(url, http.Header{
					"Authorization": []string{"Bearer mastodon-gostreaming-test-token"},
				})
				if err != websocket.ErrBadHandshake {
					t.Fatal(err)
				}

				// 4.2.2.  Sending the Server's Opening Handshake
				// https://tools.ietf.org/html/rfc6455#section-4.2.2
				// > If the requested service is not available,
				// > the server MUST send an appropriate HTTP
				// > error code (such as 404 Not Found) and
				// > abort the WebSocket handshake.
				expectNotFound(t, response)
			})

			// 4.2.1.  Reading the Client's Opening Handshake
			// https://tools.ietf.org/html/rfc6455#section-4.2.1
			// > If the server, while reading the handshake, finds
			// > that the client did not send a handshake that
			// > matches the description below (note that as per
			// > [RFC2616], the order of the header fields is not
			// > important), including but not limited to any
			// > violations of the ABNF grammar specified for the
			// > components of the handshake, the server MUST stop
			// > processing the client's handshake and return an
			// > HTTP response with an appropriate error code (such
			// > as 400 Bad Request).
			t.Run("upgrade failure", func(t *testing.T) {
				recorder := httptest.NewRecorder()

				// > 1.   An HTTP/1.1 or higher GET request,
				// > including a "Request-URI" [RFC2616] that
				// > should be interpreted as a /resource name/
				// > defined in Section 3 (or an absolute
				// > HTTP/HTTPS URI containing the
				// > /resource name/).
				// Required to let the server consider the
				// client is upgrading.
				request := httptest.NewRequest("GET", "https://cb6e6126.ngrok.io//api/v1/streaming?stream=user", nil)

				// > 5.   The request MUST contain an |Upgrade|
				// > header field whose value MUST include the
				// > "websocket" keyword.
				// Required to let the server consider the
				// client is upgrading.
				request.Header.Set("Upgrade", "websocket")

				// > 4.   A |Connection| header field that
				// > includes the token "Upgrade", treated as an
				// > ASCII case-insensitive value.
				// Required to let the server consider the
				// client is upgrading.
				request.Header.Set("Connection", "Upgrade")

				// > 6.   A |Sec-WebSocket-Version| header
				// > field, with a value of 13.
				// Not required to let the server consider the
				// client is upgrading, but leads to upgrade
				// failure if missing.

				// > 10.  Optionally, other header fields, such
				// >      as those used to send cookies or
				// >      request authentication to a server.
				// >      Unknown header fields are ignored, as
				// >      per [RFC2616].
				// Required for a valid API access.
				request.Header.Set("Authorization", "Bearer mastodon-gostreaming-test-token")

				h.ServeHTTP(recorder, request)
			})

			t.Run("writing", func(t *testing.T) {
				testWriting(t, "?stream=user", http.Header{
					"Authorization": []string{"Bearer mastodon-gostreaming-test-token"},
				})
			})

			t.Run("writing with access_token query", func(t *testing.T) {
				testWriting(t, "?access_token=mastodon-gostreaming-test-token&stream=user", nil)
			})
		})
	})

	t.Run("authorize", func(t *testing.T) {
		request := httptest.NewRequest("GET", "https://cb6e6126.ngrok.io/", nil)

		i := h

		for _, test := range [...]struct {
			authorization string
			stmt          *sql.Stmt
			expectedID    int64
			expectedError httpError
		}{
			{
				authorization: "",
				stmt:          h.stmt,
				expectedID:    0,
				expectedError: httpError{
					http.StatusUnauthorized,
					"Invalid access token",
				},
			},
			{
				authorization: "Bearer ",
				stmt:          closedStmt,
				expectedID:    0,
				expectedError: httpError{
					http.StatusInternalServerError,
					"Internal server error",
				},
			},
			{
				authorization: "Bearer mastodon-gostreaming-test-token",
				stmt:          h.stmt,
				expectedID:    id,
				expectedError: httpError{},
			},
		} {
			test := test

			t.Run(test.authorization, func(t *testing.T) {
				request.Header.Set("Authorization", test.authorization)
				i.stmt = test.stmt
				authorizedID, authorizeErr := i.authorize(request)

				if authorizedID != test.expectedID {
					t.Error("expected ID ", test.expectedID, ", got ", authorizedID)
				}

				if authorizeErr != test.expectedError {
					t.Errorf("expected error %v, got %v",
						test.expectedError, authorizeErr)
				}
			})
		}
	})

	t.Run("close", func(t *testing.T) {
		hErr := h.close()
		if hErr != nil {
			t.Error(hErr)
		}
	})
}
