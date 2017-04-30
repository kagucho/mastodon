package main

import (
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestDataPayload(t *testing.T) {
	t.Parallel()

	t.Run("UnmarshalJSON", func(t *testing.T) {
		t.Parallel()

		for _, test := range [...]struct {
			marshalled string
			expected   dataPayload
		}{
			{``, dataPayload{[]byte{}, ``}},
			{`" `, dataPayload{}},
			{`"{}"`, dataPayload{[]byte(`"{}"`), `{}`}},
			{`1`, dataPayload{[]byte{'1'}, ``}},
		} {
			test := test

			t.Run(test.marshalled, func(t *testing.T) {
				t.Parallel()

				var p dataPayload
				err := p.UnmarshalJSON([]byte(test.marshalled))

				if reflect.DeepEqual(test.expected, dataPayload{}) {
					if err == nil {
						t.Error("expected error, got nil")
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}

					if !reflect.DeepEqual(p, test.expected) {
						t.Errorf("expected %q, got %q",
							test.expected, p)
					}
				}
			})
		}
	})

	t.Run("MarshalJSON", func(t *testing.T) {
		t.Parallel()

		marshalled, err := dataPayload{}.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}

		if marshalled != nil {
			t.Errorf(`expected nil, got %v`, marshalled)
		}
	})
}

func TestGetQuery(t *testing.T) {
	t.Parallel()

	for _, test := range [...]struct {
		query             string
		expectedChannel   string
		expectedFiltering string
	}{
		{"", "", ""},
		{"user", "timeline:1", ""},
		{"public", "timeline:public", "1"},
		{"public:local", "timeline:public:local", "1"},
		{"hashtag", "timeline:hashtag:t", "1"},
		{"hashtag:local", "timeline:hashtag:t:local", "1"},
	} {
		test := test

		t.Run(test.query, func(t *testing.T) {
			t.Parallel()

			channel, filtering := getQuery(
				httptest.NewRequest("GET", "https://cb6e6126.ngrok.io/api/v1/streaming?tag=t", nil),
				test.query, "1")

			if channel != test.expectedChannel {
				t.Error("expected channel ", test.expectedChannel, ", got ", channel)
			}

			if filtering != test.expectedFiltering {
				t.Errorf("expected filtering account %v, got %v",
					test.expectedFiltering, filtering)
			}
		})
	}
}
