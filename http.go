package main

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
)

type HTTPServer struct {
	RIB *RIB
}

func (s *HTTPServer) handleRIB(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rib := s.RIB.Entries()

	type aux struct {
		Prefix  string   `json:"prefix"`
		ASPath  []uint16 `json:"as_path"`
		NextHop string   `json:"next_hop"`
	}
	res := make([]aux, len(rib))
	for i, e := range rib {
		res[i] = aux{
			Prefix:  e.Prefix.String(),
			ASPath:  e.ASPath.Segments,
			NextHop: net.IP(e.NextHop).String(),
		}
	}

	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.Write(b)
}

func (s *HTTPServer) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/rib", s.handleRIB)
	return http.ListenAndServe(addr, mux)
}
