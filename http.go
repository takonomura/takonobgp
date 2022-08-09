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
		if e.NextHop == nil {
			res[i].NextHop = ""
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

func (s *HTTPServer) handleNetworkAdd(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Prefix string `json:"prefix"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request body", http.StatusBadRequest)
		return
	}
	if body.Prefix == "" {
		http.Error(w, "prefix is not specified", http.StatusBadRequest)
		return
	}
	_, prefix, err := net.ParseCIDR(body.Prefix)
	if err != nil {
		http.Error(w, "bad prefix value", http.StatusBadRequest)
		return
	}

	e := s.RIB.Find(prefix)
	if e != nil {
		http.Error(w, "network already exists in RIB", http.StatusBadRequest)
		return
	}

	if err := s.RIB.Update(&RIBEntry{
		Prefix:  prefix,
		Origin:  OriginAttributeIGP,
		ASPath:  ASPath{Sequence: true, Segments: []uint16{}},
		NextHop: nil,
		Source:  nil,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *HTTPServer) handleNetworkDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	prefixStr := r.URL.Query().Get("prefix")
	if prefixStr == "" {
		http.Error(w, "prefix is not specified", http.StatusBadRequest)
		return
	}

	_, prefix, err := net.ParseCIDR(prefixStr)
	if err != nil {
		http.Error(w, "bad prefix value", http.StatusBadRequest)
		return
	}

	e := s.RIB.Find(prefix)
	if e == nil {
		http.Error(w, "not found in RIB", http.StatusNotFound)
		return
	}
	if e.Source != nil {
		http.Error(w, "the entry is not managed by us", http.StatusForbidden)
		return
	}
	if err := s.RIB.Remove(e); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *HTTPServer) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/rib", s.handleRIB)
	mux.HandleFunc("/network/add", s.handleNetworkAdd)
	mux.HandleFunc("/network/delete", s.handleNetworkDelete)
	return http.ListenAndServe(addr, mux)
}
