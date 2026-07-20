package models

import (
	"encoding/json"
	"log"
	"net/http"
)

type Response struct {
	StatusCode int               `json:"code,omitempty"`
	Header     map[string]string `json:"header,omitempty"`
	Message    string            `json:"message,omitempty"`
	Data       any               `json:"data,omitempty"`
}

func SendResponse(w http.ResponseWriter, status int, res Response) {
	jsonByte, err := json.Marshal(res)
	if err != nil {
		log.Println("Failed to marshaling the response")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(jsonByte)
}

