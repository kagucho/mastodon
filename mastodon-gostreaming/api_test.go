package main

import (
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
