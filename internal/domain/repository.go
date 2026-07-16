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


