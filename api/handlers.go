package main

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgconn"
	"github.com/jmoiron/sqlx"
)

type Chain struct {
	ID      string `json:"id" db:"id" validate:"required"`
	RPC     string `json:"rpc" db:"rpc" validate:"url"`
	WS      string `json:"ws" db:"ws" validate:"omitempty,url"`
	GRPC    string `json:"grpc" db:"grpc" validate:"omitempty,hostname_port"`
	REST    string `json:"rest" db:"rest" validate:"omitempty,url"`
	ETH_RPC string `json:"eth_rpc" db:"eth_rpc" validate:"omitempty,url"`
	ETH_WS  string `json:"eth_ws" db:"eth_ws" validate:"omitempty,url"`
}

type Project struct {
	ID         string    `json:"id" db:"id"`
	Name       string    `json:"name" db:"name" validate:"required"`
	Chain      string    `json:"chain" db:"chain" validate:"required"`
	Status     string    `json:"status" db:"status"`
	Secret     string    `json:"secret" db:"secret"`
	CreateTime time.Time `json:"-" db:"create_time"`
}

type Route struct {
	Route  bool `json:"route"`
	Target struct {
		RPC     string `json:"rpc" default:""`
		WS      string `json:"ws" default:""`
		GRPC    string `json:"grpc" default:""`
		REST    string `json:"rest" default:""`
		ETH_RPC string `json:"eth_rpc" default:""`
		ETH_WS  string `json:"eth_ws" default:""`
	} `json:"target"`
}

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"msg"`
	Data    interface{} `json:"data,omitempty"`
	Error   error       `json:"error,omitempty"`
}

func NewResponse(code int, data interface{}, err error) *Response {
	return &Response{
		Code:    code,
		Message: http.StatusText(code),
		Data:    data,
		Error:   err,
	}
}

type Handler struct {
	db       *sqlx.DB
	validate *validator.Validate
	cache    *lru.Cache
}

func NewHandler(db *sqlx.DB) *Handler {
	cache, _ := lru.New(128)
	return &Handler{
		db:       db,
		validate: validator.New(),
		cache:    cache,
	}
}

func (h *Handler) ListChains(w http.ResponseWriter, r *http.Request) {
	chains := []Chain{}
	if err := h.db.Select(&chains, "SELECT * FROM chains"); err != nil {
		render.Respond(w, r, NewResponse(http.StatusInternalServerError, nil, err))
	} else {
		render.Respond(w, r, NewResponse(http.StatusOK, chains, nil))
	}
}

func (h *Handler) GetChain(w http.ResponseWriter, r *http.Request) {
	chainID := chi.URLParam(r, "chainID")
	chain := Chain{}
	if err := h.db.Get(&chain, "SELECT * FROM chains WHERE id=$1", chainID); err != nil {
		render.Respond(w, r, NewResponse(http.StatusNotFound, nil, err))
	} else {
		render.Respond(w, r, NewResponse(http.StatusOK, chain, nil))
	}
}

func (h *Handler) CreateChain(w http.ResponseWriter, r *http.Request) {
	chain := Chain{}
	if err := render.Decode(r, &chain); err != nil {
		render.Respond(w, r, NewResponse(http.StatusBadRequest, nil, err))
		return
	}
	if err := h.validate.Struct(chain); err != nil {
		render.Respond(w, r, NewResponse(http.StatusBadRequest, nil, err))
		return
	}

	if _, err := h.db.NamedExec(`INSERT INTO chains VALUES (:id,:rpc,:ws,:grpc,:rest,:eth_rpc,:eth_ws)`, chain); err != nil {
		// UniqueViolation 23505
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			render.Respond(w, r, NewResponse(http.StatusConflict, nil, err))
		} else {
			render.Respond(w, r, NewResponse(http.StatusInternalServerError, nil, err))
		}
	} else {
		render.Respond(w, r, NewResponse(http.StatusOK, chain, nil))
	}
}

func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	projects := []Project{}
	if err := h.db.Select(&projects, "SELECT * FROM projects"); err != nil {
		render.Respond(w, r, NewResponse(http.StatusInternalServerError, nil, err))
	} else {
		render.Respond(w, r, NewResponse(http.StatusOK, projects, nil))
	}
}

func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	project := Project{}
	if err := h.db.Get(&project, "SELECT * FROM projects WHERE id=$1", projectID); err != nil {
		render.Respond(w, r, NewResponse(http.StatusNotFound, nil, err))
	} else {
		render.Respond(w, r, NewResponse(http.StatusOK, project, nil))
	}
}

func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	project := Project{}
	if err := render.Decode(r, &project); err != nil {
		render.Respond(w, r, NewResponse(http.StatusBadRequest, nil, err))
		return
	}
	if err := h.validate.Struct(project); err != nil {
		render.Respond(w, r, NewResponse(http.StatusBadRequest, nil, err))
		return
	}

	chain := Chain{}
	if err := h.db.Get(&chain, "SELECT * FROM chains WHERE id=$1", project.Chain); err != nil {
		render.Respond(w, r, NewResponse(http.StatusNotFound, nil, err))
		return
	}

	project.ID = RandStringBytesRemainder(32)
	project.Secret = RandStringBytesRemainder(32)
	project.Status = "Active"
	// project.CreateTime = time.Now()
	if _, err := h.db.NamedExec(`INSERT INTO projects VALUES (:id,:name,:chain,:status,:secret)`, project); err != nil {
		render.Respond(w, r, NewResponse(http.StatusInternalServerError, nil, err))
	} else {
		render.Respond(w, r, NewResponse(http.StatusOK, project, nil))
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.db.Ping(); err != nil {
		render.Respond(w, r, NewResponse(http.StatusInternalServerError, nil, err))
	} else {
		render.Respond(w, r, NewResponse(http.StatusOK, nil, nil))
	}
}

func (h *Handler) Clear(w http.ResponseWriter, r *http.Request) {
	h.cache.Purge()
	render.Respond(w, r, NewResponse(http.StatusOK, nil, nil))
}

func (h *Handler) Route(w http.ResponseWriter, r *http.Request) {
	chainID, projectID := chi.URLParam(r, "chainID"), chi.URLParam(r, "projectID")
	if chainID == "" || projectID == "" {
		render.Respond(w, r, Route{})
		return
	}

	if val, ok := h.cache.Get(r.URL.Path); ok {
		route := val.(*Route)
		render.Respond(w, r, route)
		return
	}

	chain := Chain{}
	stmt := "SELECT chains.* FROM projects JOIN chains ON chains.id=projects.chain WHERE projects.id=$1 AND chains.id=$2"
	if err := h.db.Get(&chain, stmt, projectID, chainID); err != nil {
		render.Respond(w, r, Route{})
		return
	}

	route := Route{
		Route: true,
		Target: struct {
			RPC     string `json:"rpc" default:""`
			WS      string `json:"ws" default:""`
			GRPC    string `json:"grpc" default:""`
			REST    string `json:"rest" default:""`
			ETH_RPC string `json:"eth_rpc" default:""`
			ETH_WS  string `json:"eth_ws" default:""`
		}{
			RPC:     chain.RPC,
			WS:      chain.WS,
			GRPC:    chain.GRPC,
			REST:    chain.REST,
			ETH_RPC: chain.ETH_RPC,
			ETH_WS:  chain.ETH_WS,
		},
	}
	h.cache.Add(r.URL.Path, &route)
	render.Respond(w, r, route)
}

func NewRouter(db *sqlx.DB) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	h := NewHandler(db)
	r.Get("/health", h.Health)
	r.Get("/clear", h.Clear)
	r.Get("/route/{chainID}/{projectID}", h.Route)
	r.Route("/chains", func(r chi.Router) {
		r.Get("/", h.ListChains)
		r.Post("/", h.CreateChain)
		r.Get("/{chainID}", h.GetChain)
	})
	r.Route("/projects", func(r chi.Router) {
		r.Get("/", h.ListProjects)
		r.Post("/", h.CreateProject)
		r.Get("/{projectID}", h.GetProject)
	})
	return r
}
