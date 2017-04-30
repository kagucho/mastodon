package main

import (
	"encoding/json"
	"net/http"
)

type data struct {
	Event   string      `json:"event"`
	Payload dataPayload `json:"payload"`
}

type account struct {
	ID int64 `json:"id"`
}

type mention struct {
	ID int64 `json:"id"`
}

type reblog struct {
	Account account `json:"account"`
}

type payload struct {
	Account  account   `json:"account"`
	Mentions []mention `json:"mentions"`
	Reblog   reblog    `json:"reblog"`
}

type dataPayload struct {
	marshalled   []byte
	unmarshalled string
}

func (p *dataPayload) UnmarshalJSON(marshalled []byte) error {
	p.marshalled = marshalled

	if len(marshalled) >= 2 && marshalled[0] == '"' {
		return json.Unmarshal(marshalled, &p.unmarshalled)
	}

	return nil
}

func (p dataPayload) MarshalJSON() ([]byte, error) {
	return p.marshalled, nil
}

func getQuery(request *http.Request, query, account string) (string, string) {
	switch query {
	case "user":
		return "timeline:" + account, ""

	case "public":
		return "timeline:public", account

	case "public:local":
		return "timeline:public:local", account

	case "hashtag":
		return "timeline:hashtag:" + request.FormValue("tag"), account

	case "hashtag:local":
		return "timeline:hashtag:" + request.FormValue("tag") + ":local", account

	default:
		return "", ""
	}
}
