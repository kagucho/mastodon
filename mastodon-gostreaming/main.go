// Command mastodon-gostreaming implements a streaming API server for Mastodon.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/garyburd/redigo/redis"
	"github.com/joho/godotenv"
	"github.com/juju/pubsub"
	_ "github.com/lib/pq"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
)

var env string

func forward(conn redis.PubSubConn, hub *pubsub.SimpleHub) error {
	for {
		received := conn.Receive()

		message, ok := received.(redis.PMessage)
		if ok {
			var d data
			if err := json.Unmarshal(message.Data, &d); err != nil {
				log.Print("failed to unmarshal data: ", err)
				continue
			}

			hub.Publish(message.Channel, d)
			continue
		}

		subscription, ok := received.(redis.Subscription)
		if ok {
			if subscription.Kind == "punsubscribe" && subscription.Channel == "timeline:*" && subscription.Count <= 0 {
				break
			}

			continue
		}

		receivedErr, ok := received.(error)
		if ok {
			return receivedErr
		}
	}

	return nil
}

func openDB(env string) (*sql.DB, error) {
	dsn := "dbname=mastodon_development host=/var/run/postgresql"
	if env == "production" {
		user := os.Getenv("DB_USER")
		pass := os.Getenv("DB_PASS")
		name := os.Getenv("DB_NAME")
		host := os.Getenv("DB_HOST")
		port := os.Getenv("DB_PORT")

		if user == "" {
			user = "mastodon"
		}

		if name == "" {
			name = "mastodon_production"
		}

		if host == "" {
			host = "localhost"
		}

		if port == "" {
			port = "5432"
		}

		fragments := append(make([]string, 16),
			"user=", user,
			" dbname=", name,
			" host=", host,
			" port=", port)

		if pass != "" {
			fragments = append(fragments, " password=", pass)
		}

		dsn = strings.Join(fragments, "")
	}

	log.Print(dsn)
	db, err := sql.Open("postgres", dsn)
	if err == nil {
		db.SetMaxOpenConns(10)
	}

	return db, err
}

func openRedis() (redis.Conn, error) {
	var option []redis.DialOption
	var network string
	var addr string

	if socket := os.Getenv("REDIS_SOCKET"); socket == "" {
		network = "tcp"
		addr = os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT")

		if password := os.Getenv("REDIS_PASSWORD"); password != "" {
			option = []redis.DialOption{redis.DialPassword(password)}
		}
	} else {
		network = "unix"
		addr = socket
	}

	return redis.Dial(network, addr, option...)
}

func init() {
	env = os.Getenv("GO_ENV")
	var envPath string

	switch env {
	case "production":
		envPath = ".env.production"

	case "":
		env = "development"
		fallthrough

	case "development":
		envPath = ".env"

	default:
		log.Fatal("Set GO_ENV to production or development. Defaults to development if unset.")
	}

	dotenvErr := godotenv.Load(envPath)
	if dotenvErr != nil {
		log.Fatal(dotenvErr)
	}
}

func main() {
	log.Print("starting streaming API server with ", runtime.GOMAXPROCS(0), " processors")

	db, dbErr := openDB(env)
	if dbErr != nil {
		log.Print(dbErr)
		return
	}

	defer func() {
		if dbErr := db.Close(); dbErr != nil {
			log.Print(dbErr)
		}
	}()

	h, hErr := newHandler(db)
	if hErr != nil {
		log.Print(hErr)
		return
	}

	server := http.Server{Handler: h}

	go func() {
		defer func() {
			if err := server.Shutdown(context.Background()); err != nil {
				log.Print(err)
			}
		}()

		redisConn, redisErr := openRedis()
		if redisErr != nil {
			log.Print(redisErr)
			return
		}

		redisSubConn := redis.PubSubConn{Conn: redisConn}

		subErr := redisSubConn.PSubscribe("timeline:*")
		if subErr != nil {
			log.Print(subErr)

			if redisErr := redisConn.Close(); redisErr != nil {
				log.Print(redisErr)
			}

			return
		}

		defer func() {
			unsubErr := redisSubConn.PUnsubscribe()
			if unsubErr != nil {
				log.Print(unsubErr)
			}
		}()

		forwarderEnded := make(chan struct{})
		go func() {
			defer func() {
				if redisErr := redisConn.Close(); redisErr != nil {
					log.Print(redisErr)
				}

				close(forwarderEnded)
			}()

			if err := forward(redisSubConn, h.hub); err != nil {
				log.Print(err)
			}
		}()

		signalChan := make(chan os.Signal, 2)
		signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

		select {
		case <-forwarderEnded:
		case <-signalChan:
		}
	}()

	log.Print("serving")

	var serveErr error
	if socket := os.Getenv("SOCKET"); socket == "" {
		server.Addr = ":" + os.Getenv("PORT")
		serveErr = server.ListenAndServe()
	} else {
		listener, listenerErr := net.Listen("unix", socket)
		if listenerErr != nil {
			log.Print(listenerErr)
		}

		defer func() {
			closeErr := listener.Close()
			if closeErr != nil {
				log.Print(closeErr)
			}
		}()

		serveErr = server.Serve(listener)
	}

	if serveErr != nil {
		log.Print(serveErr)
		return
	}
}
