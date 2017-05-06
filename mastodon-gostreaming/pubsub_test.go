package main

import (
	"database/sql"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"
)

type subscription struct {
	subscriber  <-chan data
	unsubscribe func()
}

func TestHashtagPubSub(t *testing.T) {
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

	stmt, stmtErr := db.Prepare("SELECT account_id FROM block_mutes WHERE account_id = ANY($1) AND target_account_id = ANY($2) GROUP BY account_id")
	if stmtErr != nil {
		t.Fatal(stmtErr)
	}

	defer func() {
		stmtErr = stmt.Close()
		if stmtErr != nil {
			t.Error(stmtErr)
		}
	}()

	t.Run("newHashtagPubSub", func(t *testing.T) {
		expected := hashtagPubSub{pubSubs: map[string]*muteablePubSub{}}
		result := newHashtagPubSub()

		if !reflect.DeepEqual(result, expected) {
			t.Errorf("expected %v, got %v", expected, result)
		}
	})

	t.Run("close", func(t *testing.T) {
		t.Parallel()

		muteable := newMuteablePubSub(stmt)

		subscription, _ := muteable.subscribe(1)
		hashtag := hashtagPubSub{pubSubs: map[string]*muteablePubSub{"": muteable}}
		hashtag.close()

		if _, ok := <-subscription; ok {
			t.Error("expected to close a pubSub, but it doesn't")
		}
	})

	t.Run("publish", func(t *testing.T) {
		t.Parallel()

		publishLocking := func(hashtag *hashtagPubSub, publication data, locker sync.Locker) {
			done := make(chan struct{})

			locker.Lock()
			go func() {
				hashtag.publish("", publication)
				close(done)
			}()

			select {
			case <-done:
				t.Fatal("expected subscribe to lock, but it doesn't")

			case <-time.After(67108864):
			}

			locker.Unlock()

			select {
			case <-done:

			case <-time.After(67108864):
				t.Fatal("timeout")
			}
		}

		muteable := newMuteablePubSub(stmt)
		defer muteable.close()

		subscriber, _ := muteable.subscribe(1)
		hashtag := hashtagPubSub{pubSubs: map[string]*muteablePubSub{"": muteable}}

		var published data
		publishLocking(&hashtag, published, hashtag.mutex.RLocker())

		received := <-subscriber
		if !reflect.DeepEqual(received, published) {
			t.Errorf("expected %v, got %v", published, received)
		}
	})

	t.Run("subscribe", func(t *testing.T) {
		t.Parallel()

		unsubscribe := func(t *testing.T, hashtag *hashtagPubSub, received func()) {
			hashtag.mutex.Lock()

			done := make(chan struct{})
			go func() {
				received()
				close(done)
			}()

			select {
			case <-done:
				t.Fatal("expected unsubscribe to lock, but it doesn't")

			case <-time.After(67108864):
			}

			hashtag.mutex.Unlock()

			select {
			case <-done:

			case <-time.After(67108864):
				t.Fatal("timeout")
			}
		}

		subscribeLocking := func(t *testing.T, hashtag *hashtagPubSub, locker sync.Locker) {
			subscriptionChan := make(chan subscription)

			locker.Lock()

			go func() {
				subscriber, unsubscribe := hashtag.subscribe(1, "", stmt)
				subscriptionChan <- subscription{subscriber, unsubscribe}
			}()

			select {
			case <-subscriptionChan:
				t.Fatal("expected subscribe to lock, but it doesn't")

			case <-time.After(67108864):
			}

			locker.Unlock()

			var s subscription
			select {
			case s = <-subscriptionChan:

			case <-time.After(67108864):
				t.Fatal("subscribe timeout")
			}

			if len(hashtag.pubSubs) != 1 {
				t.Fatalf("expected pubSubs length 1, got %v",
					len(hashtag.pubSubs))
			}

			pubSub, pubSubOK := hashtag.pubSubs[""]
			if !pubSubOK {
				t.Fatal(`expected pubSubs to have key "", but it doesn't`)
			}

			var published data
			go pubSub.publish(published)

			select {
			case received := <-s.subscriber:
				if !reflect.DeepEqual(received, published) {
					t.Fatalf(`expected to receive %v, got %v`,
						published, received)
				}

			case <-time.After(67108864):
				t.Fatal("receiving timeout")
			}

			unsubscribe(t, hashtag, s.unsubscribe)
		}

		subscribeUnlocking := func(t *testing.T, hashtag *hashtagPubSub, locker sync.Locker) {
			subscriptionChan := make(chan subscription)

			locker.Lock()

			go func() {
				subscriber, unsubscribe := hashtag.subscribe(1, "", stmt)
				subscriptionChan <- subscription{subscriber, unsubscribe}
			}()

			var s subscription
			select {
			case s = <-subscriptionChan:

			case <-time.After(67108864):
				t.Fatal("subscribe timeout")
			}

			locker.Unlock()

			if len(hashtag.pubSubs) != 1 {
				t.Fatalf("expected pubSubs length 1, got %v",
					len(hashtag.pubSubs))
			}

			pubSub, pubSubOK := hashtag.pubSubs[""]
			if !pubSubOK {
				t.Fatal(`expected pubSubs to have key "", but it doesn't`)
			}

			var published data
			go pubSub.publish(published)

			select {
			case received := <-s.subscriber:
				if !reflect.DeepEqual(received, published) {
					t.Fatalf(`expected to receive %v, got %v`,
						published, received)
				}

			case <-time.After(67108864):
				t.Fatal("receiving timeout")
			}

			unsubscribe(t, hashtag, s.unsubscribe)
		}

		t.Run("primary", func(t *testing.T) {
			t.Parallel()

			hashtag := hashtagPubSub{pubSubs: map[string]*muteablePubSub{}}

			subscribeLocking(t, &hashtag, hashtag.mutex.RLocker())
			if _, ok := hashtag.pubSubs[""]; ok {
				t.Error("expected a pubSub no one subscribes to be removed, but it isn't")
			}
		})

		t.Run("secondary", func(t *testing.T) {
			t.Parallel()

			t.Run("Lock", func(t *testing.T) {
				t.Parallel()

				muteable := newMuteablePubSub(stmt)
				defer muteable.close()

				subscriber, _ := muteable.subscribe(1)
				go func() {
					for range subscriber {
					}
				}()

				hashtag := hashtagPubSub{pubSubs: map[string]*muteablePubSub{"": muteable}}

				subscribeLocking(t, &hashtag, &hashtag.mutex)
				if _, ok := hashtag.pubSubs[""]; !ok {
					t.Error("expected a pubSub which has subscribers survives, but it doesn't")
				}
			})

			t.Run("RLock", func(t *testing.T) {
				t.Parallel()

				muetable := newMuteablePubSub(stmt)
				defer muetable.close()

				subscriber, _ := muetable.subscribe(1)
				go func() {
					for range subscriber {
					}
				}()

				hashtag := hashtagPubSub{pubSubs: map[string]*muteablePubSub{"": muetable}}

				subscribeUnlocking(t, &hashtag, hashtag.mutex.RLocker())
				if _, ok := hashtag.pubSubs[""]; !ok {
					t.Error("expected a pubSub which has subscribers survives, but it doesn't")
				}
			})
		})
	})
}

func TestMuteablePubSub(t *testing.T) {
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

	stmtInvalid, stmtInvalidErr := db.Prepare("SELECT 0")
	if stmtInvalidErr != nil {
		t.Fatal(stmtInvalidErr)
	}

	stmtInvalidErr = stmtInvalid.Close()
	if stmtInvalidErr != nil {
		t.Fatal(stmtInvalidErr)
	}

	stmtSelect1, stmtSelect1Err := db.Prepare("SELECT 1 WHERE $1::int[] IS NOT NULL OR $2::int[] IS NOT NULL")
	if stmtSelect1Err != nil {
		t.Fatal(stmtSelect1Err)
	}

	defer func() {
		stmtSelect1Err = stmtSelect1.Close()
		if stmtSelect1Err != nil {
			t.Error(stmtSelect1Err)
		}
	}()

	stmtSelectNothing, stmtSelectNothingErr := db.Prepare("SELECT 0 WHERE $1::int[] IS NULL OR $2::int[] IS NULL")
	if stmtSelectNothingErr != nil {
		t.Fatal(stmtSelectNothingErr)
	}

	defer func() {
		stmtSelectNothingErr = stmtSelectNothing.Close()
		if stmtSelectNothingErr != nil {
			t.Error(stmtSelectNothingErr)
		}
	}()

	t.Run("newMuteablePubSub", func(t *testing.T) {
		t.Parallel()

		muteable := newMuteablePubSub(stmtSelectNothing)
		defer close(muteable.publisher)

		if len(muteable.pubSub) != 0 {
			t.Errorf("expected pubSub length 0, got %v", len(muteable.pubSub))
		}

		c := make(chan data)
		muteable.pubSub[1] = &muteablePublisher{
			false,
			publisherSet{c: struct{}{}},
		}

		t.Run("error", func(t *testing.T) {
			muteable.publisher <- data{"update", dataPayload{unmarshalled: "@"}}
		})

		t.Run("success", func(t *testing.T) {
			var published data
			muteable.publisher <- published

			received := <-c
			if !reflect.DeepEqual(received, published) {
				t.Errorf("expected %v, got %v", published, received)
			}
		})
	})

	t.Run("close", func(t *testing.T) {
		t.Parallel()

		c := make(chan data)
		d := make(chan data)

		muteable := muteablePubSub{
			publisher: c,
			pubSub: map[int64]*muteablePublisher{
				1: &muteablePublisher{
					false,
					publisherSet{d: struct{}{}},
				},
			},
		}

		muteable.chanMutex.Lock()
		done := make(chan struct{})
		go func() {
			muteable.close()
			close(done)
		}()

		select {
		case <-done:
			t.Fatal("unexpectedly done before unlocking chanMutex")

		case <-time.After(67108864):
		}

		muteable.mutex.Lock()
		muteable.chanMutex.Unlock()

		select {
		case <-done:
			t.Fatal("unexpectedly done before unlocking mutex")

		case <-time.After(67108864):
		}

		muteable.mutex.Unlock()

		select {
		case <-done:

		case <-time.After(67108864):
			t.Fatal("timeout")
		}

		if _, ok := <-c; ok {
			t.Error("expected to close publisher, but it doesn't")
		}

		if _, ok := <-d; ok {
			t.Error("expected to close an entry channel, but it doesn't")
		}
	})

	t.Run("forward", func(t *testing.T) {
		testMuted := func(t *testing.T, forwarded data, forwardErr bool, stmt *sql.Stmt) {
			c := make(chan data)
			muteable := muteablePubSub{
				publisher: nil,
				pubSub: map[int64]*muteablePublisher{
					1: &muteablePublisher{
						false,
						publisherSet{c: struct{}{}},
					},
				},
			}

			muteable.mutex.Lock()
			done := make(chan struct{})
			go func() {
				if err := muteable.forward(forwarded, stmt); (err == nil) == forwardErr {
					t.Errorf("unexpected forward error %v", err)
				}

				close(done)
			}()

			select {
			case <-c:
				t.Error("unexpectedly received a message")

			case <-done:
				t.Error("unexpectedly done before unlocking mutex")

			case <-time.After(67108864):
			}

			muteable.chanMutex.Lock()
			muteable.mutex.Unlock()

			select {
			case <-c:
				t.Error("unexpectedly received a message")

			case <-done:

			case <-time.After(67108864):
			}

			muteable.chanMutex.Unlock()
		}

		testUnmuted := func(t *testing.T, forwarded data, forwardErr bool, stmt *sql.Stmt) {
			c := make(chan data)
			muteable := muteablePubSub{
				publisher: nil,
				pubSub: map[int64]*muteablePublisher{
					1: &muteablePublisher{
						false,
						publisherSet{c: struct{}{}},
					},
				},
			}

			muteable.mutex.Lock()
			done := make(chan struct{})
			go func() {
				if err := muteable.forward(forwarded, stmt); (err == nil) == forwardErr {
					t.Errorf("unexpected forward error %v", err)
				}

				close(done)
			}()

			select {
			case <-c:
				t.Error("unexpectedly received a message")

			case <-done:
				t.Error("unexpectedly done before unlocking mutex")

			case <-time.After(67108864):
			}

			muteable.chanMutex.Lock()
			muteable.mutex.Unlock()

			select {
			case <-c:
				t.Error("unexpectedly received a message")

			case <-done:
				t.Error("unexpectedly done before unlocking chanMutex")

			case <-time.After(67108864):
			}

			muteable.chanMutex.Unlock()

			select {
			case <-done:
				t.Fatal("unexpectedly done")

			case <-time.After(67108864):
			}

			select {
			case <-done:
				t.Fatal("unexpectedly done")

			case received, ok := <-c:
				if !ok {
					t.Error("unexpectedly closed")
				}

				if !reflect.DeepEqual(received, forwarded) {
					t.Errorf("expected %v, got %v",
						forwarded, received)
				}

			case <-time.After(67108864):
				t.Fatal("receving timeout")
			}

			select {
			case <-done:

			case <-time.After(67108864):
				t.Fatal("forwarding timeout")
			}
		}

		t.Run("update", func(t *testing.T) {
			t.Parallel()

			t.Run("invalid payload", func(t *testing.T) {
				t.Parallel()
				testMuted(t,
					data{
						"update",
						dataPayload{unmarshalled: ``},
					},
					true,
					stmtSelectNothing)
			})

			t.Run("invalid stmt", func(t *testing.T) {
				t.Parallel()

				testMuted(t,
					data{
						"update",
						dataPayload{unmarshalled: `{}`},
					},
					true,
					stmtInvalid)
			})

			t.Run("muted account", func(t *testing.T) {
				t.Parallel()

				testMuted(t,
					data{
						"update",
						dataPayload{unmarshalled: `{"account":{"id":1}}`},
					},
					false,
					stmtSelect1)
			})

			t.Run("muted mention", func(t *testing.T) {
				t.Parallel()

				testMuted(t,
					data{
						"update",
						dataPayload{unmarshalled: `{"mentions":[{"id":1}]}`},
					},
					false,
					stmtSelect1)
			})

			t.Run("muted reblog", func(t *testing.T) {
				t.Parallel()

				testMuted(t,
					data{
						"update",
						dataPayload{unmarshalled: `{"reblog":{"account":{"id":1}}}`},
					},
					false,
					stmtSelect1)
			})

			t.Run("normal", func(t *testing.T) {
				t.Parallel()

				testUnmuted(t,
					data{
						"update",
						dataPayload{unmarshalled: `{}`},
					},
					false,
					stmtSelectNothing)
			})
		})

		t.Run("non update", func(t *testing.T) {
			t.Parallel()
			testUnmuted(t, data{}, false, stmtSelectNothing)
		})
	})

	t.Run("publish", func(t *testing.T) {
		t.Parallel()

		c := make(chan data, 1)

		var published data
		muteablePubSub{publisher: c}.publish(published)

		received, ok := <-c
		if !ok {
			t.Error("expected to send a message, but the channel got closed.")
		}

		if !reflect.DeepEqual(received, published) {
			t.Errorf("expected %v, got %v", published, received)
		}
	})

	t.Run("subscribe", func(t *testing.T) {
		t.Parallel()

		testSubscribe := func(t *testing.T, muteable *muteablePubSub, expectedSetLen int) subscription {
			muteable.mutex.Lock()
			subscriptionChan := make(chan subscription)
			go func() {
				subscriber, unsubscribe := muteable.subscribe(1)
				subscriptionChan <- subscription{subscriber, unsubscribe}
			}()

			select {
			case <-subscriptionChan:
				t.Fatal("expected subscribe to lock, but it doesn't")

			case <-time.After(67108864):
			}

			muteable.mutex.Unlock()

			var s subscription
			select {
			case s = <-subscriptionChan:

			case <-time.After(67108864):
				t.Fatal("subscribe timeout")
			}

			if len(muteable.pubSub) != 1 {
				t.Fatalf("expected pubSub length 1, got %v",
					len(muteable.pubSub))
			}

			publisher, publisherOK := muteable.pubSub[1]
			if !publisherOK {
				t.Fatal("expected pubSub publisher has key 1, but it doesn't")
			}

			if len(publisher.set) != expectedSetLen {
				t.Fatalf("expected set length %v, got %v",
					expectedSetLen, len(publisher.set))
			}

			select {
			case <-s.subscriber:
				t.Fatal("unexpectedly received a message before publishing")

			case <-time.After(67108864):
			}

			var published data
			go func() {
				for p := range publisher.set {
					p <- published
				}
			}()

			select {
			case received := <-s.subscriber:
				if !reflect.DeepEqual(received, published) {
					t.Fatalf("expected to receive %v, got %v",
						published, received)
				}

			case <-time.After(67108864):
			}

			return s
		}

		testUnsubscribe := func(t *testing.T, muteable *muteablePubSub, s subscription) {
			muteable.mutex.Lock()
			unsubscribed := make(chan struct{})
			go func() {
				s.unsubscribe()
				close(unsubscribed)
			}()

			select {
			case <-unsubscribed:
				t.Fatal("expected unsubscribe to lock, but it doesn't")

			case <-time.After(67108864):
			}

			muteable.mutex.Unlock()

			select {
			case <-unsubscribed:

			case <-time.After(67108864):
				t.Fatal("unsubscribe timeout")
			}

			if _, ok := <-s.subscriber; ok {
				t.Error("expected subscriber to be closed after unsubscription, but it is not")
			}
		}

		t.Run("primary", func(t *testing.T) {
			t.Parallel()

			muteable := muteablePubSub{pubSub: map[int64]*muteablePublisher{}}
			s := testSubscribe(t, &muteable, 1)
			testUnsubscribe(t, &muteable, s)

			if len(muteable.pubSub) != 0 {
				t.Errorf("expected pubSub length 0, got %v",
					len(muteable.pubSub))
			}
		})

		t.Run("secondary", func(t *testing.T) {
			t.Parallel()

			seed := struct{}{}
			muteable := muteablePubSub{
				pubSub: map[int64]*muteablePublisher{
					1: &muteablePublisher{
						set: publisherSet{nil: seed},
					},
				},
			}

			s := testSubscribe(t, &muteable, 2)
			testUnsubscribe(t, &muteable, s)

			if len(muteable.pubSub) != 1 {
				t.Errorf("expectecd pubSub length 1, got %v",
					len(muteable.pubSub))
			}

			publisher := muteable.pubSub[1]

			if len(publisher.set) != 1 {
				t.Errorf("expected publisher length 1, got %v",
					len(publisher.set))
			}

			value, ok := publisher.set[nil]

			if !ok {
				t.Error("expected to leave a published, but it doesn't")
			}

			if value != seed {
				t.Errorf("expected %v, got %v", seed, value)
			}
		})

		t.Run("closed primary", func(t *testing.T) {
			t.Parallel()

			muteable := muteablePubSub{pubSub: map[int64]*muteablePublisher{}}
			s := testSubscribe(t, &muteable, 1)
			for publisher, _ := range muteable.pubSub[1].set {
				close(publisher)
			}
			testUnsubscribe(t, &muteable, s)

			if len(muteable.pubSub) != 0 {
				t.Errorf("expected pubSub length 0, got %v",
					len(muteable.pubSub))
			}
		})
	})
}

func TestUserPubSub(t *testing.T) {
	t.Parallel()

	t.Run("newUserPubSub", func(t *testing.T) {
		t.Parallel()

		user := newUserPubSub()
		defer close(user.publisher)

		if len(user.pubSub) != 0 {
			t.Errorf("expected pubSub length 0, got %v", len(user.pubSub))
		}

		c := make(chan data)
		user.pubSub[1] = publisherSet{c: struct{}{}}

		var published data
		user.publisher <- userPublication{published, 1}

		received := <-c
		if !reflect.DeepEqual(received, published) {
			t.Errorf("expected %v, got %v", published, received)
		}
	})

	t.Run("close", func(t *testing.T) {
		t.Parallel()

		dataChan := make(chan data)
		publicationChan := make(chan userPublication)

		user := userPubSub{
			publisher: publicationChan,
			pubSub:    map[int64]publisherSet{1: publisherSet{dataChan: struct{}{}}},
		}

		user.chanMutex.Lock()
		done := make(chan struct{})
		go func() {
			user.close()
			close(done)
		}()

		select {
		case <-done:
			t.Fatal("unexpectedly done before unlocking chanMutex")

		case <-time.After(67108864):
		}

		user.mutex.Lock()
		user.chanMutex.Unlock()

		select {
		case <-done:
			t.Fatal("unexpectedly done before unlocking mutex")

		case <-time.After(67108864):
		}

		user.mutex.Unlock()

		select {
		case <-done:

		case <-time.After(67108864):
			t.Fatal("timeout")
		}

		if _, ok := <-dataChan; ok {
			t.Error("expected to close an entry channel, but it doesn't")
		}

		if _, ok := <-publicationChan; ok {
			t.Error("expected to close publisher, but it doesn't")
		}
	})

	t.Run("forward", func(t *testing.T) {
		t.Parallel()

		c := make(chan data)
		user := userPubSub{
			publisher: nil,
			pubSub:    map[int64]publisherSet{1: publisherSet{c: struct{}{}}},
		}

		user.mutex.Lock()
		done := make(chan struct{})
		var forwarded data
		go func() {
			user.forward(userPublication{forwarded, 1})
			close(done)
		}()

		select {
		case <-c:
			t.Error("unexpectedly received a message")

		case <-done:
			t.Error("unexpectedly done before unlocking mutex")

		case <-time.After(67108864):
		}

		user.chanMutex.Lock()
		user.mutex.Unlock()

		select {
		case <-c:
			t.Error("unexpectedly received a message")

		case <-done:
			t.Error("unexpectedly done before unlocking chanMutex")

		case <-time.After(67108864):
		}

		user.chanMutex.Unlock()

		select {
		case <-done:
			t.Fatal("unexpectedly done")

		case <-time.After(67108864):
		}

		select {
		case <-done:
			t.Fatal("unexpectedly done")

		case received, ok := <-c:
			if !ok {
				t.Error("unexpectedly closed")
			}

			if !reflect.DeepEqual(received, forwarded) {
				t.Errorf("expected %v, got %v",
					forwarded, received)
			}

		case <-time.After(67108864):
			t.Fatal("receving timeout")
		}

		select {
		case <-done:

		case <-time.After(67108864):
			t.Fatal("forwarding timeout")
		}
	})

	t.Run("publish", func(t *testing.T) {
		t.Parallel()

		c := make(chan userPublication, 1)

		var publishedID int64
		var published data
		userPubSub{publisher: c}.publish(publishedID, published)

		received, ok := <-c
		if !ok {
			t.Error("expected to send a message, but the channel got closed.")
		}

		if !reflect.DeepEqual(received.data, published) {
			t.Errorf("expected data %v, got %v", published, received.data)
		}

		if received.id != publishedID {
			t.Errorf("expected id %v, got %v", publishedID, received.id)
		}
	})

	t.Run("subscribe", func(t *testing.T) {
		t.Parallel()

		testSubscribe := func(t *testing.T, user *userPubSub, expectedSetLen int) subscription {
			user.mutex.Lock()
			subscriptionChan := make(chan subscription)
			go func() {
				subscriber, unsubscribe := user.subscribe(1)
				subscriptionChan <- subscription{subscriber, unsubscribe}
			}()

			select {
			case <-subscriptionChan:
				t.Fatal("expected subscribe to lock, but it doesn't")

			case <-time.After(67108864):
			}

			user.mutex.Unlock()

			var s subscription
			select {
			case s = <-subscriptionChan:

			case <-time.After(67108864):
				t.Fatal("subscribe timeout")
			}

			if len(user.pubSub) != 1 {
				t.Fatalf("expected pubSub length 1, got %v",
					len(user.pubSub))
			}

			publisher, publisherOK := user.pubSub[1]
			if !publisherOK {
				t.Fatal("expected pubSub publisher has key 1, but it doesn't")
			}

			if len(publisher) != expectedSetLen {
				t.Fatalf("expected set length %v, got %v",
					expectedSetLen, len(publisher))
			}

			select {
			case <-s.subscriber:
				t.Fatal("unexpectedly received a message before publishing")

			case <-time.After(67108864):
			}

			var published data
			go func() {
				for p := range publisher {
					p <- published
				}
			}()

			select {
			case received := <-s.subscriber:
				if !reflect.DeepEqual(received, published) {
					t.Fatalf("expected to receive %v, got %v",
						published, received)
				}

			case <-time.After(67108864):
			}

			return s
		}

		testUnsubscribe := func(t *testing.T, user *userPubSub, s subscription) {
			user.mutex.Lock()
			unsubscribed := make(chan struct{})
			go func() {
				s.unsubscribe()
				close(unsubscribed)
			}()

			select {
			case <-unsubscribed:
				t.Fatal("expected unsubscribe to lock, but it doesn't")

			case <-time.After(67108864):
			}

			user.mutex.Unlock()

			select {
			case <-unsubscribed:

			case <-time.After(67108864):
				t.Fatal("unsubscribe timeout")
			}

			if _, ok := <-s.subscriber; ok {
				t.Error("expected subscriber to be closed after unsubscription, but it is not")
			}
		}

		t.Run("primary", func(t *testing.T) {
			t.Parallel()

			user := userPubSub{pubSub: map[int64]publisherSet{}}
			s := testSubscribe(t, &user, 1)
			testUnsubscribe(t, &user, s)

			if len(user.pubSub) != 0 {
				t.Errorf("expected pubSub length 0, got %v",
					len(user.pubSub))
			}
		})

		t.Run("secondary", func(t *testing.T) {
			t.Parallel()

			seed := struct{}{}
			user := userPubSub{
				pubSub: map[int64]publisherSet{1: publisherSet{nil: seed}},
			}

			s := testSubscribe(t, &user, 2)
			testUnsubscribe(t, &user, s)

			if len(user.pubSub) != 1 {
				t.Errorf("expectecd pubSub length 1, got %v",
					len(user.pubSub))
			}

			publisher := user.pubSub[1]

			if len(publisher) != 1 {
				t.Errorf("expected publisher length 1, got %v",
					len(publisher))
			}

			value, ok := publisher[nil]

			if !ok {
				t.Error("expected to leave a published, but it doesn't")
			}

			if value != seed {
				t.Errorf("expected %v, got %v", seed, value)
			}
		})

		t.Run("closed primary", func(t *testing.T) {
			t.Parallel()

			user := userPubSub{pubSub: map[int64]publisherSet{}}
			s := testSubscribe(t, &user, 1)
			for publisher, _ := range user.pubSub[1] {
				close(publisher)
			}
			testUnsubscribe(t, &user, s)

			if len(user.pubSub) != 0 {
				t.Errorf("expected pubSub length 0, got %v",
					len(user.pubSub))
			}
		})
	})
}

func TestMuxPubSub(t *testing.T) {
	t.Parallel()

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

	stmt, stmtErr := db.Prepare("SELECT 2 WHERE $1::int[] IS NOT NULL OR $2::int[] IS NOT NULL")
	if stmtErr != nil {
		t.Fatal(stmtErr)
	}

	defer func() {
		stmtErr = stmt.Close()
		if stmtErr != nil {
			t.Error(stmtErr)
		}
	}()

	t.Run("newMuxPubSub", func(t *testing.T) {
		t.Parallel()

		db, dbErr := openDB("production")
		if dbErr != nil {
			t.Fatal(dbErr)
		}

		t.Run("success", func(t *testing.T) {
			mux, muxErr := newMuxPubSub(db)
			if muxErr != nil {
				t.Error(muxErr)
			}

			mux.hashtag.close()
			mux.hashtagLocal.close()
			mux.public.close()
			mux.publicLocal.close()
			mux.user.close()

			stmtErr := mux.stmt.Close()
			if stmtErr != nil {
				t.Error(stmtErr)
			}
		})

		dbErr = db.Close()
		if dbErr != nil {
			t.Error(dbErr)
		}

		t.Run("error", func(t *testing.T) {
			_, muxErr := newMuxPubSub(db)
			if muxErr == nil {
				t.Error("expected an error, got <nil>")
			}
		})
	})

	t.Run("close", func(t *testing.T) {
		t.Parallel()

		mux := muxPubSub{
			hashtag:      newHashtagPubSub(),
			hashtagLocal: newHashtagPubSub(),
			public:       newMuteablePubSub(stmt),
			publicLocal:  newMuteablePubSub(stmt),
			user:         newUserPubSub(),
			stmt:         stmt,
		}

		hashtagSubscriber, _ := mux.hashtag.subscribe(1, "", stmt)
		hashtagLocalSubscriber, _ := mux.hashtagLocal.subscribe(1, "", stmt)
		publicSubscriber, _ := mux.public.subscribe(1)
		publicLocalSubscriber, _ := mux.publicLocal.subscribe(1)
		userSubscriber, _ := mux.user.subscribe(1)

		closeErr := mux.close()
		if closeErr != nil {
			t.Error(closeErr)
		}

		if _, ok := <-hashtagSubscriber; ok {
			t.Error("expected to close hashtag, but it doesn't")
		}

		if _, ok := <-hashtagLocalSubscriber; ok {
			t.Error("expected to close hashtagLocal, but it doesn't")
		}

		if _, ok := <-publicSubscriber; ok {
			t.Error("expected to close public, but it doesn't")
		}

		if _, ok := <-publicLocalSubscriber; ok {
			t.Error("expected to close publicLocal, but it doesn't")
		}

		if _, ok := <-userSubscriber; ok {
			t.Error("expected to close user, but it doesn't")
		}

		if _, err := stmt.Exec("{}", "{}"); err == nil {
			t.Error("expected to close stmt, but it doesn't")
		}
	})

	t.Run("publish", func(t *testing.T) {
		t.Parallel()

		t.Run("timeline:hashtag:", func(t *testing.T) {
			t.Parallel()

			mux := muxPubSub{hashtag: newHashtagPubSub(), stmt: stmt}
			defer mux.hashtag.close()

			subscriber, _ := mux.hashtag.subscribe(1, "", stmt)

			var published data
			if err := mux.publish("timeline:hashtag:", published); err != nil {
				t.Fatal(err)
			}

			received := <-subscriber
			if !reflect.DeepEqual(received, published) {
				t.Errorf("expected %v, got %v",
					published, received)
			}
		})

		t.Run("timeline:hashtag::local", func(t *testing.T) {
			t.Parallel()

			mux := muxPubSub{hashtagLocal: newHashtagPubSub(), stmt: stmt}
			defer mux.hashtagLocal.close()

			subscriber, _ := mux.hashtagLocal.subscribe(1, "", stmt)

			var published data
			if err := mux.publish("timeline:hashtag::local", published); err != nil {
				t.Fatal(err)
			}

			received := <-subscriber
			if !reflect.DeepEqual(received, published) {
				t.Errorf("expected %v, got %v",
					published, received)
			}
		})

		t.Run("timeline:public", func(t *testing.T) {
			t.Parallel()

			mux := muxPubSub{public: newMuteablePubSub(stmt)}
			defer mux.public.close()

			subscriber, _ := mux.public.subscribe(1)

			var published data
			if err := mux.publish("timeline:public", published); err != nil {
				t.Fatal(err)
			}

			received := <-subscriber
			if !reflect.DeepEqual(received, published) {
				t.Errorf("expected %v, got %v",
					published, received)
			}
		})

		t.Run("timeline:public:local", func(t *testing.T) {
			t.Parallel()

			mux := muxPubSub{publicLocal: newMuteablePubSub(stmt)}
			defer mux.publicLocal.close()

			subscriber, _ := mux.publicLocal.subscribe(1)

			var published data
			if err := mux.publish("timeline:public:local", published); err != nil {
				t.Fatal(err)
			}

			received := <-subscriber
			if !reflect.DeepEqual(received, published) {
				t.Errorf("expected %v, got %v",
					published, received)
			}
		})

		t.Run("timeline:1", func(t *testing.T) {
			t.Parallel()

			mux := muxPubSub{user: newUserPubSub()}
			defer mux.user.close()

			subscriber, _ := mux.user.subscribe(1)

			var published data
			if err := mux.publish("timeline:1", published); err != nil {
				t.Fatal(err)
			}

			received := <-subscriber
			if !reflect.DeepEqual(received, published) {
				t.Errorf("expected %v, got %v",
					published, received)
			}
		})

		t.Run("", func(t *testing.T) {
			if err := (&muxPubSub{}).publish("timeline:", data{}); err.Error() != `unknown channel: "timeline:"` {
				t.Errorf(`expected unknwon channel: "timeline:", got %v`, err)
			}
		})
	})

	t.Run("subscribe", func(t *testing.T) {
		t.Parallel()

		t.Run("hashtag", func(t *testing.T) {
			t.Parallel()

			mux := muxPubSub{hashtag: newHashtagPubSub(), stmt: stmt}
			defer mux.hashtag.close()

			subscription, _ := mux.subscribe("hashtag", 1,
				httptest.NewRequest("GET", "https://cb6e6126.ngrok.io/api/v1/streaming/hashtag?tag=", nil))

			var published data
			mux.hashtag.publish("", published)

			received := <-subscription
			if !reflect.DeepEqual(received, published) {
				t.Errorf("expected %v, got %v",
					published, received)
			}
		})

		t.Run("hashtag:local", func(t *testing.T) {
			t.Parallel()

			mux := muxPubSub{hashtagLocal: newHashtagPubSub(), stmt: stmt}
			defer mux.hashtagLocal.close()

			subscription, _ := mux.subscribe("hashtag:local", 1,
				httptest.NewRequest("GET", "https://cb6e6126.ngrok.io/api/v1/streaming/hashtag:local?tag=", nil))

			var published data
			mux.hashtagLocal.publish("", published)

			received := <-subscription
			if !reflect.DeepEqual(received, published) {
				t.Errorf("expected %v, got %v",
					published, received)
			}
		})

		t.Run("public", func(t *testing.T) {
			t.Parallel()

			mux := muxPubSub{public: newMuteablePubSub(stmt)}
			defer mux.public.close()

			subscription, _ := mux.subscribe("public", 1,
				httptest.NewRequest("GET", "https://cb6e6126.ngrok.io/api/v1/streaming/public", nil))

			var published data
			mux.public.publish(published)

			received := <-subscription
			if !reflect.DeepEqual(received, published) {
				t.Errorf("expected %v, got %v",
					published, received)
			}
		})

		t.Run("public:local", func(t *testing.T) {
			t.Parallel()

			mux := muxPubSub{publicLocal: newMuteablePubSub(stmt)}
			defer mux.publicLocal.close()

			subscription, _ := mux.subscribe("public:local", 1,
				httptest.NewRequest("GET", "https://cb6e6126.ngrok.io/api/v1/streaming/public:local", nil))

			var published data
			mux.publicLocal.publish(published)

			received := <-subscription
			if !reflect.DeepEqual(received, published) {
				t.Errorf("expected %v, got %v",
					published, received)
			}
		})

		t.Run("user", func(t *testing.T) {
			t.Parallel()

			mux := muxPubSub{user: newUserPubSub()}
			defer mux.user.close()

			subscription, _ := mux.subscribe("user", 1,
				httptest.NewRequest("GET", "https://cb6e6126.ngrok.io/api/v1/streaming/user", nil))

			var published data
			mux.user.publish(1, published)

			received := <-subscription
			if !reflect.DeepEqual(received, published) {
				t.Errorf("expected %v, got %v",
					published, received)
			}
		})

		t.Run("", func(t *testing.T) {
			t.Parallel()

			subscription, unsubscribe := (&muxPubSub{}).subscribe("", 1,
				httptest.NewRequest("GET", "https://cb6e6126.ngrok.io/api/v1/streaming/", nil))

			if subscription != nil {
				t.Errorf("expected subscription <nil>, got %v",
					subscription)
			}

			if unsubscribe != nil {
				t.Errorf("expected unsubscribe <nil>, got %v",
					unsubscribe)
			}
		})
	})
}
