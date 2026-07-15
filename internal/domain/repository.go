package domain

type TaskRepository interface {
	Save(task *Task) error
	Get(id string) (*Task, error)
	Delete(id string) error
	List() ([]*Task, error)
}

type ProjectRepository interface {
	Save(project *Project) error
	Get(id string) (*Project, error)
	Delete(id string) error
	List() ([]*Project, error)
}

type AreaRepository interface {
	Save(area *Area) error
	Get(id string) (*Area, error)
	Delete(id string) error
	List() ([]*Area, error)
}

type PersonRepository interface {
	Save(person *Person) error
	Get(id string) (*Person, error)
	Delete(id string) error
	List() ([]*Person, error)
}

type SectionRepository interface {
	Save(section *Section) error
	Get(id string) (*Section, error)
	Delete(id string) error
	List() ([]*Section, error)
}

type SavedFilterRepository interface {
	Save(filter *SavedFilter) error
	Get(id string) (*SavedFilter, error)
	Delete(id string) error
	List() ([]*SavedFilter, error)
}
