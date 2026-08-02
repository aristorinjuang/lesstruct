package handlers

import (
	"context"
	"net/http"
	"strings"

	aliasdomain "github.com/aristorinjuang/lesstruct/internal/domain/alias"
	contentdomain "github.com/aristorinjuang/lesstruct/internal/domain/content"
)

type contentGetter interface {
	GetByID(ctx context.Context, id int) (*contentdomain.Content, error)
}

type AliasRedirectHandler struct {
	aliasService *aliasdomain.Service
	contentSvc   contentGetter
	next         http.Handler
}

func (h *AliasRedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		h.next.ServeHTTP(w, r)
		return
	}

	a, err := h.aliasService.FindByAlias(r.Context(), path)
	if err != nil {
		h.next.ServeHTTP(w, r)
		return
	}

	content, err := h.contentSvc.GetByID(r.Context(), a.ContentID)
	if err != nil || content.Slug == "" {
		h.next.ServeHTTP(w, r)
		return
	}

	target := "/" + content.Slug
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, target, http.StatusMovedPermanently)
}

func NewAliasRedirectHandler(
	aliasService *aliasdomain.Service,
	contentSvc contentGetter,
	next http.Handler,
) *AliasRedirectHandler {
	return &AliasRedirectHandler{
		aliasService: aliasService,
		contentSvc:   contentSvc,
		next:         next,
	}
}
