package mobile

import "gtd/internal/domain"

type hydratedTask struct {
	domain.Task
	ProjectTitle string `json:"projectTitle"`
	AreaName     string `json:"areaName"`
	Belongs      string `json:"belongs"`
}

func (g *Gtd) catalogTitles() (projects, areas map[string]string, err error) {
	projects = map[string]string{}
	areas = map[string]string{}
	if g == nil || g.ctx == nil {
		return projects, areas, nil
	}
	cat, err := g.ctx.EntityCatalog()
	if err != nil {
		return nil, nil, err
	}
	if cat != nil {
		for _, p := range cat.Projects {
			projects[p.ID] = p.Title
		}
		for _, a := range cat.Areas {
			areas[a.ID] = a.Name
		}
	}
	return projects, areas, nil
}

func (g *Gtd) hydrateIDs(ids []string) ([]hydratedTask, error) {
	out := []hydratedTask{}
	if g == nil || g.ctx == nil {
		return out, nil
	}
	projects, areas, err := g.catalogTitles()
	if err != nil {
		return nil, err
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

func (g *Gtd) hydratePersisted(t *domain.Task) (hydratedTask, error) {
	projects, areas, err := g.catalogTitles()
	if err != nil {
		return hydratedTask{}, err
	}
	return hydrateTask(t, projects, areas), nil
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
