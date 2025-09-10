package engine

import (
	"encoding/json"
	"io"
	"net/http"

	engine "github.com/glkeru/loyalty/engine/internal/interfaces"
	models "github.com/glkeru/loyalty/engine/internal/models"
	service "github.com/glkeru/loyalty/engine/internal/services"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

type RulesHandler struct {
	router *mux.Router
	db     engine.RuleStorage
	logger *zap.Logger
}

type CalculateResponse struct {
	Points int32 `json:"points"`
}

func NewHandler(db engine.RuleStorage, logger *zap.Logger) *RulesHandler {
	router := mux.NewRouter()
	handler := &RulesHandler{router, db, logger}
	router.HandleFunc("/calculate", handler.CalculateHandler).Methods(http.MethodPost)
	router.HandleFunc("/rules", handler.GetActiveRulesHandler).Methods(http.MethodGet)
	router.HandleFunc("/rule/{id}", handler.GetRuleHandler).Methods(http.MethodGet)
	router.HandleFunc("/all", handler.GetAllRulesHandler).Methods(http.MethodGet)
	router.HandleFunc("/rule", handler.SaveRuleHandler).Methods(http.MethodPost)

	return handler
}

func (r *RulesHandler) ServeHTTP(w http.ResponseWriter, res *http.Request) {
	r.router.ServeHTTP(w, res)
}

func (r *RulesHandler) Log(msg string, service string, err error) {
	r.logger.Error(msg,
		zap.String("service", service),
		zap.Error(err),
	)
}

// Расчет баллов
func (r RulesHandler) CalculateHandler(w http.ResponseWriter, req *http.Request) {
	serv, err := service.NewRuleEngineService(r.db, r.logger)
	if err != nil {
		r.Log("Service init", "CalculateHandler", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// заказ
	order := make(map[string]any)
	body, err := io.ReadAll(req.Body)
	if err != nil {
		r.Log("Get request body", "CalculateHandler", err)
		http.Error(w, "Body is empty", http.StatusBadRequest)
		return
	}
	defer req.Body.Close()
	err = json.Unmarshal(body, &order)
	if err != nil {
		r.Log("Unmarshal", "CalculateHandler", err)
		http.Error(w, "Body is not correct", http.StatusBadRequest)
		return
	}

	// расчет
	points := serv.Calculate(req.Context(), order)
	response := &CalculateResponse{points}

	// формирование ответа
	j, err := json.Marshal(response)
	if err != nil {
		r.Log("Marshal", "CalculateHandler", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(j)
}

// Получить активные правила
func (r RulesHandler) GetActiveRulesHandler(w http.ResponseWriter, req *http.Request) {
	rules, err := r.db.GetActiveRules(req.Context())
	if err != nil {
		r.Log("DB get", "GetActiveRulesHandler", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if rules == nil {
		r.Log("Active rules not found", "GetActiveRulesHandler", err)
		http.Error(w, "Active rules not found", http.StatusNotFound)
		return
	}
	j, err := json.Marshal(rules)
	if err != nil {
		r.Log("Marshal", "GetActiveRulesHandler", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(j)
}

// Получить все правила
func (r RulesHandler) GetAllRulesHandler(w http.ResponseWriter, req *http.Request) {
	rules, err := r.db.GetAllRules(req.Context())
	if err != nil {
		r.Log("DB get", "GetAllRulesHandler", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if rules == nil {
		r.Log("Rules not found", "GetAllRulesHandler", err)
		http.Error(w, "Rules not found", http.StatusNotFound)
		return
	}
	j, err := json.Marshal(rules)
	if err != nil {
		r.Log("Marshal", "GetAllRulesHandler", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(j)
}

// Получить правило
func (r RulesHandler) GetRuleHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	rule := r.db.GetRule(req.Context(), id)
	if rule.ID == uuid.Nil {
		http.Error(w, "Rule not found", http.StatusNotFound)
		return
	}
	j, err := json.Marshal(rule)
	if err != nil {
		r.Log("Marshal", "GetRuleHandler", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(j)
	w.Header().Set("Content-Type", "application/json") // вынести в middleware
}

// Создать/обновить правило
func (r RulesHandler) SaveRuleHandler(w http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		r.Log("DB get", "SaveRuleHandler", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer req.Body.Close()
	rule := &models.Rule{}
	err = json.Unmarshal(body, rule)
	if err != nil {
		r.Log("Unmarshal", "SaveRuleHandler", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = r.db.SaveRule(req.Context(), *rule)
	if err != nil {
		r.Log("SaveRule", "SaveRuleHandler", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
