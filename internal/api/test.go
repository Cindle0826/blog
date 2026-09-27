package api

import (
	"encoding/json"
	"net/http"
)

func PingHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	ok := map[string]int{
		"status": 200,
	}
	if err := json.NewEncoder(w).Encode(ok); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
