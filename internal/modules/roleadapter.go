package modules

import (
	roledomain "github.com/aristorinjuang/lesstruct/internal/domain/role"
	mediaapi "github.com/aristorinjuang/lesstruct/modules/media/api"
	mediaagent "github.com/aristorinjuang/lesstruct/modules/media/api/agent"
	"github.com/aristorinjuang/lesstruct/modules/posttypes-basic/api"
)

type roleCapabilities struct {
	service *roledomain.Service
}

func (a *roleCapabilities) IsAdmin(name string) bool {
	return a.service.IsAdmin(name)
}

func (a *roleCapabilities) ManageablePostTypes(name string, allSlugs []string) []string {
	return a.service.ManageablePostTypes(name, allSlugs)
}

func (a *roleCapabilities) CanPublish(name string) bool {
	return a.service.CanPublish(name)
}

func (a *roleCapabilities) CanMedia(name string) bool {
	return a.service.CanMedia(name)
}

func (a *roleCapabilities) CanComment(name string) bool {
	return a.service.CanComment(name)
}

func (a *roleCapabilities) GetAll() []api.Role {
	roles := a.service.GetAll()
	result := make([]api.Role, 0, len(roles))
	for _, role := range roles {
		result = append(result, api.Role{
			Name:      role.Name,
			PostTypes: role.PostTypes,
			AllTypes:  role.AllTypes,
			Publish:   role.Publish,
			Media:     role.Media,
			Comments:  role.Comments,
			IsAdmin:   role.IsAdmin,
		})
	}
	return result
}

// NewRoleCapabilities adapts the config-driven role registry to the
// posttypes-basic module's narrow seam. It lives in core — the only layer
// that may import both sides — until the users module implements the seam
// natively, at which point this adapter is deleted. A nil service yields
// nil, and the module fails its roles endpoint closed.
func NewRoleCapabilities(service *roledomain.Service) api.RoleCapabilities {
	if service == nil {
		return nil
	}
	return &roleCapabilities{
		service: service,
	}
}

// NewMediaRoleCapabilities adapts the config-driven role registry to the
// media module's browser seam (upload/generate/delete gates). A nil service
// yields nil, and the module preserves the legacy allow-all behavior.
func NewMediaRoleCapabilities(service *roledomain.Service) mediaapi.RoleCapabilities {
	if service == nil {
		return nil
	}
	return &roleCapabilities{
		service: service,
	}
}

// NewAgentMediaRoleCapabilities adapts the config-driven role registry to the
// media module's agent seam (upload gate). A nil service yields nil, and the
// module preserves the legacy allow-all behavior.
func NewAgentMediaRoleCapabilities(service *roledomain.Service) mediaagent.RoleCapabilities {
	if service == nil {
		return nil
	}
	return &roleCapabilities{
		service: service,
	}
}
