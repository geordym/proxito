package api

import (
	"encoding/json"
	"net/http"

	"proxito/proxy"
)

func (s *Server) handleListRules(w http.ResponseWriter, r *http.Request) {
	rules := s.rules.List()
	if rules == nil {
		rules = []proxy.Rule{}
	}
	jsonOK(w, rules)
}

func (s *Server) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	var rule proxy.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		jsonErr(w, "invalid body", http.StatusBadRequest)
		return
	}
	rule = s.rules.Add(rule)
	jsonOK(w, rule)
}

func (s *Server) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var rule proxy.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		jsonErr(w, "invalid body", http.StatusBadRequest)
		return
	}
	rule.ID = id
	if !s.rules.Update(rule) {
		jsonErr(w, "not found", http.StatusNotFound)
		return
	}
	jsonOK(w, rule)
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	if !s.rules.Delete(r.PathValue("id")) {
		jsonErr(w, "not found", http.StatusNotFound)
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}
