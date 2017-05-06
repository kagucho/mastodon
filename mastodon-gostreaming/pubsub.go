package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type publisherSet map[chan<- data]struct{}

// Do not copy hashtagPubSub in use.
type hashtagPubSub struct {
	// mutex locks pubSubs. It uses sync.RWMutex because there are readers
	// as many as hashtags already subscribed.
	mutex sync.RWMutex

	pubSubs map[string]*muteablePubSub
}

// Do not copy muteablePubSub in use.
type muteablePubSub struct {
	// mutex locks read and write of all fields.
	// It is fine to lock both of read and write because the number of
	// the reader, namely calls of forward function, is expected to be
	// small.
	// It is also fine to lock the all fields because the length of
	// publisherSet in muteablePublisher is expected to be very small.
	mutex sync.Mutex

	// chanMutex locks sending and receiving of publishers in pubSub.
	// It has a distinct mutex because it could be locked for a long time,
	// while it doesn't block some other expensive operations, namely
	// subscribe function and query of muted accounts in forward function.
	chanMutex sync.Mutex

	publisher chan<- data
	pubSub    map[int64]*muteablePublisher
}

type muteablePublisher struct {
	muting bool
	set    publisherSet
}

// Do not copy userPubSub in use.
type userPubSub struct {
	// mutex locks read and write of all fields.
	// It is fine to lock both of read and write because the number of
	// the reader, namely calls of forward function, is expected to be
	// small.
	// It is also fine to lock the all fields because the length of
	// publisherSet is expected to be very small.
	mutex sync.Mutex

	// chanMutex locks sending and receiving of publishers in pubSub.
	// It has a distinct mutex because it could be locked for a long time,
	// while it doesn't block an expensive operations, namely subscribe
	// function.
	chanMutex sync.Mutex

	publisher chan<- userPublication
	pubSub    map[int64]publisherSet
}

type userPublication struct {
	data
	id int64
}

// Do not copy muxPubSub in use.
type muxPubSub struct {
	hashtag      hashtagPubSub
	hashtagLocal hashtagPubSub
	public       *muteablePubSub
	publicLocal  *muteablePubSub
	user         *userPubSub
	stmt         *sql.Stmt
}

func newHashtagPubSub() hashtagPubSub {
	return hashtagPubSub{pubSubs: map[string]*muteablePubSub{}}
}

func newMuteablePubSub(stmt *sql.Stmt) *muteablePubSub {
	publisher := make(chan data)

	pubSub := muteablePubSub{
		publisher: publisher,
		pubSub:    map[int64]*muteablePublisher{},
	}

	go func() {
		for publication := range publisher {
			if err := pubSub.forward(publication, stmt); err != nil {
				log.Print(err)
			}
		}
	}()

	return &pubSub
}

func newUserPubSub() *userPubSub {
	publisher := make(chan userPublication)
	pubSub := userPubSub{pubSub: map[int64]publisherSet{}, publisher: publisher}

	go func() {
		for publication := range publisher {
			pubSub.forward(publication)
		}
	}()

	return &pubSub
}

func newMuxPubSub(db *sql.DB) (muxPubSub, error) {
	stmt, err := db.Prepare("SELECT account_id FROM block_mutes WHERE account_id = ANY($1) AND target_account_id = ANY($2) GROUP BY account_id")
	if err != nil {
		return muxPubSub{}, err
	}

	return muxPubSub{
		hashtag:      newHashtagPubSub(),
		hashtagLocal: newHashtagPubSub(),
		public:       newMuteablePubSub(stmt),
		publicLocal:  newMuteablePubSub(stmt),
		user:         newUserPubSub(),
		stmt:         stmt,
	}, nil
}

// close is not expected to be thread-safe with publish.
func (pubSub *hashtagPubSub) close() {
	pubSub.mutex.Lock()
	defer pubSub.mutex.Unlock()

	for _, muteablePubSub := range pubSub.pubSubs {
		muteablePubSub.close()
	}
}

// publish is not expected to be thread-safe with close.
func (pubSub *hashtagPubSub) publish(hashtag string, received data) {
	pubSub.mutex.Lock()
	defer pubSub.mutex.Unlock()

	muteablePubSub, muteablePubSubOK := pubSub.pubSubs[hashtag]
	if muteablePubSubOK {
		muteablePubSub.publish(received)
	}
}

// make sure to call the returned function AND drain the returned channel later.
func (pubSub *hashtagPubSub) subscribe(account int64, hashtag string, stmt *sql.Stmt) (<-chan data, func()) {
	locker := pubSub.mutex.RLocker()
	locker.Lock()

	muteablePubSub, muteablePubSubOK := pubSub.pubSubs[hashtag]
	if !muteablePubSubOK {
		locker.Unlock()

		locker = &pubSub.mutex
		locker.Lock()

		muteablePubSub, muteablePubSubOK = pubSub.pubSubs[hashtag]
		if !muteablePubSubOK {
			muteablePubSub = newMuteablePubSub(stmt)
			pubSub.pubSubs[hashtag] = muteablePubSub
		}
	}

	defer locker.Unlock()

	subscriptionChan, unsubscribe := muteablePubSub.subscribe(account)

	return subscriptionChan, func() {
		unsubscribe()

		pubSub.mutex.Lock()
		defer pubSub.mutex.Unlock()

		if len(muteablePubSub.pubSub) <= 0 {
			muteablePubSub.close()
			delete(pubSub.pubSubs, hashtag)
		}
	}
}

// close is not thread-safe with itself and publish.
func (pubSub *muteablePubSub) close() {
	close(pubSub.publisher)

	pubSub.chanMutex.Lock()
	defer pubSub.chanMutex.Unlock()

	pubSub.mutex.Lock()
	defer pubSub.mutex.Unlock()

	for _, publishers := range pubSub.pubSub {
		for publisher := range publishers.set {
			close(publisher)
		}
	}
}

// forward is NOT reentrant.
func (pubSub *muteablePubSub) forward(received data, stmt *sql.Stmt) error {
	pubSub.mutex.Lock()

	if received.Event == "update" {
		var hubPayload payload
		unmarshalErr := json.Unmarshal([]byte(received.Payload.unmarshalled), &hubPayload)
		if unmarshalErr != nil {
			pubSub.mutex.Unlock()
			return unmarshalErr
		}

		subscribers := newPQInt64Buffer(len(pubSub.pubSub))
		for subscriber := range pubSub.pubSub {
			subscribers.write(subscriber)
		}

		targets := newPQInt64Buffer(len(hubPayload.Mentions))
		targets.write(hubPayload.Account.ID)

		for _, mention := range hubPayload.Mentions {
			targets.write(mention.ID)
		}

		if hubPayload.Reblog.Account.ID != 0 {
			targets.write(hubPayload.Reblog.Account.ID)
		}

		if muteErr := func() error {
			rows, queryErr := stmt.Query(
				subscribers.finalize(), targets.finalize())
			if queryErr != nil {
				return queryErr
			}

			defer func() {
				closeErr := rows.Close()
				if closeErr != nil {
					log.Print(closeErr)
				}
			}()

			for rows.Next() {
				var id int64
				scanErr := rows.Scan(&id)
				if scanErr != nil {
					return scanErr
				}

				pubSub.pubSub[id].muting = true
			}

			return nil
		}(); muteErr != nil {
			pubSub.mutex.Unlock()
			return muteErr
		}
	}

	var group sync.WaitGroup
	pubSub.chanMutex.Lock()

	for _, publishers := range pubSub.pubSub {
		if publishers.muting {
			publishers.muting = false
		} else {
			group.Add(len(publishers.set))

			for publisher := range publishers.set {
				publisher := publisher

				go func() {
					publisher <- received
					group.Done()
				}()
			}
		}
	}

	pubSub.mutex.Unlock()
	group.Wait()
	pubSub.chanMutex.Unlock()

	return nil
}

// publish is not thread-safe with itself and close.
func (pubSub muteablePubSub) publish(received data) {
	pubSub.publisher <- received
}

// make sure to call the returned function AFTER finishing to receive, but
// NOT while receiving from the returned channel.
func (pubSub *muteablePubSub) subscribe(id int64) (<-chan data, func()) {
	c := make(chan data)

	pubSub.mutex.Lock()

	idPubSub, ok := pubSub.pubSub[id]
	if ok {
		idPubSub.set[c] = struct{}{}
	} else {
		idPubSub = &muteablePublisher{false, publisherSet{c: struct{}{}}}
		pubSub.pubSub[id] = idPubSub
	}

	pubSub.mutex.Unlock()

	return c, func() {
		pubSub.mutex.Lock()

		if len(idPubSub.set) > 1 {
			delete(idPubSub.set, c)
		} else {
			delete(pubSub.pubSub, id)
		}

		pubSub.mutex.Unlock()

		select {
		case _, ok := <-c:
			if !ok {
				return
			}

		default:
		}

		pubSub.chanMutex.Lock()
		close(c)
		pubSub.chanMutex.Unlock()
	}
}

// close is not thread-safe with itself and publish.
func (pubSub *userPubSub) close() {
	close(pubSub.publisher)

	pubSub.chanMutex.Lock()
	defer pubSub.chanMutex.Unlock()

	pubSub.mutex.Lock()
	defer pubSub.mutex.Unlock()

	for _, publishers := range pubSub.pubSub {
		for publisher := range publishers {
			close(publisher)
		}
	}
}

// forward is NOT reentrant.
func (pubSub *userPubSub) forward(publication userPublication) {
	pubSub.mutex.Lock()

	publishers := pubSub.pubSub[publication.id]

	var group sync.WaitGroup
	group.Add(len(publishers))

	pubSub.chanMutex.Lock()

	for publisher := range publishers {
		publisher := publisher

		go func() {
			publisher <- publication.data
			group.Done()
		}()
	}

	pubSub.mutex.Unlock()
	group.Wait()
	pubSub.chanMutex.Unlock()
}

// publish is not thread-safe with itself and close.
func (pubSub userPubSub) publish(id int64, received data) {
	pubSub.publisher <- userPublication{received, id}
}

// make sure to call the returned function AFTER finishing to receive, but
// NOT while receiving from the returned channel.
func (pubSub *userPubSub) subscribe(id int64) (<-chan data, func()) {
	c := make(chan data)

	pubSub.mutex.Lock()
	defer pubSub.mutex.Unlock()

	publishers, ok := pubSub.pubSub[id]
	if ok {
		publishers[c] = struct{}{}
	} else {
		publishers = publisherSet{c: struct{}{}}
		pubSub.pubSub[id] = publishers
	}

	return c, func() {
		pubSub.mutex.Lock()

		if len(publishers) > 1 {
			delete(publishers, c)
		} else {
			delete(pubSub.pubSub, id)
		}

		pubSub.mutex.Unlock()

		select {
		case _, ok := <-c:
			if !ok {
				return
			}

		default:
		}

		pubSub.chanMutex.Lock()
		close(c)
		pubSub.chanMutex.Unlock()
	}
}

// close is not expected to be thread-safe with publish.
func (pubSub *muxPubSub) close() error {
	pubSub.hashtag.close()
	pubSub.hashtagLocal.close()
	pubSub.public.close()
	pubSub.publicLocal.close()
	pubSub.user.close()

	return pubSub.stmt.Close()
}

// publish is not thread-safe with itself and close.
func (pubSub *muxPubSub) publish(channel string, received data) error {
	trimmed := channel[len("timeline:"):]

	if strings.HasPrefix(trimmed, "hashtag:") {
		hashtag := trimmed[len("hashtag:"):]
		if strings.HasSuffix(hashtag, ":local") {
			pubSub.hashtagLocal.publish(hashtag[:len(hashtag)-len(":local")], received)
		} else {
			pubSub.hashtag.publish(hashtag, received)
		}
	} else if strings.HasPrefix(trimmed, "public") {
		if trimmed[len("public"):] == ":local" {
			pubSub.publicLocal.publish(received)
		} else {
			pubSub.public.publish(received)
		}
	} else {
		account, parseErr := strconv.ParseInt(trimmed, 10, 64)
		if parseErr != nil {
			return fmt.Errorf("unknown channel: %q", channel)
		}

		pubSub.user.publish(account, received)
	}

	return nil
}

// make sure to call the returned function AND drain the returned channel later.
func (pubSub *muxPubSub) subscribe(query string, account int64, request *http.Request) (<-chan data, func()) {
	switch query {
	case "hashtag":
		return pubSub.hashtag.subscribe(account, request.FormValue("tag"), pubSub.stmt)

	case "hashtag:local":
		return pubSub.hashtagLocal.subscribe(account, request.FormValue("tag"), pubSub.stmt)

	case "public":
		return pubSub.public.subscribe(account)

	case "public:local":
		return pubSub.publicLocal.subscribe(account)

	case "user":
		return pubSub.user.subscribe(account)

	default:
		return nil, nil
	}
}
