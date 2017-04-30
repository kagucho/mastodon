package main

import (
	"github.com/garyburd/redigo/redis"
	"github.com/gorilla/websocket"
	"github.com/juju/pubsub"
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
	var pub redis.Conn
	var sub redis.Conn

	if !t.Run("openRedis", func(t *testing.T) {
		var err error
		pub, err = openRedis()
		if err != nil {
			t.Fatal(err)
		}

		sub, err = openRedis()
		if err != nil {
			t.Fatal(err)
		}
	}) {
		t.FailNow()
	}

	t.Run("forward", func(t *testing.T) {
		subPubSub := redis.PubSubConn{Conn: sub}

		if err := subPubSub.PSubscribe("timeline:*"); err != nil {
			t.Fatal(err)
		}

		forwarded := make(chan error)
		hub := pubsub.NewSimpleHub(nil)
		go func() {
			forwarded <- forward(subPubSub, hub)
		}()

		dataChan := make(chan data)
		hub.Subscribe("timeline:1", func(hubChannel string, hubData interface{}) {
			dataChan <- hubData.(data)
		})

		// wait for forward to initialize
		time.Sleep(65536)

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

			if err := subPubSub.PUnsubscribe(); err != nil {
				t.Fatal(err)
			}

			if err := <-forwarded; err != nil {
				t.Error(err)
			}
		})

		t.Run("closed", func(t *testing.T) {
			if err := subPubSub.Close(); err != nil {
				t.Fatal(err)
			}

			if opErr := forward(subPubSub, hub).(*net.OpError); opErr.Err == nil {
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
		closeErr := sub.WriteControl(websocket.CloseMessage, nil, time.Now().Add(65536))
		if closeErr != nil {
			t.Error(closeErr)
		}
	}()

	pub, pubErr := openRedis()
	if pubErr != nil {
		t.Fatal(pubErr)
	}

	// wait for main to initialze
	time.Sleep(65536)

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
