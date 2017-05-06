package main

import (
	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/websocket"
	"net"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestOpenDB(t *testing.T) {
	t.Parallel()

	for _, test := range [...]string{"", "production"} {
		test := test

		t.Run(test, func(t *testing.T) {
			t.Parallel()

			db, err := openDB(test)
			if err == nil {
				err = db.Close()
				if err != nil {
					t.Error(err)
				}
			} else {
				t.Error(err)
			}
		})
	}
}

func TestRedis(t *testing.T) {
	pub, pubErr := openRedis()
	if pubErr != nil {
		t.Fatal(pubErr)
	}

	sub, subErr := openRedis()
	if subErr != nil {
		t.Fatal(subErr)
	}

	t.Run("forward", func(t *testing.T) {
		sub := redis.PubSubConn{Conn: sub}

		if err := sub.PSubscribe("timeline:*"); err != nil {
			t.Fatal(err)
		}

		db, dbErr := openDB("production")
		if dbErr != nil {
			t.Fatal(dbErr)
		}

		defer func() {
			dbErr = db.Close()
			if dbErr != nil {
				t.Error(dbErr)
			}
		}()

		mux, muxErr := newMuxPubSub(db)
		if muxErr != nil {
			t.Fatal(muxErr)
		}

		defer func() {
			muxErr = mux.close()
			if muxErr != nil {
				t.Error(muxErr)
			}
		}()

		forwarded := make(chan error)
		go func() {
			forwarded <- forward(sub, &mux)
		}()

		dataChan, unsubscribe := mux.subscribe("user", 1, nil)
		defer unsubscribe()

		// wait for forward to initialize
		time.Sleep(33554432)

		t.Run("", func(t *testing.T) {
			if _, err := pub.Do("PUBLISH", "timeline:1", ""); err != nil {
				t.Error(err)
			}

			if err := pub.Flush(); err != nil {
				t.Error(err)
			}

			select {
			case received := <-dataChan:
				t.Error(`unexpectedly received`, received, `after publishing ""`)

			default:
			}
		})

		t.Run("{}", func(t *testing.T) {
			if _, err := pub.Do("PUBLISH", "timeline:1", "{}"); err != nil {
				t.Error(err)
			}

			if err := pub.Flush(); err != nil {
				t.Error(err)
			}

			if received := <-dataChan; !reflect.DeepEqual(received, data{}) {
				t.Errorf(`expected zero value after publishing {}, got %v`,
					received)
			}
		})

		t.Run("unsubscribe", func(t *testing.T) {
			select {
			case err := <-forwarded:
				t.Fatal(err)

			default:
			}

			if err := sub.PUnsubscribe(); err != nil {
				t.Fatal(err)
			}

			if err := <-forwarded; err != nil {
				t.Error(err)
			}
		})

		t.Run("closed", func(t *testing.T) {
			if err := sub.Close(); err != nil {
				t.Fatal(err)
			}

			if opErr := forward(sub, &mux).(*net.OpError); opErr.Err == nil {
				t.Errorf("expected non-nil *net.OpError, got %v", opErr)
			}
		})
	})

	if err := pub.Close(); err != nil {
		t.Error(err)
	}
}

func TestMain(t *testing.T) {
	env = "production"
	go main()

	db, _ := openTestDB(t)
	defer closeTestDB(t, db)

	sub, _, subErr := (&websocket.Dialer{}).Dial("ws://:"+os.Getenv("PORT")+"?stream=public", http.Header{
		"Authorization": []string{"Bearer mastodon-gostreaming-test-token"},
	})
	if subErr != nil {
		t.Fatal(subErr)
	}

	defer func() {
		closeErr := sub.WriteControl(websocket.CloseMessage, nil, time.Now().Add(33554432))
		if closeErr != nil {
			t.Error(closeErr)
		}
	}()

	pub, pubErr := openRedis()
	if pubErr != nil {
		t.Fatal(pubErr)
	}

	// wait for main to initialze
	time.Sleep(33554432)

	if _, err := pub.Do("PUBLISH", "timeline:public", `{"event": "delete", "payload": 1}`); err != nil {
		t.Fatal(err)
	}

	if err := pub.Close(); err != nil {
		t.Error(err)
	}

	var read data
	readErr := sub.ReadJSON(&read)
	if readErr != nil {
		t.Error(readErr)
	}

	if expected := (data{"delete", dataPayload{[]byte{'1'}, ""}}); !reflect.DeepEqual(read, expected) {
		t.Errorf("expected %v, got %v", expected, read)
	}
}
