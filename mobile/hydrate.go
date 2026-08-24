package mobile

import "gtd/internal/domain"

type hydratedTask struct {
	domain.Task
	ProjectTitle string `json:"projectTitle"`
	AreaName     string `json:"areaName"`
	Belongs      string `json:"belongs"`
}

func (g *Gtd) hydrateIDs(ids []string) ([]hydratedTask, error) {
	out := []hydratedTask{}
	if g == nil || g.ctx == nil {
		return out, nil
	}
	cat, err := g.ctx.EntityCatalog()
	if err != nil {
		return nil, err
	}
	projects := map[string]string{}
	areas := map[string]string{}
	if cat != nil {
		for _, p := range cat.Projects {
			projects[p.ID] = p.Title
		}
		for _, a := range cat.Areas {
			areas[a.ID] = a.Name
		}
	}
	for _, id := range ids {
		t, err := g.ctx.GetTask(id)
		if err != nil || t == nil {
			continue
		}
		out = append(out, hydrateTask(t, projects, areas))
	}
	return out, nil
}

func hydrateTask(t *domain.Task, projects, areas map[string]string) hydratedTask {
	h := hydratedTask{Task: *t}
	if t.ProjectID != nil {
		h.ProjectTitle = projects[*t.ProjectID]
	}
	if t.AreaID != nil {
		h.AreaName = areas[*t.AreaID]
	}
	switch {
	case t.ProjectID != nil && h.ProjectTitle != "":
		h.Belongs = "project:" + h.ProjectTitle
	case t.AreaID != nil && h.AreaName != "":
		h.Belongs = "area:" + h.AreaName
	case t.Status == domain.TaskStatusNext:
		h.Belongs = "next"
	default:
		h.Belongs = "inbox"
	}
	return h
}
