package mobile

type catalogData struct {
	Tags     []string         `json:"tags"`
	Contexts []string         `json:"contexts"`
	Projects []catalogProject `json:"projects"`
	Areas    []catalogArea    `json:"areas"`
}

type catalogProject struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type catalogArea struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (g *Gtd) catalog() string {
	if g == nil || g.ctx == nil {
		return encodeFail("not_open", "workspace is not open")
	}
	tags, err := g.ctx.ListTags()
	if err != nil {
		return encodeFail("internal", err.Error())
	}
	if tags == nil {
		tags = []string{}
	}
	contexts, err := g.ctx.ListContexts()
	if err != nil {
		return encodeFail("internal", err.Error())
	}
	if contexts == nil {
		contexts = []string{}
	}
	cat, err := g.ctx.EntityCatalog()
	if err != nil {
		return encodeFail("internal", err.Error())
	}
	projects := []catalogProject{}
	areas := []catalogArea{}
	if cat != nil {
		for _, p := range cat.Projects {
			projects = append(projects, catalogProject{ID: p.ID, Title: p.Title})
		}
		for _, a := range cat.Areas {
			areas = append(areas, catalogArea{ID: a.ID, Name: a.Name})
		}
	}
	return encodeOK(catalogData{
		Tags:     tags,
		Contexts: contexts,
		Projects: projects,
		Areas:    areas,
	})
}
